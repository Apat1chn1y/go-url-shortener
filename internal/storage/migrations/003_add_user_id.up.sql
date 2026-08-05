ALTER TABLE short_urls ADD COLUMN user_id TEXT;
CREATE INDEX idx_short_urls_user_id ON short_urls(user_id);