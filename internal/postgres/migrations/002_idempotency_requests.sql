CREATE TABLE IF NOT EXISTS idempotency_requests (
    idempotency_key TEXT PRIMARY KEY,
    request_hash BYTEA NOT NULL,
    outcome TEXT NOT NULL CHECK (outcome IN ('pending', 'created', 'conflict')),
    item JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ NOT NULL,
    CHECK (octet_length(request_hash) = 32),
    CHECK (
        (outcome = 'created' AND item IS NOT NULL)
        OR (outcome IN ('pending', 'conflict') AND item IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idempotency_requests_expires_at_idx
    ON idempotency_requests (expires_at);
