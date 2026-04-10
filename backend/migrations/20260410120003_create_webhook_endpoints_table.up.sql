CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID         NOT NULL,
    url        VARCHAR(500) NOT NULL,
    secret     VARCHAR(255) NOT NULL,
    events     JSONB        NOT NULL,
    is_active  BOOLEAN      NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,

    CONSTRAINT fk_webhook_endpoints_user FOREIGN KEY (user_id) REFERENCES users (id)
);

-- Index on user_id (FK)
CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_user_id ON webhook_endpoints (user_id);

-- Index on deleted_at for soft delete queries
CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_deleted_at ON webhook_endpoints (deleted_at);
