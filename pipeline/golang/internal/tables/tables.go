// Package tables defines the metadata for every source/raw table.
// It is the Go counterpart of pipeline/python/raw_schema.py and MUST stay in
// sync with it: both loaders create and write to identical ClickHouse tables.
package tables

import "fmt"

// Column describes one column of a raw table.
type Column struct {
	Name string // logical name (same in postgres + clickhouse)
	// SelectExpr is the postgres SELECT expression. Numerics are cast to
	// double precision so lib/pq returns float64 (matches CH Float64).
	SelectExpr string
	CHType     string // ClickHouse column type
}

// Table describes one raw table and how to extract it incrementally.
type Table struct {
	Name         string
	PK           string
	WatermarkCol string // column used for incremental high-watermark
	Columns      []Column
}

func col(name, chType string) Column { return Column{name, name, chType} }
func numCol(name string) Column {
	return Column{name, name + "::double precision AS " + name, "Float64"}
}
func numColN(name string) Column {
	return Column{name, name + "::double precision AS " + name, "Nullable(Float64)"}
}

// All returns every raw table in dependency-friendly order.
var All = []Table{
	{Name: "customers", PK: "customer_id", WatermarkCol: "updated_at", Columns: []Column{
		col("customer_id", "Int64"),
		col("name", "String"),
		col("phone", "Nullable(String)"),
		col("email", "Nullable(String)"),
		col("gender", "Nullable(String)"),
		col("city", "Nullable(String)"),
		col("created_at", "DateTime"),
		col("updated_at", "DateTime"),
	}},
	{Name: "products", PK: "product_id", WatermarkCol: "updated_at", Columns: []Column{
		col("product_id", "Int64"),
		col("product_name", "String"),
		col("category", "Nullable(String)"),
		col("brand", "Nullable(String)"),
		numCol("unit_price"),
		col("is_active", "Bool"),
		col("created_at", "DateTime"),
		col("updated_at", "DateTime"),
	}},
	{Name: "stores", PK: "store_id", WatermarkCol: "updated_at", Columns: []Column{
		col("store_id", "Int64"),
		col("store_name", "String"),
		col("city", "Nullable(String)"),
		col("province", "Nullable(String)"),
		col("store_type", "Nullable(String)"),
		col("opened_at", "Nullable(Date)"),
		col("is_active", "Bool"),
		col("created_at", "DateTime"),
		col("updated_at", "DateTime"),
	}},
	{Name: "suppliers", PK: "supplier_id", WatermarkCol: "updated_at", Columns: []Column{
		col("supplier_id", "Int64"),
		col("supplier_name", "Nullable(String)"),
		col("contact_name", "Nullable(String)"),
		col("city", "Nullable(String)"),
		col("country", "Nullable(String)"),
		col("created_at", "DateTime"),
		col("updated_at", "DateTime"),
	}},
	{Name: "promotions", PK: "promo_id", WatermarkCol: "updated_at", Columns: []Column{
		col("promo_id", "Int64"),
		col("promo_name", "Nullable(String)"),
		col("promo_type", "Nullable(String)"),
		numColN("discount_pct"),
		col("start_date", "Nullable(Date)"),
		col("end_date", "Nullable(Date)"),
		numColN("min_purchase"),
		col("created_at", "DateTime"),
		col("updated_at", "DateTime"),
	}},
	{Name: "transactions", PK: "transaction_id", WatermarkCol: "updated_at", Columns: []Column{
		col("transaction_id", "Int64"),
		col("customer_id", "Nullable(Int64)"),
		col("store_id", "Nullable(Int64)"),
		col("employee_id", "Nullable(Int64)"),
		col("transaction_date", "DateTime"),
		numCol("total_amount"),
		col("payment_method", "Nullable(String)"),
		col("status", "Nullable(String)"),
		col("created_at", "DateTime"),
		col("updated_at", "DateTime"),
	}},
	{Name: "transaction_items", PK: "item_id", WatermarkCol: "updated_at", Columns: []Column{
		col("item_id", "Int64"),
		col("transaction_id", "Int64"),
		col("product_id", "Nullable(Int64)"),
		col("quantity", "Int64"),
		numCol("unit_price"),
		numCol("discount"),
		numCol("subtotal"),
		col("created_at", "DateTime"),
		col("updated_at", "DateTime"),
	}},
	{Name: "transaction_promotions", PK: "id", WatermarkCol: "updated_at", Columns: []Column{
		col("id", "Int64"),
		col("transaction_id", "Int64"),
		col("promo_id", "Nullable(Int64)"),
		numColN("discount_applied"),
		col("created_at", "DateTime"),
		col("updated_at", "DateTime"),
	}},
	// ---------------- Advanced tables ----------------
	{Name: "employees", PK: "employee_id", WatermarkCol: "updated_at", Columns: []Column{
		col("employee_id", "Int64"),
		col("store_id", "Nullable(Int64)"),
		col("name", "String"),
		col("role", "Nullable(String)"),
		col("hire_date", "Nullable(Date)"),
		col("is_active", "Bool"),
		col("created_at", "DateTime"),
		col("updated_at", "DateTime"),
	}},
	{Name: "inventory", PK: "inventory_id", WatermarkCol: "updated_at", Columns: []Column{
		col("inventory_id", "Int64"),
		col("product_id", "Nullable(Int64)"),
		col("store_id", "Nullable(Int64)"),
		col("supplier_id", "Nullable(Int64)"),
		col("stock_qty", "Int64"),
		col("reorder_level", "Int64"),
		col("last_restocked", "Nullable(DateTime)"),
		col("created_at", "DateTime"),
		col("updated_at", "DateTime"),
	}},
	{Name: "product_supplier", PK: "id", WatermarkCol: "updated_at", Columns: []Column{
		col("id", "Int64"),
		col("product_id", "Nullable(Int64)"),
		col("supplier_id", "Nullable(Int64)"),
		numColN("cost_price"),
		col("lead_time_days", "Nullable(Int64)"),
		col("is_primary", "Bool"),
		col("created_at", "DateTime"),
		col("updated_at", "DateTime"),
	}},
	{Name: "customer_loyalty", PK: "loyalty_id", WatermarkCol: "updated_at", Columns: []Column{
		col("loyalty_id", "Int64"),
		col("customer_id", "Nullable(Int64)"),
		col("tier", "Nullable(String)"),
		col("points", "Int64"),
		numCol("total_spend"),
		col("member_since", "Nullable(Date)"),
		col("created_at", "DateTime"),
		col("updated_at", "DateTime"),
	}},
}

// SelectList returns the postgres SELECT column expressions.
func (t Table) SelectList() string {
	out := ""
	for i, c := range t.Columns {
		if i > 0 {
			out += ", "
		}
		out += c.SelectExpr
	}
	return out
}

// CHColumns returns clickhouse column names with tenant_id prepended.
func (t Table) CHColumns() []string {
	cols := []string{"tenant_id"}
	for _, c := range t.Columns {
		cols = append(cols, c.Name)
	}
	return cols
}

// CreateDDL builds the ClickHouse CREATE TABLE statement (matches python).
func (t Table) CreateDDL(db string) string {
	out := fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s`.`%s` (\n", db, t.Name)
	out += "    `tenant_id` String,\n"
	for _, c := range t.Columns {
		out += fmt.Sprintf("    `%s` %s,\n", c.Name, c.CHType)
	}
	out += "    `_extracted_at` DateTime DEFAULT now()\n"
	out += fmt.Sprintf(") ENGINE = ReplacingMergeTree(updated_at)\nORDER BY (tenant_id, %s)", t.PK)
	return out
}
