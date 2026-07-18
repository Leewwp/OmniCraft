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
