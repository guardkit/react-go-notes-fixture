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

// listLimit bounds one page of notes; the fixture has no paging.
const listLimit = 100

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
	if err := tx.QueryRow(queryCtx,
		"INSERT INTO notes (owner_sub, label) VALUES ($1, $2) RETURNING id, label, created_at", owner, label,
	).Scan(&id, &note.Label, &note.CreatedAt); err != nil {
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
		"SELECT id, label, created_at FROM notes WHERE owner_sub = $1 ORDER BY created_at DESC, id DESC LIMIT $2",
		owner, listLimit)
	if err != nil {
		return nil, fmt.Errorf("query notes: %w", err)
	}
	defer rows.Close()
	list := []Note{}
	for rows.Next() {
		var id int64
		var note Note
		if err := rows.Scan(&id, &note.Label, &note.CreatedAt); err != nil {
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
