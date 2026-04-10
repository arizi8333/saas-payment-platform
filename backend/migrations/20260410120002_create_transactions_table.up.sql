CREATE TABLE IF NOT EXISTS transactions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID          NOT NULL,
    external_id     VARCHAR(255)  NOT NULL,
    amount          BIGINT        NOT NULL,
    currency        VARCHAR(3)    NOT NULL DEFAULT 'IDR',
    status          VARCHAR(20)   NOT NULL DEFAULT 'pending',
    payment_method  VARCHAR(30)   NOT NULL,
    description     TEXT,
    customer_email  VARCHAR(255),
    idempotency_key VARCHAR(255),
    metadata        JSONB,
    paid_at         TIMESTAMP WITH TIME ZONE,
    expired_at      TIMESTAMP WITH TIME ZONE,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMP WITH TIME ZONE,

    CONSTRAINT fk_transactions_user FOREIGN KEY (user_id) REFERENCES users (id)
);

-- Unique constraint on external_id
CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_external_id ON transactions (external_id);

-- Unique constraint on idempotency_key (partial - only non-null values)
CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_idempotency_key ON transactions (idempotency_key) WHERE idempotency_key IS NOT NULL;

-- Index on user_id (FK)
CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON transactions (user_id);

-- Index on status for frequent queries
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions (status);

-- Index on deleted_at for soft delete queries
CREATE INDEX IF NOT EXISTS idx_transactions_deleted_at ON transactions (deleted_at);
