package dbmonitor

import (
	"reflect"
	"testing"
)

func TestValidTableName(t *testing.T) {
	cases := map[string]bool{
		"users":          true,
		"project_groups": true,
		"_internal":      true,
		"a1":             true,
		"1table":         false,
		"":               false,
		"a-b":            false,
		"drop table":     false,
		"tbl;DROP":       false,
	}
	for name, want := range cases {
		if got := validTableName(name); got != want {
			t.Errorf("validTableName(%q)=%v want %v", name, got, want)
		}
	}
}

func TestSortTables(t *testing.T) {
	tables := []TableStat{
		{Name: "b", Rows: 10, TotalBytes: 100},
		{Name: "a", Rows: 30, TotalBytes: 50},
		{Name: "c", Rows: 20, TotalBytes: 200},
	}
	sortTables(tables, "", "")
	if tables[0].Name != "a" || tables[2].Name != "c" {
		t.Fatalf("name asc failed: %v", tables)
	}
	sortTables(tables, "totalBytes", "desc")
	if !reflect.DeepEqual([]string{tables[0].Name, tables[1].Name, tables[2].Name}, []string{"c", "b", "a"}) {
		t.Fatalf("totalBytes desc failed: %v", tables)
	}
	sortTables(tables, "rows", "asc")
	if tables[0].Rows != 10 || tables[2].Rows != 30 {
		t.Fatalf("rows asc failed: %v", tables)
	}
}
