-- +goose Up
CREATE TABLE trip_idempotency (
    key UUID PRIMARY KEY,
    request JSONB NOT NULL,
    trip_id UUID REFERENCES trips(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX trip_idempotency_expires_at_idx ON trip_idempotency (expires_at);

-- +goose Down
DROP TABLE trip_idempotency;
