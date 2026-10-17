package notes

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGStore is the PostgreSQL adapter for Store. The schema comes from the
// explicit migrations in ../../migrations; the API never migrates on start.
type PGStore struct{ Pool *pgxpool.Pool }

// Note is one row of the notes table, and the note object the API answers with.
// ID uses a decimal string because JavaScript numbers cannot safely hold all
// BIGINT values. Done is the completion flag the 0003 migration adds: the column
// is NOT NULL DEFAULT false, so every stored row carries a flag - including the
// rows written before the column existed - and a listed note can never be
// answered without it.
type Note struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"createdAt"`
	Done      bool      `json:"done"`
}

// listLimit bounds one page of notes; the fixture has no paging.
const listLimit = 100

// ErrNoteNotFound reports that no note of the owner handed to the store carries
// the requested id. A note that was never written and a note owned by a
// different subject are reported by this one error, so the HTTP layer answers
// both with the same body and the answer discloses neither (ASSUM-003).
var ErrNoteNotFound = errors.New("note not found")

// storageTitle is the last step before a note title is written: the standard
// library trim of leading and trailing whitespace, with the interior left
// exactly as it was. It is the one normalisation every write path applies -
// Create today, and an update path whenever one joins it - so the title that
// reaches storage is trimmed whatever the caller handed over.
func storageTitle(label string) string {
	return strings.TrimSpace(label)
}

// Create inserts one note for owner inside a transaction, with bound SQL.
func (s PGStore) Create(ctx context.Context, owner, label string) (note Note, err error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.Pool.Begin(queryCtx)
	if err != nil {
		return Note{}, fmt.Errorf("begin note transaction: %w", err)
	}
	defer func() {
		rollbackCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		if rollbackErr := tx.Rollback(rollbackCtx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback note transaction: %w", rollbackErr))
		}
	}()
	// The last step before the row is written: the title reaches storage without
	// its surrounding whitespace, so the stored row - and the label this call
	// reads back from RETURNING - can never carry it. Only the edges are cut.
	label = storageTitle(label)
	var id int64
	// The new row names no done column, so the column default writes it: every
	// note this call creates starts not-done, and RETURNING reads that flag back
	// rather than trusting the caller to have said so.
	if err := tx.QueryRow(queryCtx,
		"INSERT INTO notes (owner_sub, label) VALUES ($1, $2) RETURNING id, label, created_at, done", owner, label,
	).Scan(&id, &note.Label, &note.CreatedAt, &note.Done); err != nil {
		return Note{}, fmt.Errorf("insert note: %w", err)
	}
	if err := tx.Commit(queryCtx); err != nil {
		return Note{}, fmt.Errorf("commit note transaction: %w", err)
	}
	note.ID = strconv.FormatInt(id, 10)
	note.CreatedAt = note.CreatedAt.UTC()
	return note, nil
}

// List returns owner's newest notes first.
func (s PGStore) List(ctx context.Context, owner string) ([]Note, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	rows, err := s.Pool.Query(queryCtx,
		"SELECT id, label, created_at, done FROM notes WHERE owner_sub = $1 ORDER BY created_at DESC, id DESC LIMIT $2",
		owner, listLimit)
	if err != nil {
		return nil, fmt.Errorf("query notes: %w", err)
	}
	defer rows.Close()
	list := []Note{}
	for rows.Next() {
		var id int64
		var note Note
		if err := rows.Scan(&id, &note.Label, &note.CreatedAt, &note.Done); err != nil {
			return nil, fmt.Errorf("read note: %w", err)
		}
		note.ID = strconv.FormatInt(id, 10)
		note.CreatedAt = note.CreatedAt.UTC()
		list = append(list, note)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read notes: %w", err)
	}
	return list, nil
}

// UpdateDone sets the completion flag of owner's note id and answers with the
// row as it now stands. It is the only write the done flag has, and the one
// field it writes: the UPDATE names the done column alone, so the label and the
// creation time of the row are never touched and are read back exactly as they
// were stored (ASSUM-002).
//
// The statement is scoped to the row's owner as well as to its id, so a note
// belonging to another subject is unreachable - the UPDATE matches no row and
// the call answers ErrNoteNotFound, the same answer a missing id gets. Every
// value reaches PostgreSQL as a bound parameter; none of them is spelled into
// the SQL.
func (s PGStore) UpdateDone(ctx context.Context, owner, id string, done bool) (Note, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	noteID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		// An id that is not a number cannot name a row of the notes table, so
		// it is refused exactly as any other note this owner does not have.
		return Note{}, ErrNoteNotFound
	}
	var note Note
	if err := s.Pool.QueryRow(queryCtx,
		"UPDATE notes SET done = $1 WHERE id = $2 AND owner_sub = $3 RETURNING id, label, created_at, done",
		done, noteID, owner,
	).Scan(&noteID, &note.Label, &note.CreatedAt, &note.Done); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Note{}, ErrNoteNotFound
		}
		return Note{}, fmt.Errorf("update note: %w", err)
	}
	note.ID = strconv.FormatInt(noteID, 10)
	note.CreatedAt = note.CreatedAt.UTC()
	return note, nil
}
