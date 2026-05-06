package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContentSourceMigrationIncludesPublishBodyFields(t *testing.T) {
	root := filepath.Join("..", "..", "migrations")
	bytes, err := os.ReadFile(filepath.Join(root, "036_content_source_original.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := strings.ToLower(string(bytes))

	for _, want := range []string{
		"source_original_id",
		"description",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration 036 must include %s so publish schema matches ContentItem", want)
		}
	}
}
