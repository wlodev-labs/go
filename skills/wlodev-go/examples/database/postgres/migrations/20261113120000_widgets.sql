-- +goose Up
CREATE TABLE widgets (
    id SERIAL PRIMARY KEY,
    public_id VARCHAR(12) NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    tags TEXT[],
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_widgets_user_id_created_at ON widgets (user_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS widgets;
