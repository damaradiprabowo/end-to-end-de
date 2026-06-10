"""Unit tests for the raw schema helpers (run with: pytest)."""
from raw_schema import RAW_TABLES, create_table_ddl, source_columns


def test_all_tables_have_pk_in_columns():
    for table, (pk, cols) in RAW_TABLES.items():
        col_names = [c for c, _ in cols]
        assert pk in col_names, f"{table}: pk {pk} missing from columns"


def test_create_ddl_contains_tenant_and_engine():
    ddl = create_table_ddl("raw", "customers")
    assert "`tenant_id` String" in ddl
    assert "ReplacingMergeTree(updated_at)" in ddl
    assert "ORDER BY (tenant_id, customer_id)" in ddl


def test_source_columns_excludes_tenant_id():
    cols = source_columns("transactions")
    assert "tenant_id" not in cols
    assert "transaction_id" in cols
    assert cols[0] == "transaction_id"


def test_every_table_has_updated_at():
    for table in RAW_TABLES:
        assert "updated_at" in source_columns(table), table
