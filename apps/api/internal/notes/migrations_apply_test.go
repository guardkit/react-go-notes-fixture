package notes

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The connected check applies the tree three ways - an upgrade from 0001 with
// rows already in the table, a fresh install, and a re-run that must apply
// nothing - and then asserts schema_migrations holds one row per file. Those
// steps need a container, so this drives the very same scripts/migrate.sh
// against the stub psql in testdata instead: the stub keeps the applied
// versions and the added columns per database and refuses what PostgreSQL
// would refuse, so a migration that re-applied itself, recorded a version
// twice, or added a NOT NULL column no row could satisfy fails here too.

// stubScript is the stand-in for `docker compose exec ... psql`.
const stubScript = "testdata/docker-psql-stub.sh"

// migrateStub is the stubbed environment scripts/migrate.sh runs in.
type migrateStub struct {
	t     *testing.T
	root  string
	shim  string
	state string
}

func newMigrateStub(t *testing.T) *migrateStub {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("locate the repository root: %v", err)
	}
	body, err := os.ReadFile(stubScript)
	if err != nil {
		t.Fatalf("read the psql stub: %v", err)
	}
	shim := t.TempDir()
	if err := os.WriteFile(filepath.Join(shim, "docker"), body, 0o755); err != nil {
		t.Fatalf("install the psql stub: %v", err)
	}
	return &migrateStub{t: t, root: root, shim: shim, state: t.TempDir()}
}

// env is the whole environment of a stubbed run: nothing is inherited from the
// host, so the outcome cannot move with the machine running the test.
func (s *migrateStub) env() []string {
	return []string{
		"PATH=" + s.shim + ":/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"LANG=C",
		"LC_ALL=C",
		"COMPOSE_PROJECT_NAME=notes-go-test",
		"NOTES_PSQL_STUB_STATE=" + s.state,
	}
}

// run starts one command of the stubbed environment with a pinned environment
// and the repository root as its working folder.
func (s *migrateStub) run(name string, args ...string) (string, error) {
	s.t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = s.root
	cmd.Env = s.env()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// migrate runs scripts/migrate.sh and returns its output.
func (s *migrateStub) migrate(args ...string) string {
	s.t.Helper()
	out, err := s.run("bash", append([]string{filepath.Join(s.root, "scripts", "migrate.sh")}, args...)...)
	if err != nil {
		s.t.Fatalf("scripts/migrate.sh %v: %v\n%s", args, err, out)
	}
	return out
}

// psql sends one statement through the stub the way scripts/check-feature.sh
// does, and reports the stub's answer and exit status.
func (s *migrateStub) psql(database, statement string) (string, error) {
	s.t.Helper()
	return s.run(filepath.Join(s.shim, "docker"), "compose", "exec", "-T", "postgres",
		"psql", "-X", "-q", "-At", "-v", "ON_ERROR_STOP=1", "-U", "notes_app", "-d", database,
		"-c", statement)
}

func (s *migrateStub) mustPSQL(database, statement string) string {
	s.t.Helper()
	out, err := s.psql(database, statement)
	if err != nil {
		s.t.Fatalf("statement on %s: %v\n%s", database, err, out)
	}
	return strings.TrimSpace(out)
}

func (s *migrateStub) lines(name string) []string {
	s.t.Helper()
	raw, err := os.ReadFile(filepath.Join(s.state, name))
	if err != nil {
		s.t.Fatalf("read stub state %s: %v", name, err)
	}
	body := strings.TrimSpace(string(raw))
	if body == "" {
		return nil
	}
	return strings.Split(body, "\n")
}

// TestMigrateAppliesTheTreeTheWayTheConnectedCheckDoes mirrors step 3 of
// scripts/check-feature.sh: upgrade from 0001 with seeded rows, fresh install,
// idempotent re-run, and one schema_migrations row per migration file.
func TestMigrateAppliesTheTreeTheWayTheConnectedCheckDoes(t *testing.T) {
	files := migrationFiles(t)
	stub := newMigrateStub(t)

	// 1. Upgrade: 0001 only, rows seeded, then the rest of the tree.
	out := stub.migrate("--database", "notes_upgrade", "--to", "0001")
	if want := "migrate: 1 applied to notes_upgrade"; !strings.Contains(out, want) {
		t.Fatalf("applying to 0001: want %q in:\n%s", want, out)
	}
	if got := stub.lines("notes_upgrade.versions"); len(got) != 1 || !strings.HasPrefix(got[0], "0001_") {
		t.Fatalf("the upgrade database holds %v before the rest of the tree", got)
	}
	seed := "INSERT INTO notes (owner_sub, label) VALUES " +
		"('upgrade:a', 'kept one'), ('upgrade:a', 'kept two'), ('upgrade:b', 'kept three')"
	if got := stub.mustPSQL("notes_upgrade", seed); got != "" {
		t.Fatalf("seeding answered %q", got)
	}

	// The upgrade past the seeded rows is the case that matters: a NOT NULL
	// column with no default would be refused here, exactly as PostgreSQL
	// refuses it, and the run would stop with the file unapplied.
	out = stub.migrate("--database", "notes_upgrade")
	if want := fmt.Sprintf("migrate: %d applied to notes_upgrade", len(files)-1); !strings.Contains(out, want) {
		t.Fatalf("finishing the upgrade: want %q in:\n%s", want, out)
	}
	if got := len(stub.lines("notes_upgrade.versions")); got != len(files) {
		t.Errorf("the upgrade recorded %d of the %d migrations in schema_migrations", got, len(files))
	}
	if got := stub.mustPSQL("notes_upgrade", "SELECT count(*) FROM schema_migrations"); got != fmt.Sprint(len(files)) {
		t.Errorf("schema_migrations answers %q for the upgrade, want %d", got, len(files))
	}
	if got := stub.mustPSQL("notes_upgrade", "SELECT count(*) FROM notes"); got != "3" {
		t.Errorf("the upgrade kept %s of the 3 seeded rows", got)
	}
	if got := stub.mustPSQL("notes_upgrade", "SELECT count(*) FROM notes WHERE done IS NULL"); got != "0" {
		t.Errorf("%s seeded rows read done back as NULL", got)
	}
	if !containsFold(stub.lines("notes_upgrade.columns"), "done") {
		t.Errorf("the upgrade never added the done column; the table has %v", stub.lines("notes_upgrade.columns"))
	}
	// The column must have arrived after the rows existed, which is the
	// pre-existing-row case the default is for.
	seeded, added := statementPosition(t, stub, "notes_upgrade", "INSERT INTO NOTES"),
		statementPosition(t, stub, "notes_upgrade", "ALTER TABLE NOTES ADD COLUMN DONE")
	if seeded < 0 || added < 0 {
		t.Fatalf("the upgrade log is missing the seed (%d) or the done column (%d)", seeded, added)
	}
	if seeded > added {
		t.Errorf("the done column was added before the rows were seeded (statements %d and %d)", seeded, added)
	}

	// 2. Fresh install: every file applies, and every version is recorded.
	out = stub.migrate("--database", "notes")
	if want := fmt.Sprintf("migrate: %d applied to notes", len(files)); !strings.Contains(out, want) {
		t.Fatalf("fresh install: want %q in:\n%s", want, out)
	}
	if got := stub.mustPSQL("notes", "SELECT count(*) FROM schema_migrations"); got != fmt.Sprint(len(files)) {
		t.Errorf("the fresh install recorded %s of the %d migrations", got, len(files))
	}

	// 3. Re-run: nothing is applied again, so no version is recorded twice and
	// no column is added a second time.
	out = stub.migrate("--database", "notes")
	if want := "migrate: 0 applied to notes"; !strings.Contains(out, want) {
		t.Fatalf("the second run applied something: want %q in:\n%s", want, out)
	}
	if got := stub.mustPSQL("notes", "SELECT count(*) FROM schema_migrations"); got != fmt.Sprint(len(files)) {
		t.Errorf("the second run left %s schema_migrations rows, want %d", got, len(files))
	}
	if got := countContaining(stub.lines("notes.log"), "ALTER TABLE NOTES ADD COLUMN DONE"); got != 1 {
		t.Errorf("the done column was added %d times on a fresh install, want once", got)
	}
}

// TestStubRefusesWhatPostgresWouldRefuse keeps the harness above from passing
// vacuously: the same upgrade that the done migration survives must fail when
// the NOT NULL column carries no default.
func TestStubRefusesWhatPostgresWouldRefuse(t *testing.T) {
	stub := newMigrateStub(t)
	stub.migrate("--database", "notes_guard", "--to", "0001")
	stub.mustPSQL("notes_guard", "INSERT INTO notes (owner_sub, label) VALUES ('guard:a', 'one')")

	out, err := stub.psql("notes_guard", "ALTER TABLE notes ADD COLUMN risky BOOLEAN NOT NULL;")
	if err == nil {
		t.Fatalf("a NOT NULL column with no default was accepted over a seeded row:\n%s", out)
	}
	if !strings.Contains(out, "contains null values") {
		t.Fatalf("the seeded row did not refuse the column, and the stub said something else:\n%s", out)
	}
	if containsFold(stub.lines("notes_guard.columns"), "risky") {
		t.Error("the refused column was left on the table")
	}
	if _, err := stub.psql("notes_guard", "ALTER TABLE notes ADD COLUMN safe BOOLEAN NOT NULL DEFAULT false;"); err != nil {
		t.Fatalf("the same column with a false default was refused: %v", err)
	}
}

func containsFold(lines []string, want string) bool {
	for _, line := range lines {
		if strings.EqualFold(line, want) {
			return true
		}
	}
	return false
}

func countContaining(lines []string, want string) int {
	count := 0
	for _, line := range lines {
		if strings.Contains(strings.ToUpper(line), want) {
			count++
		}
	}
	return count
}

// statementPosition is the index of the first logged statement containing want,
// or -1 when the log holds none.
func statementPosition(t *testing.T, stub *migrateStub, database, want string) int {
	t.Helper()
	for i, line := range stub.lines(database + ".log") {
		if strings.Contains(strings.ToUpper(line), want) {
			return i
		}
	}
	return -1
}
