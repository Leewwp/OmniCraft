package testutil

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const defaultAdminDSN = "host=127.0.0.1 port=5432 user=omnicraft password=omnicraft dbname=postgres sslmode=disable"

func newEphemeralDatabaseName() (string, error) {
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", fmt.Errorf("generate ephemeral database name: %w", err)
	}
	return "omnicraft_test_" + hex.EncodeToString(entropy[:]), nil
}

func OpenEphemeralPostgres(t *testing.T) *gorm.DB {
	t.Helper()

	adminDSN := os.Getenv("OMNICRAFT_TEST_POSTGRES_ADMIN_DSN")
	if strings.TrimSpace(adminDSN) == "" {
		adminDSN = defaultAdminDSN
	}
	if err := validateEphemeralAdminDSN(adminDSN); err != nil {
		t.Fatalf("unsafe postgres admin DSN: %v", err)
	}

	adminDB, err := sql.Open("postgres", adminDSN)
	if err != nil {
		t.Fatalf("open postgres admin connection: %v", err)
	}
	if err := adminDB.Ping(); err != nil {
		t.Fatalf("ping postgres admin connection: %v", err)
	}

	dbName, err := newEphemeralDatabaseName()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adminDB.Exec(`CREATE DATABASE "` + dbName + `"`); err != nil {
		t.Fatalf("create temp database %s: %v", dbName, err)
	}

	testDSN := rewriteDSNDatabase(adminDSN, dbName)
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{})
	if err != nil {
		t.Fatalf("open temp postgres database: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		_, _ = adminDB.Exec(`
			SELECT pg_terminate_backend(pid)
			FROM pg_stat_activity
			WHERE datname = $1 AND pid <> pg_backend_pid()
		`, dbName)
		_, _ = adminDB.Exec(`DROP DATABASE IF EXISTS "` + dbName + `"`)
		_ = adminDB.Close()
	})

	return db
}

func validateEphemeralAdminDSN(dsn string) error {
	host, database, err := postgresDSNHostAndDatabase(dsn)
	if err != nil {
		return err
	}
	if !isLoopbackPostgresHost(host) {
		return fmt.Errorf("postgres admin DSN host %q must be loopback", host)
	}
	if database != "postgres" {
		return fmt.Errorf("postgres admin DSN database %q must be postgres", database)
	}
	return nil
}

func postgresDSNHostAndDatabase(dsn string) (string, string, error) {
	trimmed := strings.TrimSpace(dsn)
	if strings.HasPrefix(trimmed, "postgres://") || strings.HasPrefix(trimmed, "postgresql://") {
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return "", "", fmt.Errorf("parse postgres URL: %w", err)
		}
		return parsed.Hostname(), strings.TrimPrefix(parsed.EscapedPath(), "/"), nil
	}

	values := postgresDSNKeywordValues(trimmed)
	return values["host"], values["dbname"], nil
}

func postgresDSNKeywordValues(dsn string) map[string]string {
	values := make(map[string]string)
	for _, part := range strings.Fields(dsn) {
		key, value, ok := strings.Cut(part, "=")
		if ok {
			values[strings.ToLower(key)] = strings.Trim(value, "'")
		}
	}
	return values
}

func isLoopbackPostgresHost(host string) bool {
	trimmed := strings.Trim(strings.TrimSpace(host), "[]")
	if strings.EqualFold(trimmed, "localhost") {
		return true
	}
	ip := net.ParseIP(trimmed)
	return ip != nil && ip.IsLoopback()
}

func ApplyMigrationFile(t *testing.T, db *gorm.DB, path string) {
	t.Helper()

	contents, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read migration %s: %v", path, err)
	}
	for _, statement := range splitSQLStatements(string(contents)) {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("apply migration statement %q: %v", statement, err)
		}
	}
}

func ColumnMetadata(t *testing.T, db *gorm.DB, table, column string) (dataType string, nullable bool) {
	t.Helper()

	var row struct {
		DataType   string
		IsNullable string
	}
	if err := db.Raw(`
		SELECT data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = ?
		  AND column_name = ?
	`, table, column).Scan(&row).Error; err != nil {
		t.Fatalf("lookup column %s.%s: %v", table, column, err)
	}
	if row.DataType == "" {
		t.Fatalf("column %s.%s not found", table, column)
	}
	return row.DataType, row.IsNullable == "YES"
}

func IndexExists(t *testing.T, db *gorm.DB, table, index string) bool {
	t.Helper()

	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM pg_indexes
			WHERE schemaname = current_schema()
			  AND tablename = ?
			  AND indexname = ?
		)
	`, table, index).Scan(&exists).Error; err != nil {
		t.Fatalf("lookup index %s on %s: %v", index, table, err)
	}
	return exists
}

func ForeignKeyExists(t *testing.T, db *gorm.DB, table, column, referencedTable string) bool {
	t.Helper()

	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu
			  ON tc.constraint_name = kcu.constraint_name
			 AND tc.table_schema = kcu.table_schema
			JOIN information_schema.constraint_column_usage ccu
			  ON ccu.constraint_name = tc.constraint_name
			 AND ccu.table_schema = tc.table_schema
			WHERE tc.constraint_type = 'FOREIGN KEY'
			  AND tc.table_schema = current_schema()
			  AND tc.table_name = ?
			  AND kcu.column_name = ?
			  AND ccu.table_name = ?
		)
	`, table, column, referencedTable).Scan(&exists).Error; err != nil {
		t.Fatalf("lookup foreign key %s.%s -> %s: %v", table, column, referencedTable, err)
	}
	return exists
}

func rewriteDSNDatabase(dsn, dbName string) string {
	trimmed := strings.TrimSpace(dsn)
	if strings.HasPrefix(trimmed, "postgres://") || strings.HasPrefix(trimmed, "postgresql://") {
		parsed, err := url.Parse(trimmed)
		if err == nil {
			parsed.Path = "/" + dbName
			return parsed.String()
		}
	}

	parts := strings.Fields(dsn)
	values := map[string]string{}
	for _, part := range parts {
		key, value, ok := strings.Cut(part, "=")
		if ok {
			values[key] = value
		}
	}
	values["dbname"] = dbName

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	rebuilt := make([]string, 0, len(keys))
	for _, key := range keys {
		rebuilt = append(rebuilt, fmt.Sprintf("%s=%s", key, values[key]))
	}
	return strings.Join(rebuilt, " ")
}

func splitSQLStatements(sqlText string) []string {
	var (
		statements     []string
		current        strings.Builder
		inSingleQuote  bool
		inDoubleQuote  bool
		inLineComment  bool
		inBlockComment bool
		dollarQuoteTag string
	)

	for i := 0; i < len(sqlText); i++ {
		ch := sqlText[i]
		next := byte(0)
		if i+1 < len(sqlText) {
			next = sqlText[i+1]
		}

		if inLineComment {
			current.WriteByte(ch)
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			current.WriteByte(ch)
			if ch == '*' && next == '/' {
				current.WriteByte(next)
				i++
				inBlockComment = false
			}
			continue
		}

		if dollarQuoteTag != "" {
			current.WriteByte(ch)
			if ch == '$' && strings.HasPrefix(sqlText[i:], dollarQuoteTag) {
				current.WriteString(dollarQuoteTag[1:])
				i += len(dollarQuoteTag) - 1
				dollarQuoteTag = ""
			}
			continue
		}

		if inSingleQuote {
			current.WriteByte(ch)
			if ch == '\'' {
				if next == '\'' {
					current.WriteByte(next)
					i++
				} else {
					inSingleQuote = false
				}
			}
			continue
		}

		if inDoubleQuote {
			current.WriteByte(ch)
			if ch == '"' {
				inDoubleQuote = false
			}
			continue
		}

		if ch == '-' && next == '-' {
			current.WriteByte(ch)
			current.WriteByte(next)
			i++
			inLineComment = true
			continue
		}
		if ch == '/' && next == '*' {
			current.WriteByte(ch)
			current.WriteByte(next)
			i++
			inBlockComment = true
			continue
		}
		if ch == '\'' {
			inSingleQuote = true
			current.WriteByte(ch)
			continue
		}
		if ch == '"' {
			inDoubleQuote = true
			current.WriteByte(ch)
			continue
		}
		if ch == '$' {
			if tag, ok := consumeDollarQuoteTag(sqlText[i:]); ok {
				dollarQuoteTag = tag
				current.WriteString(tag)
				i += len(tag) - 1
				continue
			}
		}
		if ch == ';' {
			trimmed := strings.TrimSpace(current.String())
			if trimmed != "" {
				statements = append(statements, trimmed)
			}
			current.Reset()
			continue
		}
		current.WriteByte(ch)
	}

	if trimmed := strings.TrimSpace(current.String()); trimmed != "" {
		statements = append(statements, trimmed)
	}
	return statements
}

func consumeDollarQuoteTag(input string) (string, bool) {
	if len(input) < 2 || input[0] != '$' {
		return "", false
	}
	for i := 1; i < len(input); i++ {
		if input[i] == '$' {
			tag := input[:i+1]
			for _, r := range tag[1 : len(tag)-1] {
				if r != '_' && !isASCIILetter(r) && !isASCIIDigit(r) {
					return "", false
				}
			}
			return tag, true
		}
	}
	return "", false
}

func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}
