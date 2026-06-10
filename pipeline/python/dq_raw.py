"""
Raw-layer Data Quality — row-count reconciliation
=================================================
Fail-fast reconciliation of the ClickHouse `raw.*` landing tables against their
PostgreSQL source of truth. This is the one DQ check dbt cannot perform, because
dbt only sees the warehouse — it cannot reach back to the OLTP source. It is run
as the first gate of the Advanced DAG, immediately after extract-load and before
any dbt transformation, so a broken/partial load is caught before it propagates
downstream.

For every (tenant, table):
  * source count  = rows in PostgreSQL "<schema>".<table>
  * landed count  = rows in ClickHouse raw.<table> FINAL WHERE tenant_id = ...
                    (FINAL collapses ReplacingMergeTree versions so re-runs of an
                     incremental load do not inflate the count)

A non-zero exit code is returned if any table is missing rows (landed < source).
Landed > source is reported as a warning, not a failure: the warehouse may retain
history for rows that were later hard-deleted from the source.

Usage:
    python dq_raw.py                                  # all tenants from TENANT_SCHEMAS
    python dq_raw.py --tenants tenant_01,tenant_02
    python dq_raw.py --tables customers,transactions  # subset of tables
    python dq_raw.py --tolerance 0                    # exact-match (default)
"""
import os
import sys
import argparse
import logging

# Reuse the connection helpers + table catalog from the loader so the DQ check
# can never drift from what was actually loaded.
from extract_load import pg_conn, ch_client
from raw_schema import RAW_TABLES

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s | %(levelname)-7s | dq-raw | %(message)s",
)
log = logging.getLogger("dq-raw")


def pg_count(pg, schema: str, table: str) -> int:
    with pg.cursor() as cur:
        cur.execute(f'SELECT count(*) FROM "{schema}".{table}')
        return int(cur.fetchone()[0])


def ch_count(ch, db: str, table: str, tenant_id: str) -> int:
    # FINAL dedupes the ReplacingMergeTree so incremental re-loads are not counted
    # twice; scoped to this tenant only.
    result = ch.query(
        f"SELECT count(*) FROM `{db}`.`{table}` FINAL WHERE tenant_id = %(t)s",
        parameters={"t": tenant_id},
    )
    return int(result.result_rows[0][0])


def reconcile(pg, ch, db, tenant, schema, tables, tolerance):
    """Return a list of failure descriptions for one tenant (empty == all good)."""
    failures = []
    for table in tables:
        src = pg_count(pg, schema, table)
        dst = ch_count(ch, db, table, tenant)
        diff = dst - src
        if diff < -tolerance:
            log.error("[%s.%s] MISSING rows: source=%d landed=%d (diff=%d)",
                      tenant, table, src, dst, diff)
            failures.append(f"{tenant}.{table}: source={src} landed={dst}")
        elif diff > tolerance:
            log.warning("[%s.%s] landed > source: source=%d landed=%d (diff=+%d) "
                        "(tolerated — likely source hard-deletes)",
                        tenant, table, src, dst, diff)
        else:
            log.info("[%s.%s] OK: %d rows", tenant, table, src)
    return failures


def main():
    ap = argparse.ArgumentParser(description="Raw-layer row-count reconciliation")
    ap.add_argument(
        "--tenants",
        default=os.getenv("TENANT_SCHEMAS", "tenant_01,tenant_02,tenant_03"),
        help="comma separated tenant schemas to check",
    )
    ap.add_argument("--tables", default=",".join(RAW_TABLES.keys()),
                    help="comma separated subset of tables to reconcile")
    ap.add_argument("--tolerance", type=int, default=0,
                    help="allowed absolute row-count drift before failing")
    args = ap.parse_args()

    tenants = [t.strip() for t in args.tenants.split(",") if t.strip()]
    tables = [t.strip() for t in args.tables.split(",") if t.strip()]
    unknown = [t for t in tables if t not in RAW_TABLES]
    if unknown:
        log.error("unknown table(s): %s", ", ".join(unknown))
        sys.exit(2)

    db = os.getenv("CLICKHOUSE_RAW_DB", "raw")
    log.info("=== Raw DQ start | tenants=%s | tables=%d | tolerance=%d ===",
             tenants, len(tables), args.tolerance)

    pg, ch = pg_conn(), ch_client()
    all_failures = []
    try:
        for tenant in tenants:
            # tenant_id == schema name in this project's seed convention
            all_failures += reconcile(pg, ch, db, tenant, tenant, tables, args.tolerance)
    finally:
        pg.close()

    if all_failures:
        log.error("=== Raw DQ FAILED | %d table(s) under source count ===",
                  len(all_failures))
        for f in all_failures:
            log.error("  - %s", f)
        sys.exit(1)

    log.info("=== Raw DQ PASSED | all %d table(s) reconciled across %d tenant(s) ===",
             len(tables), len(tenants))


if __name__ == "__main__":
    main()
