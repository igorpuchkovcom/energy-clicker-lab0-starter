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
	// TODO(Lab 0):
	// 1. Start a transaction.
	// 2. Lock the session row with SELECT ... FOR UPDATE.
	// 3. Look for (session_id, idempotency_key) in collect_requests.
	// 4. Replay points_after when it already exists.
	// 5. Otherwise increment points and insert the idempotency record.
	// 6. Commit both effects atomically.
	//
	// See docs/LAB_0_WORKBOOK.md, Milestone D.
	return 0, false, store.ErrNotImplemented
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) Close() {
	s.pool.Close()
}
