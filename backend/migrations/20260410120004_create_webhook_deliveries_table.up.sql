CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_endpoint_id UUID         NOT NULL,
    event_type          VARCHAR(50)  NOT NULL,
    payload             JSONB        NOT NULL,
    status              VARCHAR(20)  NOT NULL DEFAULT 'pending',
    response_code       INTEGER,
    response_body       TEXT,
    retry_count         INTEGER      NOT NULL DEFAULT 0,
    max_retries         INTEGER      NOT NULL DEFAULT 5,
    next_retry_at       TIMESTAMP WITH TIME ZONE,
    delivered_at        TIMESTAMP WITH TIME ZONE,
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_webhook_deliveries_endpoint FOREIGN KEY (webhook_endpoint_id) REFERENCES webhook_endpoints (id)
);

-- Index on webhook_endpoint_id (FK)
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_endpoint_id ON webhook_deliveries (webhook_endpoint_id);

-- Index on status for polling pending deliveries
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status ON webhook_deliveries (status);

-- Index on next_retry_at for retry scheduling queries
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_next_retry_at ON webhook_deliveries (next_retry_at);
