CREATE TABLE IF NOT EXISTS plans (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id       UUID         NOT NULL,
    name             VARCHAR(255) NOT NULL,
    amount           BIGINT       NOT NULL,
    currency         VARCHAR(3)   NOT NULL DEFAULT 'IDR',
    billing_interval VARCHAR(20)  NOT NULL,
    is_active        BOOLEAN      NOT NULL DEFAULT true,
    created_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMP WITH TIME ZONE,

    CONSTRAINT fk_plans_product FOREIGN KEY (product_id) REFERENCES products (id)
);

-- Index on product_id (FK)
CREATE INDEX IF NOT EXISTS idx_plans_product_id ON plans (product_id);

-- Index on deleted_at for soft delete queries
CREATE INDEX IF NOT EXISTS idx_plans_deleted_at ON plans (deleted_at);
