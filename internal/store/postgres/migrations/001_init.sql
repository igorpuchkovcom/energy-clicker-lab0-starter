CREATE TABLE IF NOT EXISTS game_sessions (
    id UUID PRIMARY KEY,
    points BIGINT NOT NULL DEFAULT 0 CHECK (points >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS collect_requests (
    session_id UUID NOT NULL REFERENCES game_sessions(id) ON DELETE CASCADE,
    idempotency_key TEXT NOT NULL,
    points_after BIGINT NOT NULL CHECK (points_after >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS collect_requests_created_at_idx
    ON collect_requests(created_at);
