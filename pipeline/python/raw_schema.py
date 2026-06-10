"""
Shared definition of the raw ClickHouse landing tables.

Both the Python (Beginner) and the Go (Intermediate) loaders land data into
these tables. Tables use ReplacingMergeTree(updated_at) keyed by
(tenant_id, <pk>) so that:
  * full-load (Beginner) simply truncates + inserts
  * incremental-load (Intermediate) re-inserts changed rows and the engine
    keeps the latest version per key (read with FINAL downstream).
"""

# table -> (primary key column, [ (col_name, clickhouse_type), ... ])
# Every table additionally gets: tenant_id String, _extracted_at DateTime
RAW_TABLES = {
    "customers": ("customer_id", [
        ("customer_id", "Int64"),
        ("name", "String"),
        ("phone", "Nullable(String)"),
        ("email", "Nullable(String)"),
        ("gender", "Nullable(String)"),
        ("city", "Nullable(String)"),
        ("created_at", "DateTime"),
        ("updated_at", "DateTime"),
    ]),
    "products": ("product_id", [
        ("product_id", "Int64"),
        ("product_name", "String"),
        ("category", "Nullable(String)"),
        ("brand", "Nullable(String)"),
        ("unit_price", "Float64"),
        ("is_active", "Bool"),
        ("created_at", "DateTime"),
        ("updated_at", "DateTime"),
    ]),
    "stores": ("store_id", [
        ("store_id", "Int64"),
        ("store_name", "String"),
        ("city", "Nullable(String)"),
        ("province", "Nullable(String)"),
        ("store_type", "Nullable(String)"),
        ("opened_at", "Nullable(Date)"),
        ("is_active", "Bool"),
        ("created_at", "DateTime"),
        ("updated_at", "DateTime"),
    ]),
    "suppliers": ("supplier_id", [
        ("supplier_id", "Int64"),
        ("supplier_name", "Nullable(String)"),
        ("contact_name", "Nullable(String)"),
        ("city", "Nullable(String)"),
        ("country", "Nullable(String)"),
        ("created_at", "DateTime"),
        ("updated_at", "DateTime"),
    ]),
    "promotions": ("promo_id", [
        ("promo_id", "Int64"),
        ("promo_name", "Nullable(String)"),
        ("promo_type", "Nullable(String)"),
        ("discount_pct", "Nullable(Float64)"),
        ("start_date", "Nullable(Date)"),
        ("end_date", "Nullable(Date)"),
        ("min_purchase", "Nullable(Float64)"),
        ("created_at", "DateTime"),
        ("updated_at", "DateTime"),
    ]),
    "transactions": ("transaction_id", [
        ("transaction_id", "Int64"),
        ("customer_id", "Nullable(Int64)"),
        ("store_id", "Nullable(Int64)"),
        ("employee_id", "Nullable(Int64)"),
        ("transaction_date", "DateTime"),
        ("total_amount", "Float64"),
        ("payment_method", "Nullable(String)"),
        ("status", "Nullable(String)"),
        ("created_at", "DateTime"),
        ("updated_at", "DateTime"),
    ]),
    "transaction_items": ("item_id", [
        ("item_id", "Int64"),
        ("transaction_id", "Int64"),
        ("product_id", "Nullable(Int64)"),
        ("quantity", "Int64"),
        ("unit_price", "Float64"),
        ("discount", "Float64"),
        ("subtotal", "Float64"),
        ("created_at", "DateTime"),
        ("updated_at", "DateTime"),
    ]),
    "transaction_promotions": ("id", [
        ("id", "Int64"),
        ("transaction_id", "Int64"),
        ("promo_id", "Nullable(Int64)"),
        ("discount_applied", "Nullable(Float64)"),
        ("created_at", "DateTime"),
        ("updated_at", "DateTime"),
    ]),
    # ---------------- Advanced tables ----------------
    "employees": ("employee_id", [
        ("employee_id", "Int64"),
        ("store_id", "Nullable(Int64)"),
        ("name", "String"),
        ("role", "Nullable(String)"),
        ("hire_date", "Nullable(Date)"),
        ("is_active", "Bool"),
        ("created_at", "DateTime"),
        ("updated_at", "DateTime"),
    ]),
    "inventory": ("inventory_id", [
        ("inventory_id", "Int64"),
        ("product_id", "Nullable(Int64)"),
        ("store_id", "Nullable(Int64)"),
        ("supplier_id", "Nullable(Int64)"),
        ("stock_qty", "Int64"),
        ("reorder_level", "Int64"),
        ("last_restocked", "Nullable(DateTime)"),
        ("created_at", "DateTime"),
        ("updated_at", "DateTime"),
    ]),
    "product_supplier": ("id", [
        ("id", "Int64"),
        ("product_id", "Nullable(Int64)"),
        ("supplier_id", "Nullable(Int64)"),
        ("cost_price", "Nullable(Float64)"),
        ("lead_time_days", "Nullable(Int64)"),
        ("is_primary", "Bool"),
        ("created_at", "DateTime"),
        ("updated_at", "DateTime"),
    ]),
    "customer_loyalty": ("loyalty_id", [
        ("loyalty_id", "Int64"),
        ("customer_id", "Nullable(Int64)"),
        ("tier", "Nullable(String)"),
        ("points", "Int64"),
        ("total_spend", "Float64"),
        ("member_since", "Nullable(Date)"),
        ("created_at", "DateTime"),
        ("updated_at", "DateTime"),
    ]),
}


def create_table_ddl(db: str, table: str) -> str:
    pk, cols = RAW_TABLES[table]
    col_defs = [f"    `tenant_id` String"]
    col_defs += [f"    `{name}` {ctype}" for name, ctype in cols]
    col_defs.append("    `_extracted_at` DateTime DEFAULT now()")
    cols_sql = ",\n".join(col_defs)
    return (
        f"CREATE TABLE IF NOT EXISTS `{db}`.`{table}` (\n{cols_sql}\n)\n"
        f"ENGINE = ReplacingMergeTree(updated_at)\n"
        f"ORDER BY (tenant_id, {pk})"
    )


def source_columns(table: str):
    """Column names as they appear in the postgres source (no tenant_id)."""
    return [name for name, _ in RAW_TABLES[table][1]]
