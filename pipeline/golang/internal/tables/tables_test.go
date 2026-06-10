package tables

import (
	"strings"
	"testing"
)

func TestEveryTableHasWatermarkColumn(t *testing.T) {
	for _, tbl := range All {
		found := false
		for _, c := range tbl.Columns {
			if c.Name == tbl.WatermarkCol {
				found = true
			}
		}
		if !found {
			t.Errorf("%s missing watermark column %q", tbl.Name, tbl.WatermarkCol)
		}
	}
}

func TestCHColumnsPrependTenant(t *testing.T) {
	cols := All[0].CHColumns()
	if cols[0] != "tenant_id" {
		t.Fatalf("expected tenant_id first, got %q", cols[0])
	}
}

func TestCreateDDLShape(t *testing.T) {
	ddl := All[0].CreateDDL("raw")
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS `raw`.`customers`",
		"`tenant_id` String",
		"ReplacingMergeTree(updated_at)",
		"ORDER BY (tenant_id, customer_id)",
	} {
		if !strings.Contains(ddl, want) {
			t.Errorf("ddl missing %q\n%s", want, ddl)
		}
	}
}

func TestSelectListCastsNumerics(t *testing.T) {
	for _, tbl := range All {
		if tbl.Name == "products" {
			if !strings.Contains(tbl.SelectList(), "unit_price::double precision") {
				t.Errorf("products select should cast unit_price")
			}
		}
	}
}
