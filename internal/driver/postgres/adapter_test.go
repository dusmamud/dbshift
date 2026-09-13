package postgres

import (
	"strings"
	"testing"

	"github.com/dusmamud/dbshift/internal/core"
)

func TestQuoteIdentifier(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"users", `"users"`},
		{"order_items", `"order_items"`},
		{"table\"name", `"table""name"`},
	}

	for _, c := range cases {
		got := quoteIdentifier(c.input)
		if got != c.expected {
			t.Errorf("quoteIdentifier(%q) = %q; want %q", c.input, got, c.expected)
		}
	}
}

func TestMapPgType(t *testing.T) {
	if mapPgType("character varying") != "TEXT" {
		t.Errorf("expected TEXT, got %s", mapPgType("character varying"))
	}
	if mapPgType("integer") != "INT" {
		t.Errorf("expected INT, got %s", mapPgType("integer"))
	}
	if mapPgType("json") != "JSONB" {
		t.Errorf("expected JSONB, got %s", mapPgType("json"))
	}
}

func TestGenerateCreateTableDDL(t *testing.T) {
	d := NewDriver()
	meta := core.CollectionMeta{
		Name: "users",
		Columns: []core.ColumnMeta{
			{Name: "id", DataType: "uuid", IsPrimaryKey: true, IsNullable: false},
			{Name: "email", DataType: "text", IsNullable: false},
			{Name: "age", DataType: "integer", IsNullable: true},
		},
	}

	ddl := d.generateCreateTableDDL(meta)
	if !strings.Contains(ddl, `CREATE TABLE IF NOT EXISTS "users"`) {
		t.Errorf("expected CREATE TABLE statement, got: %s", ddl)
	}
	if !strings.Contains(ddl, `PRIMARY KEY ("id")`) {
		t.Errorf("expected PRIMARY KEY, got: %s", ddl)
	}
}

func TestSeparateForeignKeys(t *testing.T) {
	rawDDL := `CREATE TABLE music_chart_cache (
    id UUID NOT NULL,
    user_id UUID NOT NULL,
    CONSTRAINT "primary" PRIMARY KEY (id ASC),
    CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES music_users(id) ON DELETE CASCADE
);`

	cleanDDL, alterStmts := separateForeignKeys("music_chart_cache", rawDDL)
	if strings.Contains(cleanDDL, "FOREIGN KEY") {
		t.Errorf("expected FOREIGN KEY to be removed from cleanDDL, got: %s", cleanDDL)
	}
	if len(alterStmts) != 1 {
		t.Fatalf("expected 1 alter statement, got %d", len(alterStmts))
	}
	if !strings.Contains(alterStmts[0], "ALTER TABLE \"music_chart_cache\" ADD CONSTRAINT fk_user FOREIGN KEY") {
		t.Errorf("unexpected alter statement: %s", alterStmts[0])
	}
}
