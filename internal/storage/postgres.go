package storage

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

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
	if err := applyMigrations(db); err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	}
	return &PostgresStorage{db: db}, nil
}

func applyMigrations(db *sql.DB) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create driver: %w", err)
	}
	src, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("create iofs source: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
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

// SaveBatch сохраняет множество записей в рамках одной транзакции.
func (s *PostgresStorage) SaveBatch(urls map[string]string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO short_urls (id, original_url)
		VALUES ($1, $2)
		ON CONFLICT (id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for id, originalURL := range urls {
		if _, err := stmt.Exec(id, originalURL); err != nil {
			return fmt.Errorf("exec: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
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
