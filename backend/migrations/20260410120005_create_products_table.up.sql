CREATE TABLE IF NOT EXISTS products (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         NOT NULL,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    is_active   BOOLEAN      NOT NULL DEFAULT true,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMP WITH TIME ZONE,

    CONSTRAINT fk_products_user FOREIGN KEY (user_id) REFERENCES users (id)
);

-- Index on user_id (FK)
CREATE INDEX IF NOT EXISTS idx_products_user_id ON products (user_id);

-- Index on deleted_at for soft delete queries
CREATE INDEX IF NOT EXISTS idx_products_deleted_at ON products (deleted_at);
