ALTER TABLE short_urls ADD COLUMN is_deleted BOOLEAN DEFAULT FALSE;
CREATE INDEX idx_short_urls_is_deleted ON short_urls(is_deleted);