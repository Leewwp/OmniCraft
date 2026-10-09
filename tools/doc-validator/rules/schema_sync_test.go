package rules

import (
	"strings"
	"testing"
)

func TestSchemaSyncPreservesCompositeUniqueConstraint(t *testing.T) {
	table := parseTableBody("content_series_items", `
id BIGSERIAL PRIMARY KEY,
series_id BIGINT NOT NULL,
content_item_id BIGINT NOT NULL,
UNIQUE (series_id, content_item_id)
`)
	if len(table.UniqueConstraints) != 1 {
		t.Fatalf("unique constraints = %#v, want one composite constraint", table.UniqueConstraints)
	}
	for _, column := range table.Columns {
		if (column.Name == "series_id" || column.Name == "content_item_id") && column.Unique {
			t.Fatalf("column %s incorrectly marked individually unique", column.Name)
		}
	}

	generated := generateSchemaTable([]TableDef{table})
	if strings.Contains(generated, "`series_id` | `BIGINT` | NOT NULL UNIQUE") ||
		strings.Contains(generated, "`content_item_id` | `BIGINT` | NOT NULL UNIQUE") {
		t.Fatalf("generated schema misrepresents composite uniqueness:\n%s", generated)
	}
	if !strings.Contains(generated, "UNIQUE (`series_id`, `content_item_id`)") {
		t.Fatalf("generated schema omits composite constraint:\n%s", generated)
	}
}

func TestSchemaSyncPreservesMultilineCompositeUniqueConstraint(t *testing.T) {
	table := parseTableBody("rag_chunks", `
id BIGSERIAL PRIMARY KEY,
content_id BIGINT NOT NULL,
content_version INT NOT NULL,
chunking_version INT NOT NULL,
index_version INT NOT NULL,
chunk_index INT NOT NULL,
CONSTRAINT uq_rag_chunks_generation_order UNIQUE (
    content_id, content_version, chunking_version, index_version, chunk_index
)
`)
	want := []string{"content_id", "content_version", "chunking_version", "index_version", "chunk_index"}
	if len(table.UniqueConstraints) != 1 {
		t.Fatalf("unique constraints = %#v, want one multiline composite constraint", table.UniqueConstraints)
	}
	if strings.Join(table.UniqueConstraints[0], ",") != strings.Join(want, ",") {
		t.Fatalf("unique constraint = %#v, want %#v", table.UniqueConstraints[0], want)
	}
	generated := generateSchemaTable([]TableDef{table})
	if !strings.Contains(generated, "UNIQUE (`content_id`, `content_version`, `chunking_version`, `index_version`, `chunk_index`)") {
		t.Fatalf("generated schema omits multiline composite constraint:\n%s", generated)
	}
}

func TestSchemaSyncDoesNotTreatIndexPrefixedColumnAsConstraint(t *testing.T) {
	table := parseTableBody("rag_chunks", `
id BIGSERIAL PRIMARY KEY,
index_version INT NOT NULL CHECK (index_version > 0),
INDEX idx_rag_chunks_version (index_version)
`)
	if !table.hasColumn("index_version") {
		t.Fatalf("index_version column was mistaken for an INDEX table constraint: %#v", table.Columns)
	}
	if table.hasColumn("INDEX") {
		t.Fatalf("INDEX table constraint was mistaken for a column: %#v", table.Columns)
	}
}

func TestParseAlterTableColumnsMergesAddedColumns(t *testing.T) {
	content := `
CREATE TABLE content_items (
    id BIGSERIAL PRIMARY KEY,
    title VARCHAR(500) NOT NULL
);

ALTER TABLE content_items
    ADD COLUMN IF NOT EXISTS cover_width INT,
    ADD COLUMN IF NOT EXISTS cover_height INT;

ALTER TABLE content_attachments
    ADD COLUMN IF NOT EXISTS sort_order INT NOT NULL DEFAULT 0;

ALTER TABLE content_items
    ADD COLUMN IF NOT EXISTS source_original_id BIGINT REFERENCES content_items(id) ON DELETE SET NULL;
`
	cols := parseAlterTableColumns(content)
	if len(cols["content_items"]) != 3 {
		t.Fatalf("content_items alter columns = %d, want 3: %#v", len(cols["content_items"]), cols["content_items"])
	}
	if len(cols["content_attachments"]) != 1 {
		t.Fatalf("content_attachments alter columns = %d, want 1", len(cols["content_attachments"]))
	}
	coverWidth := cols["content_items"][0]
	if coverWidth.Name != "cover_width" || coverWidth.Type != "INT" {
		t.Fatalf("cover_width = %#v, want name=cover_width type=INT", coverWidth)
	}
	sortOrder := cols["content_attachments"][0]
	if sortOrder.Name != "sort_order" || !sortOrder.NotNull || sortOrder.Default != "0" {
		t.Fatalf("sort_order = %#v, want NOT NULL DEFAULT 0", sortOrder)
	}
	sourceID := cols["content_items"][2]
	if sourceID.Name != "source_original_id" || sourceID.References != "content_items.id" {
		t.Fatalf("source_original_id = %#v, want REFERENCES content_items.id", sourceID)
	}
}

func TestParseAlterTableForeignKeysMergesBackfilledReferences(t *testing.T) {
	// #745 形态：088 的孤儿清理 + DROP IF EXISTS + 命名 ADD CONSTRAINT FK。
	content := `
DELETE FROM content_usage_guide_cache AS cache
WHERE NOT EXISTS (
    SELECT 1 FROM content_items AS item WHERE item.id = cache.content_id
);

ALTER TABLE content_usage_guide_cache DROP CONSTRAINT IF EXISTS fk_usage_guide_cache_content;
ALTER TABLE content_usage_guide_cache
    ADD CONSTRAINT fk_usage_guide_cache_content
    FOREIGN KEY (content_id) REFERENCES content_items(id) ON DELETE CASCADE;
`
	fks := parseAlterTableForeignKeys(content)
	if len(fks["content_usage_guide_cache"]) != 1 {
		t.Fatalf("content_usage_guide_cache foreign keys = %#v, want one", fks)
	}
	if fks["content_usage_guide_cache"]["content_id"] != "content_items.id" {
		t.Fatalf("content_id reference = %q, want content_items.id", fks["content_usage_guide_cache"]["content_id"])
	}
}

func TestParseAlterTableForeignKeysAcceptUnnamedConstraintAndSkipsInlineWins(t *testing.T) {
	content := `
ALTER TABLE feedback_tickets
    ADD FOREIGN KEY (author_id) REFERENCES users(id);

ALTER TABLE content_items
    ADD CONSTRAINT fk_content_items_author
    FOREIGN KEY (author_id) REFERENCES users(id);
`
	fks := parseAlterTableForeignKeys(content)
	if fks["feedback_tickets"]["author_id"] != "users.id" {
		t.Fatalf("unnamed constraint form not parsed: %#v", fks)
	}
	if fks["content_items"]["author_id"] != "users.id" {
		t.Fatalf("named constraint form not parsed: %#v", fks)
	}
}

// 089 形态（#854）：ALTER COLUMN ... DROP NOT NULL 必须清掉 CREATE TABLE 里
// 的 NOT NULL 标记——否则快照与真源 DDL 相悖（two-axis review MAJOR-1）。
func TestParseAlterTableDropNotNullClearsFlag(t *testing.T) {
	content := `
CREATE TABLE IF NOT EXISTS agent_conversations (
    id      BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE agent_conversations ALTER COLUMN user_id DROP NOT NULL; -- 089 起可空；guest 行 NULL
`
	tables, err := parseMigrationsFrom(content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	td := findTable(tables, "agent_conversations")
	if td == nil {
		t.Fatal("agent_conversations missing")
	}
	for _, col := range td.Columns {
		if col.Name == "user_id" && col.NotNull {
			t.Fatal("user_id must lose NOT NULL after ALTER COLUMN DROP NOT NULL")
		}
	}
}

// 089 的表级 CHECK（DO $$ 守卫内的 ADD CONSTRAINT ... CHECK）必须渲染为
// table constraint 行，与 UNIQUE 行同惯例。
func TestParseAlterTableCheckConstraintRenders(t *testing.T) {
	content := `
CREATE TABLE IF NOT EXISTS agent_conversations (
    id BIGSERIAL PRIMARY KEY
);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'owner_check') THEN
        ALTER TABLE agent_conversations ADD CONSTRAINT agent_conversations_owner_check CHECK (
            (is_guest = FALSE AND user_id IS NOT NULL AND guest_device_key IS NULL)
            OR (is_guest = TRUE AND user_id IS NULL AND guest_device_key IS NOT NULL)
        );
    END IF;
END $$;
`
	tables, err := parseMigrationsFrom(content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	td := findTable(tables, "agent_conversations")
	if td == nil {
		t.Fatal("agent_conversations missing")
	}
	generated := generateSchemaTable(tables)
	if !strings.Contains(generated, "CHECK (") || !strings.Contains(generated, "table constraint |") {
		t.Fatalf("generated = %s, want a CHECK table-constraint row", generated)
	}
	if !strings.Contains(generated, "is_guest = FALSE") || !strings.Contains(generated, "guest_device_key IS NOT NULL") {
		t.Fatalf("CHECK body must survive paren-nested extraction, got: %s", generated)
	}
}
