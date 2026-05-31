-- +goose Up

-- sessions: authenticated dashboard sessions created on login via PAM.
-- id is a UUID v4 stored as TEXT. expires_at is a Unix timestamp;
-- the middleware rejects sessions past this time. CleanupLoop purges
-- expired rows every hour so the table stays small.
CREATE TABLE sessions (
    id         TEXT    PRIMARY KEY,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,  -- Unix timestamp; session invalid after this
    username   TEXT    NOT NULL   -- Linux system username from PAM
);

CREATE INDEX idx_sessions_expires ON sessions(expires_at);

-- +goose Down

DROP TABLE IF EXISTS sessions;
