CREATE TABLE IF NOT EXISTS api_keys (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID         NOT NULL,
    name         VARCHAR(100) NOT NULL,
    key_hash     VARCHAR(255) NOT NULL,
    key_prefix   VARCHAR(12)  NOT NULL,
    is_active    BOOLEAN      NOT NULL DEFAULT true,
    last_used_at TIMESTAMP WITH TIME ZONE,
    expires_at   TIMESTAMP WITH TIME ZONE,
    rate_limit   INTEGER      NOT NULL DEFAULT 1000,
    created_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMP WITH TIME ZONE,

    CONSTRAINT fk_api_keys_user FOREIGN KEY (user_id) REFERENCES users (id)
);

-- Unique constraint on key_hash
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys (key_hash);

-- Index on user_id (FK)
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys (user_id);

-- Index on deleted_at for soft delete queries
CREATE INDEX IF NOT EXISTS idx_api_keys_deleted_at ON api_keys (deleted_at);
