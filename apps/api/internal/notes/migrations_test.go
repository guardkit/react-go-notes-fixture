package notes

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The schema PGStore reads and writes comes from apps/api/migrations: scripts/
// migrate.sh applies the files in file-name order, each one inside a single
// transaction, and records the file's version in schema_migrations. The API
// never migrates on start, so the tree is the whole of the storage contract -
// these tests pin what the tree must keep saying.

// migrationsDir is the tree relative to this package, the same folder
// scripts/migrate.sh globs for [0-9][0-9][0-9][0-9]_*.sql.
var migrationsDir = filepath.Join("..", "..", "migrations")

// migrationName is the file-name rule scripts/migrate.sh enforces before it
// applies a file: a four-digit version and a lower-case snake name.
var migrationName = regexp.MustCompile(`^[0-9]{4}_[a-z0-9_]+$`)

// txnControl names the statements and options that cannot go through the
// `psql --single-transaction -f -` pipe migrate.sh uses: opening a second
// transaction, or CREATE INDEX CONCURRENTLY, fails the apply - and with it the
// schema_migrations row that commits in the same transaction.
var txnControl = regexp.MustCompile(`(?i)\b(begin|commit|rollback|savepoint|concurrently)\b`)

// doneColumn captures the clause of the statement that adds the completion
// flag, so its nullability and default are read from the statement itself
// rather than matched as one fixed string.
var doneColumn = regexp.MustCompile(`(?is)alter\s+table\s+notes\s+add\s+column\s+(?:if\s+not\s+exists\s+)?done\b([^;]*)`)

// migrationFiles lists the migration file names in the order migrate.sh
// applies them.
func migrationFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read the migrations folder: %v", err)
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatal("the migrations folder holds no [0-9][0-9][0-9][0-9]_*.sql file")
	}
	return files
}

// migrationBody returns one file with its `--` comments removed, so a check
// cannot be satisfied by prose in a header.
func migrationBody(t *testing.T, file string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(migrationsDir, file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	lines := strings.Split(string(raw), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		if strings.TrimSpace(line) != "" {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

func TestMigrationsAreNamedAndOrderedForTheApplyScript(t *testing.T) {
	previous := ""
	for _, file := range migrationFiles(t) {
		version := strings.TrimSuffix(file, ".sql")
		if !migrationName.MatchString(version) {
			t.Errorf("%s is not a name scripts/migrate.sh accepts: want NNNN_name.sql", file)
		}
		if version <= previous {
			t.Errorf("%s does not sort after %s; migrate.sh applies the tree in file-name order", file, previous)
		}
		previous = version
	}
	if first := migrationFiles(t)[0]; !strings.HasPrefix(first, "0001_") {
		t.Errorf("the tree starts at %s, not at a 0001 baseline", first)
	}
}

func TestEveryMigrationRunsInASingleTransaction(t *testing.T) {
	for _, file := range migrationFiles(t) {
		body := migrationBody(t, file)
		if word := txnControl.FindString(body); word != "" {
			t.Errorf("%s uses %q, which scripts/migrate.sh cannot pipe through psql --single-transaction", file, word)
		}
		if strings.Contains(body, `\`) {
			t.Errorf("%s holds a psql meta-command; migrate.sh pipes the file through -f -", file)
		}
		if trimmed := strings.TrimSpace(body); trimmed == "" {
			t.Errorf("%s states nothing", file)
		} else if !strings.HasSuffix(trimmed, ";") {
			// migrate.sh appends the schema_migrations INSERT to the same
			// stream, so a statement left unterminated would swallow it.
			t.Errorf("%s does not end its last statement with a semicolon", file)
		}
	}
}

func TestDoneMigrationFollowsTheCreatedAtMigration(t *testing.T) {
	files := migrationFiles(t)
	done, createdAt := -1, -1
	for i, file := range files {
		switch {
		case strings.HasPrefix(file, "0003_") && strings.Contains(file, "done"):
			done = i
		case strings.HasPrefix(file, "0002_"):
			createdAt = i
		}
	}
	if done < 0 {
		t.Fatalf("no 0003_*done*.sql migration in %s", strings.Join(files, ", "))
	}
	if createdAt < 0 {
		t.Fatal("the 0002 created_at migration disappeared from the tree")
	}
	if want := createdAt + 1; done != want {
		t.Errorf("%s is applied at position %d, immediately after %s it should be at %d",
			files[done], done+1, files[createdAt], want+1)
	}
}

func TestDoneColumnIsNotNullAndDefaultsToFalse(t *testing.T) {
	var clauses []string
	var owners []string
	for _, file := range migrationFiles(t) {
		for _, match := range doneColumn.FindAllStringSubmatch(migrationBody(t, file), -1) {
			clauses = append(clauses, match[1])
			owners = append(owners, file)
		}
	}
	if len(clauses) != 1 {
		t.Fatalf("the notes table gains its done column %d times (%s); want exactly once",
			len(clauses), strings.Join(owners, ", "))
	}
	if owners[0] != "0003_add_done.sql" {
		t.Errorf("done is added by %s, want 0003_add_done.sql", owners[0])
	}
	clause := strings.ToUpper(clauses[0])
	if !regexp.MustCompile(`\bBOOLEAN\b|\bBOOL\b`).MatchString(clause) {
		t.Errorf("done is not a boolean: %q", clauses[0])
	}
	if !regexp.MustCompile(`NOT\s+NULL`).MatchString(clause) {
		t.Errorf("done is nullable, so a row could read back as NULL instead of false: %q", clauses[0])
	}
	if !regexp.MustCompile(`DEFAULT\s+FALSE\b`).MatchString(clause) {
		t.Errorf("done has no false default, so rows created before the migration could not read back as false: %q",
			clauses[0])
	}
}
