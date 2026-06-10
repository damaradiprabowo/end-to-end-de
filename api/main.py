"""
Intermediate Data API (FastAPI)
================================
Serves the 5 analytic questions from the ClickHouse `analytics` star schema.
The companion dashboard (../dashboard) consumes these endpoints.
"""
import os
import logging

import clickhouse_connect
from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware

import queries

logging.basicConfig(level=logging.INFO,
                    format="%(asctime)s | %(levelname)s | api | %(message)s")
log = logging.getLogger("api")

app = FastAPI(title="Minimarket Analytics API", version="1.0.0")
app.add_middleware(
    CORSMiddleware, allow_origins=["*"], allow_methods=["*"], allow_headers=["*"],
)


def get_client():
    return clickhouse_connect.get_client(
        host=os.getenv("CLICKHOUSE_HOST", "localhost"),
        port=int(os.getenv("CLICKHOUSE_HTTP_PORT", "8123")),
        username=os.getenv("CLICKHOUSE_USER", "default"),
        password=os.getenv("CLICKHOUSE_PASSWORD", "clickhouse"),
    )


def run_query(sql: str):
    """Execute SQL and return a list of dicts."""
    try:
        client = get_client()
        result = client.query(sql)
        cols = result.column_names
        return [dict(zip(cols, row)) for row in result.result_rows]
    except Exception as exc:  # noqa: BLE001
        log.exception("query failed")
        raise HTTPException(status_code=500, detail=str(exc))


@app.get("/health")
def health():
    try:
        get_client().query("SELECT 1")
        return {"status": "ok"}
    except Exception as exc:  # noqa: BLE001
        raise HTTPException(status_code=503, detail=str(exc))


@app.get("/api/revenue-by-store")
def revenue_by_store():
    """Q1 — revenue per store per month (last 6 months)."""
    return run_query(queries.REVENUE_BY_STORE)


@app.get("/api/promotion-effectiveness")
def promotion_effectiveness():
    """Q2 — discount totals per promo + avg txn value with/without promo."""
    return {
        "promotions": run_query(queries.PROMOTION_EFFECTIVENESS),
        "avg_transaction": run_query(queries.PROMO_AVG_COMPARISON),
    }


@app.get("/api/top-products-by-city")
def top_products_by_city():
    """Q3 — top 3 products by revenue in each city."""
    return run_query(queries.TOP_PRODUCTS_BY_CITY)


@app.get("/api/customer-segments")
def customer_segments():
    """Q4 — customer High/Medium/Low segmentation per city."""
    return run_query(queries.CUSTOMER_SEGMENTS)


@app.get("/api/transactions-by-day")
def transactions_by_day():
    """Q5 — transactions and revenue by day of week."""
    return run_query(queries.TRANSACTIONS_BY_DAY)
