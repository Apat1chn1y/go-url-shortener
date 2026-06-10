package storage

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

// PostgresStorage реализует Storage через PostgreSQL.
type PostgresStorage struct {
	db *sql.DB
}

// NewPostgresStorage создаёт новое PostgreSQL-хранилище и выполняет миграцию.
func NewPostgresStorage(dsn string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	// Создаём таблицу, если её нет
	const createTableSQL = `
	CREATE TABLE IF NOT EXISTS short_urls (
		id VARCHAR(255) PRIMARY KEY,
		original_url TEXT NOT NULL,
		uuid VARCHAR(36) NOT NULL DEFAULT gen_random_uuid()
	);
	`
	if _, err := db.Exec(createTableSQL); err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}
	return &PostgresStorage{db: db}, nil
}

// Save сохраняет пару id -> original_url.
// Если id уже существует, возвращает ErrAlreadyExists.
func (s *PostgresStorage) Save(id, originalURL string) error {
	_, err := s.db.Exec(
		"INSERT INTO short_urls (id, original_url) VALUES ($1, $2)",
		id, originalURL,
	)
	if err != nil {
		// Проверяем нарушение уникальности (код ошибки 23505 в PostgreSQL)
		if isDuplicateKeyError(err) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("insert: %w", err)
	}
	return nil
}

// Load возвращает оригинальный URL по id.
// Если id не найден, возвращает ErrNotFound.
func (s *PostgresStorage) Load(id string) (string, error) {
	var originalURL string
	err := s.db.QueryRow(
		"SELECT original_url FROM short_urls WHERE id = $1",
		id,
	).Scan(&originalURL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("select: %w", err)
	}
	return originalURL, nil
}

// Ping проверяет соединение с БД.
func (s *PostgresStorage) Ping() error {
	return s.db.Ping()
}

// Close закрывает соединение с БД.
func (s *PostgresStorage) Close() error {
	return s.db.Close()
}

// isDuplicateKeyError проверяет, является ли ошибка нарушением уникальности.
func isDuplicateKeyError(err error) bool {
	// код ошибки PostgreSQL для duplicate key: 23505
	if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
		return true
	}
	return false
}
