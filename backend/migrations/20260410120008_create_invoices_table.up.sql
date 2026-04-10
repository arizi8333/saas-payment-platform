CREATE TABLE IF NOT EXISTS invoices (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL,
    transaction_id  UUID,
    subscription_id UUID,
    invoice_number  VARCHAR(50) NOT NULL,
    amount          BIGINT      NOT NULL,
    currency        VARCHAR(3)  NOT NULL DEFAULT 'IDR',
    status          VARCHAR(20) NOT NULL DEFAULT 'unpaid',
    due_date        TIMESTAMP WITH TIME ZONE NOT NULL,
    paid_at         TIMESTAMP WITH TIME ZONE,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMP WITH TIME ZONE,

    CONSTRAINT fk_invoices_user         FOREIGN KEY (user_id)         REFERENCES users (id),
    CONSTRAINT fk_invoices_transaction  FOREIGN KEY (transaction_id)  REFERENCES transactions (id),
    CONSTRAINT fk_invoices_subscription FOREIGN KEY (subscription_id) REFERENCES subscriptions (id)
);

-- Unique constraint on invoice_number
CREATE UNIQUE INDEX IF NOT EXISTS idx_invoices_invoice_number ON invoices (invoice_number);

-- Index on user_id (FK)
CREATE INDEX IF NOT EXISTS idx_invoices_user_id ON invoices (user_id);

-- Index on transaction_id (FK)
CREATE INDEX IF NOT EXISTS idx_invoices_transaction_id ON invoices (transaction_id);

-- Index on subscription_id (FK)
CREATE INDEX IF NOT EXISTS idx_invoices_subscription_id ON invoices (subscription_id);

-- Index on deleted_at for soft delete queries
CREATE INDEX IF NOT EXISTS idx_invoices_deleted_at ON invoices (deleted_at);
