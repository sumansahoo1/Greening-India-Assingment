CREATE TABLE user_preferences (
    user_id   UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    page_size INT  NOT NULL DEFAULT 12,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_user_preferences_user_id ON user_preferences (user_id);
