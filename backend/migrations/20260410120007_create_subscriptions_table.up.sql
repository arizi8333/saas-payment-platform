CREATE TABLE IF NOT EXISTS subscriptions (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id              UUID        NOT NULL,
    plan_id              UUID        NOT NULL,
    status               VARCHAR(30) NOT NULL DEFAULT 'pending_payment',
    current_period_start TIMESTAMP WITH TIME ZONE NOT NULL,
    current_period_end   TIMESTAMP WITH TIME ZONE NOT NULL,
    cancelled_at         TIMESTAMP WITH TIME ZONE,
    created_at           TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at           TIMESTAMP WITH TIME ZONE,

    CONSTRAINT fk_subscriptions_user FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT fk_subscriptions_plan FOREIGN KEY (plan_id) REFERENCES plans (id)
);

-- Index on user_id (FK)
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id ON subscriptions (user_id);

-- Index on plan_id (FK)
CREATE INDEX IF NOT EXISTS idx_subscriptions_plan_id ON subscriptions (plan_id);

-- Index on deleted_at for soft delete queries
CREATE INDEX IF NOT EXISTS idx_subscriptions_deleted_at ON subscriptions (deleted_at);
