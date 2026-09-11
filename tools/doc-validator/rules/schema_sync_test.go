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
