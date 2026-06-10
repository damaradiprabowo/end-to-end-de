"""
Beginner ELT — Extract & Load
==============================
Extracts the core POS tables from a single PostgreSQL tenant schema and loads
them into the ClickHouse `raw` database using a simple FULL-LOAD strategy
(truncate + insert). Transformation is handled downstream by dbt.

Usage:
    python extract_load.py                 # loads default tenant (tenant_01)
    python extract_load.py --tenant tenant_02
    python extract_load.py --tables customers,products

Design notes:
  * Beginner scope = single tenant, full load, no incremental watermark.
  * Data lands in the same `raw.*` tables the Go loader uses, so dbt models
    work identically regardless of which loader produced the data.
"""
import os
import sys
import time
import argparse
import logging
from datetime import datetime

from decimal import Decimal

import psycopg2
import clickhouse_connect

from raw_schema import RAW_TABLES, create_table_ddl, source_columns

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s | %(levelname)-7s | el-python | %(message)s",
)
log = logging.getLogger("el")

BATCH_SIZE = 5000


def pg_conn():
    return psycopg2.connect(
        host=os.getenv("POSTGRES_HOST", "localhost"),
        port=int(os.getenv("POSTGRES_PORT", "5432")),
        dbname=os.getenv("POSTGRES_DB", "pos"),
        user=os.getenv("POSTGRES_USER", "pos_user"),
        password=os.getenv("POSTGRES_PASSWORD", "pos_password"),
    )


def ch_client():
    return clickhouse_connect.get_client(
        host=os.getenv("CLICKHOUSE_HOST", "localhost"),
        port=int(os.getenv("CLICKHOUSE_HTTP_PORT", "8123")),
        username=os.getenv("CLICKHOUSE_USER", "default"),
        password=os.getenv("CLICKHOUSE_PASSWORD", "clickhouse"),
    )


def ensure_raw_db(ch, db):
    ch.command(f"CREATE DATABASE IF NOT EXISTS `{db}`")
    for table in RAW_TABLES:
        ch.command(create_table_ddl(db, table))
    log.info("ensured raw database `%s` and %d tables", db, len(RAW_TABLES))


def _clean(value):
    """Make psycopg2 values compatible with the ClickHouse Float64/Bool columns."""
    if isinstance(value, Decimal):
        return float(value)
    return value


def extract_load_table(pg, ch, db, schema, tenant_id, table):
    cols = source_columns(table)
    col_list = ", ".join(cols)
    t0 = time.time()

    # ---- full load: clear existing rows for this tenant ----
    ch.command(f"ALTER TABLE `{db}`.`{table}` DELETE WHERE tenant_id = %(t)s",
               parameters={"t": tenant_id})

    with pg.cursor(name=f"cur_{table}") as cur:   # server-side cursor
        cur.itersize = BATCH_SIZE
        cur.execute(f'SELECT {col_list} FROM "{schema}".{table} ORDER BY 1')
        ch_cols = ["tenant_id"] + cols
        total = 0
        while True:
            rows = cur.fetchmany(BATCH_SIZE)
            if not rows:
                break
            payload = [[tenant_id, *(_clean(v) for v in row)] for row in rows]
            ch.insert(table, payload, column_names=ch_cols, database=db)
            total += len(rows)
            log.info("  [%s.%s] inserted batch, running total=%d",
                     tenant_id, table, total)

    log.info("[%s.%s] full-load complete: %d rows in %.2fs",
             tenant_id, table, total, time.time() - t0)
    return total


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--tenant", default=os.getenv("BEGINNER_TENANT", "tenant_01"))
    ap.add_argument("--tables", default=",".join(RAW_TABLES.keys()),
                    help="comma separated subset of tables to load")
    args = ap.parse_args()

    tenant = args.tenant.strip()
    tables = [t.strip() for t in args.tables.split(",") if t.strip()]
    db = os.getenv("CLICKHOUSE_RAW_DB", "raw")

    log.info("=== Beginner EL start | tenant=%s | tables=%s ===", tenant, tables)
    started = datetime.now()
    pg, ch = pg_conn(), ch_client()
    try:
        ensure_raw_db(ch, db)
        grand_total = 0
        for table in tables:
            if table not in RAW_TABLES:
                log.warning("skipping unknown table: %s", table)
                continue
            grand_total += extract_load_table(pg, ch, db, tenant, tenant, table)
        log.info("=== EL done | %d rows total | elapsed=%s ===",
                 grand_total, datetime.now() - started)
    except Exception:
        log.exception("EL failed")
        sys.exit(1)
    finally:
        pg.close()


if __name__ == "__main__":
    main()
