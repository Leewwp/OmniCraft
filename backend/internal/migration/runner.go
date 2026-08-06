package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// advisoryLockKey is the fixed session advisory lock key all migration
// runners acquire so only one runner mutates a database at a time.
const advisoryLockKey int64 = 5716435830911775558 // "OMNICRAF" as int64

// Metadata describes the transactional contract of the migrations directory.
type Metadata struct {
	SchemaVersion        int  `json:"schema_version"`
	DefaultTransactional bool `json:"default_transactional"`
	// SelfTransactional lists files that manage their own BEGIN/COMMIT and
	// are therefore executed in autocommit mode (they are atomic by their own
	// transaction control, so no attempt audit is needed).
	SelfTransactional []NonTransactionalMigration `json:"self_transactional"`
	NonTransactional  []NonTransactionalMigration `json:"non_transactional"`
}

// NonTransactionalMigration declares an explicitly reviewed migration that
// cannot run inside a transaction (for example CREATE INDEX CONCURRENTLY).
type NonTransactionalMigration struct {
	Filename         string         `json:"filename"`
	Reason           string         `json:"reason"`
	Reviewer         string         `json:"reviewer"`
	ReviewedAt       string         `json:"reviewed_at"`
	Preconditions    []SQLCondition `json:"preconditions"`
	Postconditions   []SQLCondition `json:"postconditions"`
	IdempotentResume string         `json:"idempotent_resume"`
	Reconciliation   string         `json:"reconciliation"`
}

// SQLCondition is a machine-checkable migration condition. Query must return
// exactly one boolean; false blocks the migration before its ledger row is
// written.
type SQLCondition struct {
	Description string `json:"description"`
	Query       string `json:"query"`
}

// LoadMetadata parses and structurally validates metadata.json.
func LoadMetadata(path string) (*Metadata, error) {
	contents, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read migration metadata %s: %w", path, err)
	}
	var meta Metadata
	if err := json.Unmarshal(contents, &meta); err != nil {
		return nil, fmt.Errorf("parse migration metadata %s: %w", path, err)
	}
	if meta.SchemaVersion < 1 {
		return nil, fmt.Errorf("migration metadata %s: schema_version must be >= 1", path)
	}
	if !meta.DefaultTransactional {
		return nil, fmt.Errorf("migration metadata %s: default_transactional must be true", path)
	}
	for _, list := range [][]NonTransactionalMigration{meta.SelfTransactional, meta.NonTransactional} {
		for _, entry := range list {
			if entry.Filename == "" || entry.Reason == "" || entry.Reviewer == "" || entry.ReviewedAt == "" {
				return nil, fmt.Errorf("migration metadata %s: declared migration must include filename, reason, reviewer and reviewed_at", path)
			}
			if len(entry.Preconditions) == 0 || len(entry.Postconditions) == 0 {
				return nil, fmt.Errorf("migration metadata %s: %s must declare machine-checkable preconditions and postconditions", path, entry.Filename)
			}
			for _, condition := range append(append([]SQLCondition(nil), entry.Preconditions...), entry.Postconditions...) {
				if strings.TrimSpace(condition.Description) == "" || strings.TrimSpace(condition.Query) == "" {
					return nil, fmt.Errorf("migration metadata %s: %s conditions must include description and boolean query", path, entry.Filename)
				}
			}
			if entry.IdempotentResume == "" || entry.Reconciliation == "" {
				return nil, fmt.Errorf("migration metadata %s: %s must declare idempotent/resume strategy and reconciliation steps", path, entry.Filename)
			}
		}
	}
	return &meta, nil
}

// Plan is the ordered set of migrations that must still be applied.
type Plan struct {
	ToApply []File
}

// BuildPlan compares the applied ledger with the repository files and returns
// the stable ordered set of missing migrations. It rejects checksum drift on
// applied files, ledger entries whose file disappeared from the repository,
// and duplicate versions in the file set.
func BuildPlan(ledger *Ledger, files []File) (Plan, error) {
	seen := make(map[int]string)
	for _, file := range files {
		if previous, ok := seen[file.Version]; ok {
			return Plan{}, fmt.Errorf("duplicate migration version %03d: %s and %s", file.Version, previous, file.Filename)
		}
		seen[file.Version] = file.Filename
	}

	for _, entry := range ledger.Entries() {
		file, ok := findFile(files, entry.Filename)
		if !ok {
			return Plan{}, fmt.Errorf("ledger references %s which is missing from the migrations directory", entry.Filename)
		}
		if file.Version != entry.Version {
			return Plan{}, fmt.Errorf("version drift for applied migration %s: ledger %03d, file %03d", entry.Filename, entry.Version, file.Version)
		}
		if file.Checksum != entry.Checksum {
			return Plan{}, fmt.Errorf("checksum drift for applied migration %s: ledger %s, file %s", entry.Filename, entry.Checksum, file.Checksum)
		}
	}

	var toApply []File
	for _, file := range files {
		if _, applied := ledger.Checksum(file.Filename); !applied {
			toApply = append(toApply, file)
		}
	}
	sort.Slice(toApply, func(i, j int) bool {
		if toApply[i].Version != toApply[j].Version {
			return toApply[i].Version < toApply[j].Version
		}
		return toApply[i].Filename < toApply[j].Filename
	})
	return Plan{ToApply: toApply}, nil
}

func findFile(files []File, filename string) (File, bool) {
	for _, file := range files {
		if file.Filename == filename {
			return file, true
		}
	}
	return File{}, false
}

// Result describes one migration run.
type Result struct {
	Applied    []string
	Skipped    int
	StartedAt  time.Time
	FinishedAt time.Time
}

// Runner applies forward-only migrations against a database under a session
// advisory lock. Migrations run transactionally by default; migrations that
// manage their own transaction run in autocommit mode; migrations declared in
// metadata.json as non-transactional additionally record audit attempts and
// block retry until an operator explicitly reconciles a failed version.
type Runner struct {
	DB       *sql.DB
	Dir      string
	Metadata *Metadata
	// ReconcileVersions are versions whose failed/unknown attempts the
	// operator explicitly approves for retry. Unlock only with evidence.
	ReconcileVersions map[int]bool
	// ReconciliationApproval is a ticket, incident, change request, or other
	// durable approval reference recorded with every reconciled attempt.
	ReconciliationApproval string
}

// Run executes the migration plan. Run is safe for concurrent invocation:
// only one runner holds the advisory lock at a time.
func (r *Runner) Run(ctx context.Context) (result Result, resultErr error) {
	result = Result{StartedAt: time.Now().UTC()}
	defer func() { result.FinishedAt = time.Now().UTC() }()

	conn, err := r.DB.Conn(ctx)
	if err != nil {
		return result, fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
		return result, fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	defer conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", advisoryLockKey)

	if err := EnsureSchema(ctx, conn); err != nil {
		return result, err
	}

	ledger, err := LoadLedger(ctx, conn)
	if err != nil {
		return result, err
	}
	files, err := ScanFiles(r.Dir)
	if err != nil {
		return result, err
	}
	plan, err := BuildPlan(ledger, files)
	if err != nil {
		return result, err
	}

	if err := r.resolveAttempts(ctx, conn); err != nil {
		return result, err
	}

	nonTransactional := r.nonTransactionalMigrations()
	selfManaged := r.selfTransactionalMigrations()
	for _, file := range plan.ToApply {
		result.Applied = append(result.Applied, file.Filename)
		if err := r.applyOne(ctx, conn, file, nonTransactional[file.Filename], selfManaged[file.Filename]); err != nil {
			return result, err
		}
	}
	result.Skipped = len(files) - len(plan.ToApply)
	return result, nil
}

// Plan computes the ordered set of migrations that would be applied without
// acquiring the lock or touching the database beyond a read of the ledger.
func (r *Runner) Plan(ctx context.Context) ([]File, error) {
	conn, err := r.DB.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Close()

	if err := EnsureSchema(ctx, conn); err != nil {
		return nil, err
	}
	ledger, err := LoadLedger(ctx, conn)
	if err != nil {
		return nil, err
	}
	files, err := ScanFiles(r.Dir)
	if err != nil {
		return nil, err
	}
	plan, err := BuildPlan(ledger, files)
	if err != nil {
		return nil, err
	}
	return plan.ToApply, nil
}

// Verify checks that the ledger exactly matches the repository file set
// without applying anything. It is used after a restore to prove the
// restored database is complete and undrifted.
func (r *Runner) Verify(ctx context.Context) error {
	conn, err := r.DB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
		return fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	defer conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", advisoryLockKey)

	if err := EnsureSchema(ctx, conn); err != nil {
		return err
	}
	ledger, err := LoadLedger(ctx, conn)
	if err != nil {
		return err
	}
	files, err := ScanFiles(r.Dir)
	if err != nil {
		return err
	}
	if _, err := BuildPlan(ledger, files); err != nil {
		return err
	}
	applied := len(ledger.Entries())
	if applied != len(files) {
		return fmt.Errorf("ledger has %d applied migrations, repository has %d", applied, len(files))
	}
	return nil
}

func (r *Runner) nonTransactionalMigrations() map[string]*NonTransactionalMigration {
	names := make(map[string]*NonTransactionalMigration)
	for _, entry := range r.Metadata.NonTransactional {
		entry := entry
		names[entry.Filename] = &entry
	}
	return names
}

func (r *Runner) selfTransactionalMigrations() map[string]*NonTransactionalMigration {
	names := make(map[string]*NonTransactionalMigration)
	for _, entry := range r.Metadata.SelfTransactional {
		entry := entry
		names[entry.Filename] = &entry
	}
	return names
}

// resolveAttempts blocks blind retries: any started/failed attempt for a
// version that is not yet applied stops the whole run until the operator
// explicitly reconciles every blocked version.
func (r *Runner) resolveAttempts(ctx context.Context, conn *sql.Conn) error {
	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`
		SELECT a.version, a.filename, a.checksum
		FROM %s a
		WHERE a.status IN ('started', 'failed')
		  AND NOT EXISTS (
		    SELECT 1 FROM %s r
		    WHERE r.status = 'reconciled'
		      AND r.version = a.version
		      AND r.filename = a.filename
		      AND r.checksum = a.checksum
		      AND r.id > a.id
		  )
		ORDER BY a.version, a.id`, attemptsTable, attemptsTable))
	if err != nil {
		return fmt.Errorf("query migration attempts: %w", err)
	}
	defer rows.Close()

	type blocked struct {
		version  int
		filename string
		checksum string
	}
	var pending []blocked
	for rows.Next() {
		var version int
		var filename, checksum string
		if err := rows.Scan(&version, &filename, &checksum); err != nil {
			return fmt.Errorf("scan migration attempt: %w", err)
		}
		pending = append(pending, blocked{version: version, filename: filename, checksum: checksum})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate migration attempts: %w", err)
	}
	if len(pending) == 0 {
		return nil
	}

	var unreconciled []string
	for _, attempt := range pending {
		if r.ReconcileVersions[attempt.version] {
			if strings.TrimSpace(r.ReconciliationApproval) == "" {
				return fmt.Errorf("reconciliation approval reference is required for version %03d", attempt.version)
			}
			if _, err := conn.ExecContext(ctx, fmt.Sprintf(
				"INSERT INTO %s (version, filename, checksum, status, note, approval_ref) VALUES ($1, $2, $3, 'reconciled', 'operator approved reconciliation', $4)",
				attemptsTable), attempt.version, attempt.filename, attempt.checksum, r.ReconciliationApproval); err != nil {
				return fmt.Errorf("record reconciliation for version %03d: %w", attempt.version, err)
			}
			continue
		}
		unreconciled = append(unreconciled, fmt.Sprintf("%03d (%s)", attempt.version, attempt.filename))
	}
	if len(unreconciled) > 0 {
		return fmt.Errorf("migration attempts require reconciliation: versions %s; a failed/unknown attempt blocks blind retry and later migrations until the operator approves with -ReconcileVersions",
			strings.Join(unreconciled, ", "))
	}
	return nil
}

func (r *Runner) applyOne(ctx context.Context, conn *sql.Conn, file File, nonTransactional, selfManaged *NonTransactionalMigration) error {
	statements, err := readStatements(file.Path)
	if err != nil {
		return err
	}
	if len(statements) == 0 {
		return fmt.Errorf("migration %s is empty", file.Filename)
	}

	if isSelfManagedTransaction(statements[0]) {
		// A file that manages its own BEGIN/COMMIT must be declared in
		// metadata.json so the transactional contract stays reviewed and
		// explicit rather than silently detected.
		if selfManaged == nil {
			return fmt.Errorf("migration %s manages its own transaction but is not declared in metadata.json self_transactional", file.Filename)
		}
		return r.applySelfManaged(ctx, conn, file, statements, selfManaged)
	}
	if nonTransactional != nil {
		return r.applyAutoCommit(ctx, conn, file, statements, nonTransactional)
	}
	return r.applyTransactional(ctx, conn, file, statements)
}

func (r *Runner) applySelfManaged(ctx context.Context, conn *sql.Conn, file File, statements []string, contract *NonTransactionalMigration) error {
	commitIndex := -1
	for i := len(statements) - 1; i > 0; i-- {
		if isCommitStatement(statements[i]) {
			commitIndex = i
			break
		}
	}
	if commitIndex < 1 {
		return fmt.Errorf("migration %s must end with COMMIT", file.Filename)
	}
	for _, trailing := range statements[commitIndex+1:] {
		if !isCommentOnlyStatement(trailing) {
			return fmt.Errorf("migration %s contains executable SQL after COMMIT", file.Filename)
		}
	}
	if err := checkConditions(ctx, conn, file.Filename, "precondition", contract.Preconditions); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, statements[0]); err != nil {
		return fmt.Errorf("begin self-managed migration %s: %w", file.Filename, err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	for _, statement := range statements[1:commitIndex] {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply migration %s: %w", file.Filename, err)
		}
	}
	if err := checkConditions(ctx, conn, file.Filename, "postcondition", contract.Postconditions); err != nil {
		return err
	}
	if err := r.recordLedger(ctx, conn, file); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, statements[commitIndex]); err != nil {
		return fmt.Errorf("commit self-managed migration %s: %w", file.Filename, err)
	}
	committed = true
	return nil
}

func (r *Runner) applyTransactional(ctx context.Context, conn *sql.Conn, file File, statements []string) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration transaction %s: %w", file.Filename, err)
	}
	defer tx.Rollback()

	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply migration %s: %w", file.Filename, err)
		}
	}
	if err := r.recordLedger(ctx, tx, file); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", file.Filename, err)
	}
	return nil
}

// applyAutoCommit runs a migration whose statements each commit immediately:
// either the file manages its own BEGIN/COMMIT, or it is declared
// non-transactional in metadata.json and therefore tracked in the attempts
// audit table.
func (r *Runner) applyAutoCommit(ctx context.Context, conn *sql.Conn, file File, statements []string, contract *NonTransactionalMigration) error {
	if err := checkConditions(ctx, conn, file.Filename, "precondition", contract.Preconditions); err != nil {
		return err
	}
	var attemptID int64
	attemptID, err := r.recordAttempt(ctx, conn, file, "started", "", "", "")
	if err != nil {
		return err
	}

	for _, statement := range statements {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			class, digest := redactError(err)
			if updateErr := r.updateAttempt(ctx, conn, attemptID, "failed", class, digest); updateErr != nil {
				return fmt.Errorf("apply migration %s: %w (record attempt: %v)", file.Filename, err, updateErr)
			}
			return fmt.Errorf("apply migration %s: %w", file.Filename, err)
		}
	}
	if err := checkConditions(ctx, conn, file.Filename, "postcondition", contract.Postconditions); err != nil {
		class, digest := redactError(err)
		if updateErr := r.updateAttempt(ctx, conn, attemptID, "failed", class, digest); updateErr != nil {
			return fmt.Errorf("%w (record attempt: %v)", err, updateErr)
		}
		return err
	}

	if _, err := conn.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (version, filename, checksum, applied_at) VALUES ($1, $2, $3, NOW())",
		ledgerTable), file.Version, file.Filename, file.Checksum); err != nil {
		return fmt.Errorf("record migration ledger %s: %w", file.Filename, err)
	}
	if err := r.updateAttempt(ctx, conn, attemptID, "succeeded", "", ""); err != nil {
		return err
	}
	return nil
}

func checkConditions(ctx context.Context, conn *sql.Conn, filename, kind string, conditions []SQLCondition) error {
	for _, condition := range conditions {
		var ok bool
		if err := conn.QueryRowContext(ctx, condition.Query).Scan(&ok); err != nil {
			return fmt.Errorf("migration %s %s %q could not be evaluated: %w", filename, kind, condition.Description, err)
		}
		if !ok {
			return fmt.Errorf("migration %s %s failed: %s", filename, kind, condition.Description)
		}
	}
	return nil
}

func (r *Runner) recordLedger(ctx context.Context, execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, file File) error {
	if _, err := execer.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (version, filename, checksum, applied_at) VALUES ($1, $2, $3, NOW())",
		ledgerTable), file.Version, file.Filename, file.Checksum); err != nil {
		return fmt.Errorf("record migration ledger %s: %w", file.Filename, err)
	}
	return nil
}

func (r *Runner) recordAttempt(ctx context.Context, conn *sql.Conn, file File, status, class, digest, note string) (int64, error) {
	var id int64
	err := conn.QueryRowContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (version, filename, checksum, status, error_class, error_digest, note) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id",
		attemptsTable), file.Version, file.Filename, file.Checksum, status, class, digest, note).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("record migration attempt for %s: %w", file.Filename, err)
	}
	return id, nil
}

func (r *Runner) updateAttempt(ctx context.Context, conn *sql.Conn, id int64, status, class, digest string) error {
	if _, err := conn.ExecContext(ctx, fmt.Sprintf(
		"UPDATE %s SET status = $1, error_class = $2, error_digest = $3 WHERE id = $4",
		attemptsTable), status, class, digest, id); err != nil {
		return fmt.Errorf("update migration attempt %d: %w", id, err)
	}
	return nil
}

// redactError converts an SQL error into a redacted audit summary: an error
// class plus a SHA-256 digest. Raw error text never reaches the audit table.
func redactError(err error) (class, digest string) {
	message := err.Error()
	if idx := strings.IndexByte(message, ' '); idx > 0 {
		class = message[:idx]
	} else {
		class = message
	}
	sum := sha256.Sum256([]byte(message))
	return class, hex.EncodeToString(sum[:])
}

// isSelfManagedTransaction reports whether the first statement opens an
// explicit transaction, meaning the file manages its own BEGIN/COMMIT.
func isSelfManagedTransaction(first string) bool {
	trimmed := strings.ToUpper(strings.TrimSpace(first))
	return strings.HasPrefix(trimmed, "BEGIN")
}

func isCommitStatement(statement string) bool {
	trimmed := strings.TrimSpace(strings.TrimSuffix(statement, ";"))
	return strings.EqualFold(trimmed, "COMMIT")
}

func isCommentOnlyStatement(statement string) bool {
	for _, line := range strings.Split(statement, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "--") {
			return false
		}
	}
	return true
}

func readStatements(path string) ([]string, error) {
	contents, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read migration %s: %w", path, err)
	}
	statements := splitSQLStatements(string(contents))
	return statements, nil
}

// splitSQLStatements splits SQL text on top-level semicolons while honoring
// single/double quotes, line/block comments and dollar-quoted bodies.
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
