package store

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrNotImplemented = errors.New("not implemented")
)

type State struct {
	SessionID string    `json:"session_id"`
	Points    int64     `json:"points"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Store interface {
	CreateSession(ctx context.Context) (State, error)
	GetState(ctx context.Context, sessionID string) (State, error)
	CollectUnsafe(ctx context.Context, sessionID string) (int64, error)
	Collect(ctx context.Context, sessionID, idempotencyKey string) (points int64, replayed bool, err error)
	Ping(ctx context.Context) error
	Close()
}
