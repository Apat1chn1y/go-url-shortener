CREATE TABLE IF NOT EXISTS short_urls (
    id VARCHAR(255) PRIMARY KEY,
    original_url VARCHAR(2048) NOT NULL,
    uuid UUID NOT NULL DEFAULT gen_random_uuid()
);