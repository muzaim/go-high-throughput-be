CREATE TABLE IF NOT EXISTS items (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    total_stock INT NOT NULL DEFAULT 0,
    reserved_stock INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_total_stock_non_negative CHECK (total_stock >= 0),
    CONSTRAINT chk_reserved_stock_non_negative CHECK (reserved_stock >= 0),
    CONSTRAINT chk_reserved_not_exceed_total CHECK (reserved_stock <= total_stock)
);
