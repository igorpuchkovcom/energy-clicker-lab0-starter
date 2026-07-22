package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/igor/energy-clicker/internal/idgen"
	"github.com/igor/energy-clicker/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MinConns = 1

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}

	s := &Store{pool: pool}
	if err := s.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	if err := s.Migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return s, nil
}

func (s *Store) Migrate(ctx context.Context) error {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		body, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		if _, err := s.pool.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func (s *Store) CreateSession(ctx context.Context) (store.State, error) {
	id, err := idgen.UUID()
	if err != nil {
		return store.State{}, err
	}

	var state store.State
	err = s.pool.QueryRow(ctx, `
		INSERT INTO game_sessions (id)
		VALUES ($1)
		RETURNING id::text, points, created_at, updated_at
	`, id).Scan(&state.SessionID, &state.Points, &state.CreatedAt, &state.UpdatedAt)
	if err != nil {
		return store.State{}, fmt.Errorf("insert game session: %w", err)
	}
	return state, nil
}

func (s *Store) GetState(ctx context.Context, sessionID string) (store.State, error) {
	var state store.State
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, points, created_at, updated_at
		FROM game_sessions
		WHERE id = $1
	`, sessionID).Scan(&state.SessionID, &state.Points, &state.CreatedAt, &state.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.State{}, store.ErrNotFound
	}
	if err != nil {
		return store.State{}, fmt.Errorf("get game state: %w", err)
	}
	return state, nil
}

func (s *Store) CollectUnsafe(ctx context.Context, sessionID string) (int64, error) {
	var points int64
	err := s.pool.QueryRow(ctx, `
		UPDATE game_sessions
		SET points = points + 1,
		    updated_at = now()
		WHERE id = $1
		RETURNING points
	`, sessionID).Scan(&points)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, store.ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("unsafe collect: %w", err)
	}
	return points, nil
}

func (s *Store) Collect(ctx context.Context, sessionID, idempotencyKey string) (int64, bool, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, false, fmt.Errorf("begin collect ransaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var currentPoints int64

	err = tx.QueryRow(ctx, `
		SELECT points
		FROM game_sessions
		WHERE id = $1
		FOR UPDATE
	`, sessionID).Scan(&currentPoints)

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, store.ErrNotFound
	}
	if err != nil {
		return 0, false, fmt.Errorf("lock game session: %w", err)
	}

	var previousPoints int64

	err = tx.QueryRow(ctx, `
		SELECT points_after
		FROM collect_requests
		WHERE session_id = $1
		  AND idempotency_key = $2
	`, sessionID, idempotencyKey).Scan(&previousPoints)

	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return 0, false, fmt.Errorf(
				"commit replayed collect transaction: %w",
				err,
			)
		}

		return previousPoints, true, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, false, fmt.Errorf(
			"query idempotency record: %w",
			err,
		)
	}

	newPoints := currentPoints + 1

	_, err = tx.Exec(ctx, `
		UPDATE game_sessions
		SET points = $2,
		    updated_at = now()
		WHERE id = $1
	`, sessionID, newPoints)
	if err != nil {
		return 0, false, fmt.Errorf("update game points: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO collect_requests (
			session_id,
			idempotency_key,
			points_after
		)
		VALUES ($1, $2, $3)
	`, sessionID, idempotencyKey, newPoints)

	if err != nil {
		return 0, false, fmt.Errorf(
			"insert idempotency record: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, false, fmt.Errorf(
			"commit collect transaction: %w",
			err,
		)
	}

	return newPoints, false, nil
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) Close() {
	s.pool.Close()
}
