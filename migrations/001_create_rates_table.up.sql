CREATE TABLE IF NOT EXISTS rates (
    id         BIGSERIAL PRIMARY KEY,
    ask        DOUBLE PRECISION NOT NULL CHECK (ask > 0),
    bid        DOUBLE PRECISION NOT NULL CHECK (bid > 0),
    method     VARCHAR(10)      NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rates_created_at ON rates (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_rates_method_created_at ON rates (method, created_at DESC);
