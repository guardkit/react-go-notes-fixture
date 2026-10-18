package notes

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The delivered read surface is GET /api/notes: whatever the notes table holds
// in its done column is what each note on the wire must carry, verbatim, next
// to the id, label and createdAt it already returned. These tests sit at the
// handler boundary - the surface the person uses - and cover both states of the
// Scenario Outline, the additive shape of the note, the read-only rule
// (ASSUM-003), and the column list the read path actually runs.

// doneWire is one note as the list response carries it, with the flag in the
// position the contract declares it: id, label, createdAt, then done.
type doneWire struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	CreatedAt string `json:"createdAt"`
	Done      bool   `json:"done"`
}

// listNotes asks the delivered read surface for the signed-in owner's notes and
// reports the recorded response.
func listNotes(store *fakeStore, target, body string) *httptest.ResponseRecorder {
	return request(Routes(store, fakeAuth{}, "run-1"), http.MethodGet, target, body, goodToken)
}

// decodeWireNotes reads the notes array of a list response back as raw JSON
// objects, so a test can tell a field that is absent from one that is present
// and null - neither of which a Go struct can report.
func decodeWireNotes(t *testing.T, body []byte) []map[string]any {
	t.Helper()
	var envelope struct {
		Notes []map[string]any `json:"notes"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode list response %q: %v", body, err)
	}
	return envelope.Notes
}

// The two @smoke states of "GET /api/notes returns each note's stored done
// flag": the value the row holds is the value the note answers, in both
// directions, with nothing between them rewritten.
func TestListAnswersEachNotesStoredDoneFlag(t *testing.T) {
	for _, tc := range []struct {
		name   string
		id     string
		label  string
		stored bool
	}{
		{"a row stored not-done is answered not-done", "1", "buy milk", false},
		{"a row stored done is answered done", "2", "ship spec", true},
	} {
		store := &fakeStore{list: []Note{{ID: tc.id, Label: tc.label, CreatedAt: created, Done: tc.stored}}}
		got := listNotes(store, "/api/notes", "")
		want := `{"notes":[{"id":"` + tc.id + `","label":"` + tc.label +
			`","createdAt":"2026-10-12T09:30:00Z","done":` + strconv.FormatBool(tc.stored) + `}]}` + "\n"
		if got.Code != http.StatusOK || got.Body.String() != want {
			t.Fatalf("%s: got %d %s, want 200 %s", tc.name, got.Code, got.Body.String(), want)
		}
		if store.calls != 1 || store.owner != "user-1" {
			t.Fatalf("%s: the read reached storage %d times for owner %q", tc.name, store.calls, store.owner)
		}
	}
}

// A note the caller just created is answered not-done: the flag is decided by
// the column default of 0003_add_done.sql, never by the request, and the
// response carries the value the row was written with.
func TestCreatedNoteIsAnsweredAndStoredNotDone(t *testing.T) {
	store := &fakeStore{note: Note{ID: "7", Label: "buy milk", CreatedAt: created}}
	got := request(Routes(store, fakeAuth{}, "run-1"), http.MethodPost, "/api/notes", `{"label":"buy milk"}`, goodToken)
	want := "{\"id\":\"7\",\"label\":\"buy milk\",\"createdAt\":\"2026-10-12T09:30:00Z\",\"done\":false}\n"
	if got.Code != http.StatusCreated || got.Body.String() != want {
		t.Fatalf("create: got %d %s, want 201 %s", got.Code, got.Body.String(), want)
	}
	if store.calls != 1 || store.owner != "user-1" || store.label != "buy milk" {
		t.Fatalf("create reached storage %d times as owner %q with label %q", store.calls, store.owner, store.label)
	}

	// The write path has nowhere to put a caller's flag: the only things the
	// store is handed are the request context, the owner from the token and the
	// validated label, so no boolean can reach the INSERT.
	storeType := reflect.TypeOf((*Store)(nil)).Elem()
	create, ok := storeType.MethodByName("Create")
	if !ok {
		t.Fatal("the store offers no Create, so nothing writes a note")
	}
	for arg := 1; arg < create.Type.NumIn(); arg++ {
		if kind := create.Type.In(arg).Kind(); kind == reflect.Bool {
			t.Fatalf("Create takes parameter %d as a %s, so the caller's flag could reach storage", arg, kind)
		}
	}
}

// Completion never hides a note: one not-done row and one done row are both
// answered, in the order the read returns them, each with its own flag.
func TestCompletedNoteIsNeverOmittedFromTheList(t *testing.T) {
	store := &fakeStore{list: []Note{
		{ID: "2", Label: "ship spec", CreatedAt: created, Done: true},
		{ID: "1", Label: "buy milk", CreatedAt: created, Done: false},
	}}
	got := listNotes(store, "/api/notes", "")
	if got.Code != http.StatusOK {
		t.Fatalf("list: got %d %s", got.Code, got.Body.String())
	}
	var envelope struct {
		Notes []doneWire `json:"notes"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode list response %q: %v", got.Body.String(), err)
	}
	want := []doneWire{
		{ID: "2", Label: "ship spec", CreatedAt: "2026-10-12T09:30:00Z", Done: true},
		{ID: "1", Label: "buy milk", CreatedAt: "2026-10-12T09:30:00Z", Done: false},
	}
	if !slices.Equal(envelope.Notes, want) {
		t.Fatalf("list answered %v, want both notes with their own flags: %v", envelope.Notes, want)
	}
}

// Every note in a mixed response carries the flag: the check asks the read
// surface what it returned and walks the members it finds, so it covers each
// note rather than a hand-picked one - and it refuses an empty answer, which
// would otherwise pass every per-note assertion by checking nothing.
func TestEveryReturnedNoteCarriesANonNullDoneField(t *testing.T) {
	store := &fakeStore{list: []Note{
		{ID: "3", Label: "closed task", CreatedAt: created, Done: true},
		{ID: "2", Label: "open task", CreatedAt: created, Done: false},
		{ID: "1", Label: "buy milk", CreatedAt: created, Done: false},
	}}
	got := listNotes(store, "/api/notes", "")
	if got.Code != http.StatusOK {
		t.Fatalf("list: got %d %s", got.Code, got.Body.String())
	}
	notes := decodeWireNotes(t, got.Body.Bytes())
	if len(notes) == 0 {
		t.Fatalf("the read surface answered %q, so there was no note to check the flag on", got.Body.String())
	}
	if len(notes) != len(store.list) {
		t.Fatalf("the read surface answered %d notes for the %d it was given: %s", len(notes), len(store.list), got.Body.String())
	}
	for _, note := range notes {
		label, _ := note["label"].(string)
		raw, present := note["done"]
		if !present {
			t.Fatalf("note %q was answered without a done field at all: %s", label, got.Body.String())
		}
		flag, isBool := raw.(bool)
		if !isBool {
			t.Fatalf("note %q was answered with done %v, want a boolean that is never null: %s", label, raw, got.Body.String())
		}
		for _, member := range store.list {
			if member.Label == label && member.Done != flag {
				t.Fatalf("note %q was answered done %v, but the row holds %v", label, flag, member.Done)
			}
		}
	}
}

// The flag is additive: the note still answers its id, label and createdAt in
// the order the contract lists them, and gains done beside them rather than in
// place of any of them.
func TestDoneFlagDoesNotDisplaceTheExistingNoteFields(t *testing.T) {
	store := &fakeStore{list: []Note{{ID: "9007199254740993", Label: "buy milk", CreatedAt: created, Done: true}}}
	got := listNotes(store, "/api/notes", "")
	want := "{\"notes\":[{\"id\":\"9007199254740993\",\"label\":\"buy milk\",\"createdAt\":\"2026-10-12T09:30:00Z\",\"done\":true}]}\n"
	if got.Code != http.StatusOK || got.Body.String() != want {
		t.Fatalf("got %d %s, want 200 %s", got.Code, got.Body.String(), want)
	}
	notes := decodeWireNotes(t, got.Body.Bytes())
	if len(notes) != 1 {
		t.Fatalf("the read surface answered %d notes, want the one it was given: %s", len(notes), got.Body.String())
	}
	fields := []string{}
	for field := range notes[0] {
		fields = append(fields, field)
	}
	slices.Sort(fields)
	if !slices.Equal(fields, []string{"createdAt", "done", "id", "label"}) {
		t.Fatalf("note answered fields %v, want id, label, createdAt and done", fields)
	}

	// The Go type behind the wire shape carries the flag as a boolean named
	// after the migration's column, in the contract's order.
	noteType := reflect.TypeOf(Note{})
	field, ok := noteType.FieldByName("Done")
	if !ok {
		t.Fatalf("the Note type answers no Done field, only %v", noteFields(noteType))
	}
	if field.Type.Kind() != reflect.Bool {
		t.Fatalf("Note.Done is %s, want the boolean the column holds", field.Type)
	}
	if tag := strings.Split(field.Tag.Get("json"), ",")[0]; tag != "done" {
		t.Fatalf("Note.Done is read from %q, want the wire key done", tag)
	}
	if !slices.Equal(noteFields(noteType), []string{"id", "label", "createdAt", "done"}) {
		t.Fatalf("the Note type serialises %v, want the contract order ending with done", noteFields(noteType))
	}
}

// noteFields lists the JSON keys of the Note type in the order they are written.
func noteFields(noteType reflect.Type) []string {
	fields := []string{}
	for field := range noteType.NumField() {
		fields = append(fields, strings.Split(noteType.Field(field).Tag.Get("json"), ",")[0])
	}
	return fields
}

// poisonBody is a request body that reports - and refuses - every attempt to
// read it, so a read path that consulted the caller's body is caught rather
// than trusted.
type poisonBody struct{ touched bool }

func (b *poisonBody) Read([]byte) (int, error) {
	b.touched = true
	return 0, errors.New("the notes read path read the request body")
}

func (*poisonBody) Close() error { return nil }

// The read endpoint is read-only (ASSUM-003): a done value offered by the
// caller - in the query, in the body or in a header - changes nothing. The
// response stays the one the stored rows describe, the body is never opened,
// and the store is asked for the owner's notes and nothing else.
func TestReadPathNeverAcceptsADoneValueFromTheCaller(t *testing.T) {
	stored := []Note{
		{ID: "2", Label: "ship spec", CreatedAt: created, Done: true},
		{ID: "1", Label: "buy milk", CreatedAt: created, Done: false},
	}
	plain := &fakeStore{list: stored}
	baseline := listNotes(plain, "/api/notes", "")
	if baseline.Code != http.StatusOK {
		t.Fatalf("plain list: got %d %s", baseline.Code, baseline.Body.String())
	}

	for _, tc := range []struct {
		name, target string
		header       string
	}{
		{"done in the query", "/api/notes?done=true", ""},
		{"every filter at once", "/api/notes?done=true&label=buy+milk&id=1&notes=ship+spec", ""},
		{"done in a header", "/api/notes", "true"},
	} {
		store := &fakeStore{list: stored}
		got := listNotes(store, tc.target, "")
		if got.Code != baseline.Code || got.Body.String() != baseline.Body.String() {
			t.Fatalf("%s: got %d %s, want the stored answer %s", tc.name, got.Code, got.Body.String(), baseline.Body.String())
		}
		if store.calls != 1 || store.owner != "user-1" || store.label != "" {
			t.Fatalf("%s: the read reached storage %d times as owner %q with label %q", tc.name, store.calls, store.owner, store.label)
		}
	}

	// A body is never opened by the read: this one reports the attempt and
	// refuses, and the response is still the stored answer.
	store := &fakeStore{list: stored}
	body := &poisonBody{}
	r := httptest.NewRequest(http.MethodGet, "/api/notes?done=true", body)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+goodToken)
	r.Header.Set("X-Done", "true")
	w := httptest.NewRecorder()
	Routes(store, fakeAuth{}, "run-1").ServeHTTP(w, r)
	if w.Code != http.StatusOK || w.Body.String() != baseline.Body.String() {
		t.Fatalf("a body offered to the read path changed the answer: %d %s", w.Code, w.Body.String())
	}
	if body.touched {
		t.Fatal("the notes read path opened the request body, so a caller could hand over a done value")
	}
	if store.calls != 1 || store.owner != "user-1" {
		t.Fatalf("the read reached storage %d times for owner %q", store.calls, store.owner)
	}

	// The list call the read path makes carries the owner and nothing else, so
	// there is no parameter a caller-supplied flag could travel in.
	storeType := reflect.TypeOf((*Store)(nil)).Elem()
	list, ok := storeType.MethodByName("List")
	if !ok {
		t.Fatal("the store offers no List, so nothing reads the notes back")
	}
	for arg := 1; arg < list.Type.NumIn(); arg++ {
		if kind := list.Type.In(arg).Kind(); kind != reflect.String && kind != reflect.Interface {
			t.Fatalf("List takes parameter %d as a %s, so the read is no longer scoped to the owner alone", arg, kind)
		}
	}
}

// The delivered wire response - the bytes the handler actually wrote, not a
// hand-written body - validates against the contract TASK-A91A-002 updated, and
// the same validator refuses that response with the flag taken away.
func TestDeliveredListResponseValidatesAgainstTheContract(t *testing.T) {
	router := contractRouter(t)
	store := &fakeStore{list: []Note{
		{ID: "2", Label: "ship spec", CreatedAt: created, Done: true},
		{ID: "1", Label: "buy milk", CreatedAt: created, Done: false},
	}}
	got := listNotes(store, "/api/notes", "")
	r := httptest.NewRequest(http.MethodGet, "/api/notes", nil)
	if err := checkResponse(router, r, got.Code, got.Header(), got.Body.Bytes()); err != nil {
		t.Fatalf("the contract refused the delivered response %s: %v", got.Body.String(), err)
	}

	// The same response with the flag removed is refused, so the acceptance
	// above proves the flag is what the contract asked for.
	without := strings.Replace(got.Body.String(), `,"done":true`, "", 1)
	if without == got.Body.String() {
		t.Fatalf("the delivered response %s carries no done to remove", got.Body.String())
	}
	if err := checkResponse(router, r, http.StatusOK, got.Header(), []byte(without)); err == nil {
		t.Fatal("the contract accepted the delivered response with its done field removed")
	}

	// The create response validates too, flag and all.
	createdStore := &fakeStore{note: Note{ID: "7", Label: "buy milk", CreatedAt: created}}
	posted := request(Routes(createdStore, fakeAuth{}, "run-1"), http.MethodPost, "/api/notes", `{"label":"buy milk"}`, goodToken)
	post := httptest.NewRequest(http.MethodPost, "/api/notes", strings.NewReader(`{"label":"buy milk"}`))
	if err := checkResponse(router, post, posted.Code, posted.Header(), posted.Body.Bytes()); err != nil {
		t.Fatalf("the contract refused the delivered create response %s: %v", posted.Body.String(), err)
	}
}

// The members of the notes set are every file that reads or writes the rows of
// the notes table. The check finds them itself when it runs, seeded by the read
// path in store.go, and walks the whole repository rather than a list written
// down by hand: any member that reads note columns must read the done column
// too, any member that adds a note must leave the flag to the column default of
// 0003_add_done.sql rather than take one from a caller, and any member that
// updates a note must set the flag and nothing else, scope the row by id and
// owner together, and read the whole note back. Finding no member at all is a
// failure, never a silent pass.
func TestEveryNoteReadFromTheNotesTableCarriesTheDoneColumn(t *testing.T) {
	root := filepath.Clean(notesSetRoot)
	members := []string{}
	problems := []string{}
	reads, writes, storeReads, storeWrites := 0, 0, 0, 0
	updates, storeUpdates := 0, 0

	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if notesSetSkipDirs[entry.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !notesSetExtensions[filepath.Ext(path)] {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := notesSetLines(string(raw), path)
		found := false
		for number, line := range lines {
			if !notesTableReference.MatchString(line) {
				continue
			}
			found = true
			// A column list may sit on the lines above the one naming the table,
			// so the statement is read from a short window ending at this line.
			window := line
			for back := 1; back <= 3 && number-back >= 0; back++ {
				window = lines[number-back] + " " + window
			}
			at := filepath.Clean(path) + ":" + strconv.Itoa(number+1)
			if notesWrite.MatchString(line) {
				writes++
				if filepath.Base(path) == "store.go" {
					storeWrites++
				}
				before, returning, hasReturning := notesSplitReturning(line)
				// The statement is the quoted SQL, so what follows its closing
				// quote is the surrounding code, not columns.
				returning = strings.Split(returning, "\"")[0]
				if notesUpdateRow.MatchString(line) {
					// An update exists to set the flag, so its rule is the other
					// one: the statement sets the done column and nothing else,
					// picks the row by id and owner together so another owner's
					// note is never its row, and hands the whole note back.
					updates++
					if filepath.Base(path) == "store.go" {
						storeUpdates++
					}
					set, where := notesAssignment(before)
					for _, column := range []string{"label", "created_at", "owner_sub", "id"} {
						if notesColumns(set, column) {
							problems = append(problems, at+" writes the "+column+" column of a note row, so the update mutates more than the done flag")
						}
					}
					if !notesColumns(set, "done") {
						problems = append(problems, at+" updates a note row without setting the done column, so the flag is never written")
					}
					for _, column := range []string{"id", "owner_sub"} {
						if !notesColumns(where, column) {
							problems = append(problems, at+" updates a note row without scoping it by "+column+", so a note the caller does not own could be changed")
						}
					}
					if !hasReturning {
						problems = append(problems, at+" updates a note row without RETURNING, so the caller never sees the row it changed")
					}
					for _, column := range []string{"id", "label", "created_at", "done"} {
						if !notesColumns(returning, column) {
							problems = append(problems, at+" hands back "+strings.TrimSpace(returning)+" without the "+column+" column, so the updated note is answered incomplete")
						}
					}
					continue
				}
				if notesColumns(before, "done") {
					problems = append(problems, at+" writes a note while naming the done column before RETURNING, so the flag is taken from the caller instead of the column default")
				}
				if hasReturning && !notesColumns(returning, "done") {
					problems = append(problems, at+" hands back "+strings.TrimSpace(returning)+" without the done column, so the written flag is never read back")
				}
				continue
			}
			match := notesSelectList.FindStringSubmatch(window)
			if match == nil {
				continue
			}
			columns := notesTokenise(match[1])
			if len(columns) == 0 || notesCountsRows(columns) {
				// A read of a count or an existence test answers a number, not a
				// note, so it carries no note columns to check.
				continue
			}
			reads++
			if filepath.Base(path) == "store.go" {
				storeReads++
			}
			if !slices.Contains(columns, "done") && !slices.Contains(columns, "*") {
				problems = append(problems, at+" reads "+strings.Join(columns, ", ")+" as a note without the done column")
			}
		}
		if found {
			members = append(members, filepath.Clean(path))
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk the repository for the notes set: %v", walkErr)
	}

	if len(members) == 0 {
		t.Fatalf("no member of the notes set was found under %s, so this check covered nothing", root)
	}
	if reads == 0 {
		t.Fatalf("the notes set %v holds no read of note rows to check", members)
	}
	if writes == 0 {
		t.Fatalf("the notes set %v holds no write of a note row to check", members)
	}
	if storeReads == 0 || storeWrites == 0 {
		t.Fatalf("the read path in store.go no longer reads (reads %d, writes %d) the owner's rows, so this check had nothing to cover", storeReads, storeWrites)
	}
	if updates == 0 {
		t.Fatalf("the notes set %v holds no update of a note row to check", members)
	}
	if storeUpdates == 0 {
		t.Fatalf("the update path in store.go no longer updates (updates %d) the owner's rows, so this check had nothing to cover", storeUpdates)
	}
	for _, problem := range problems {
		t.Error(problem)
	}
}

// notesSetRoot is the repository root, reached the way contract_test.go reaches
// contract/openapi.yaml.
var notesSetRoot = filepath.Join("..", "..", "..", "..")

// notesSetExtensions are the languages a member of the notes set is written in.
var notesSetExtensions = map[string]bool{".go": true, ".sql": true, ".sh": true, ".mjs": true, ".ts": true, ".tsx": true}

// notesSetSkipDirs never hold a member: vendored packages, build output and
// tool state are copies or artifacts, not the delivered read path.
var notesSetSkipDirs = map[string]bool{".git": true, "node_modules": true, "dist": true, "build": true, "coverage": true, ".guardkit": true}

// notesTableReference names a statement that reads or writes note rows. A
// CREATE or ALTER statement shapes the table instead of its rows, so it is not
// a member of the set.
var notesTableReference = regexp.MustCompile(`(?i)\b(?:from|into|update)\s+notes\b`)

// notesWrite separates the statements that add rows from the ones that read
// them, so each is held to its own rule about the flag.
var notesWrite = regexp.MustCompile(`(?i)\b(?:insert\s+into|update)\s+notes\b`)

// notesUpdateRow separates the statement that changes a row from the one that
// adds it: adding a note leaves the flag to the column default, while changing
// one is exactly how the flag is written.
var notesUpdateRow = regexp.MustCompile(`(?i)\bupdate\s+notes\b`)

// notesAssignment splits an update into the columns it writes and the clause
// that decides which row it writes them to, so the flag it mutates and the
// scoping of the row are checked separately.
func notesAssignment(statement string) (set, where string) {
	lowered := strings.ToLower(statement)
	start := strings.Index(lowered, " set ")
	if start < 0 {
		return "", lowered
	}
	end := strings.Index(lowered, " where ")
	if end < start {
		return lowered[start+len(" set "):], ""
	}
	return lowered[start+len(" set ") : end], lowered[end:]
}

// notesSelectList is the column list of the note read that starts just in front
// of the reference to the table. The greedy prefix anchors it at the last
// select before the reference, so an existence test inside the same statement
// is never mistaken for the read of the notes themselves.
var notesSelectList = regexp.MustCompile(`(?is).*\bselect\s+(.*?)\s+from\s+notes\b`)

// notesReturningKeyword splits a write into the part that decides the row and
// the part that reports it back.
var notesReturningKeyword = regexp.MustCompile(`(?i)\breturning\b`)

// notesCommentPrefix is the line-comment marker per source language, so a
// comment naming the table is not counted as a statement that touches it.
var notesCommentPrefix = map[string]string{".go": "//", ".sql": "--", ".sh": "#", ".mjs": "//", ".ts": "//", ".tsx": "//"}

// notesSetLines returns the code lines of one member, comments removed.
func notesSetLines(text, path string) []string {
	marker := notesCommentPrefix[filepath.Ext(path)]
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if marker != "" && strings.HasPrefix(strings.TrimSpace(line), marker) {
			kept = append(kept, "")
			continue
		}
		kept = append(kept, line)
	}
	return kept
}

// notesSplitReturning reports the part of a write before RETURNING and the part
// it hands back.
func notesSplitReturning(line string) (before, returning string, hasReturning bool) {
	parts := notesReturningKeyword.Split(line, 2)
	if len(parts) != 2 {
		return line, "", false
	}
	return parts[0], parts[1], true
}

// notesTokenise reduces a column list to the lower-case identifiers in it, so
// quoting, punctuation and surrounding code cannot be mistaken for a column.
func notesTokenise(list string) []string {
	lowered := strings.ToLower(list)
	return append([]string{}, strings.FieldsFunc(lowered, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_')
	})...)
}

// notesColumns reports whether a column list names one column.
func notesColumns(list, column string) bool {
	return slices.Contains(notesTokenise(list), column)
}

// notesCountsRows reports a read that answers a number rather than a note: a
// count or an existence test names no note columns to check.
func notesCountsRows(columns []string) bool {
	return slices.Equal(columns, []string{"1"}) ||
		slices.Contains(columns, "count") ||
		slices.Contains(columns, "exists")
}
