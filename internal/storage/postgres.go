// Package storage предоставляет реализацию хранилища для PostgreSQL с использованием pgx.
package storage

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // для миграций через database/sql
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// PostgresStorage реализует Storage через PostgreSQL с использованием pgxpool.
type PostgresStorage struct {
	pool *pgxpool.Pool
}

// SaveForUser сохраняет пару с привязкой к пользователю.
func (s *PostgresStorage) SaveForUser(id, originalURL, userID string) error {
	_, err := s.pool.Exec(context.TODO(),
		"INSERT INTO short_urls (id, original_url, user_id) VALUES ($1, $2, $3)",
		id, originalURL, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			switch pgErr.ConstraintName {
			case "short_urls_pkey":
				return ErrAlreadyExists
			case "idx_short_urls_original_url":
				return ErrOriginalURLDuplicate
			default:
				return fmt.Errorf("insert: %w", err)
			}
		}
		return fmt.Errorf("insert: %w", err)
	}
	return nil
}

// SaveBatchForUser сохраняет множество записей с привязкой к пользователю.
// Использует pgx.Batch для отправки всех запросов одним пакетом.
func (s *PostgresStorage) SaveBatchForUser(urls map[string]string, userID string) error {
	if len(urls) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for id, originalURL := range urls {
		batch.Queue(`
			INSERT INTO short_urls (id, original_url, user_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (id) DO NOTHING
		`, id, originalURL, userID)
	}

	ctx := context.TODO()
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()

	for range urls {
		_, err := br.Exec()
		if err != nil {
			return fmt.Errorf("batch exec: %w", err)
		}
	}
	return nil
}

// GetUserURLs возвращает все URL пользователя.
func (s *PostgresStorage) GetUserURLs(userID string) ([]UserURL, error) {
	rows, err := s.pool.Query(context.TODO(),
		"SELECT id, original_url FROM short_urls WHERE user_id = $1", userID)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	var result []UserURL
	for rows.Next() {
		var id, originalURL string
		if err := rows.Scan(&id, &originalURL); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		result = append(result, UserURL{
			ID:          id,
			ShortURL:    "", // заполняется в хендлере
			OriginalURL: originalURL,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return result, nil
}

// NewPostgresStorage создаёт новое PostgreSQL-хранилище, применяет миграции и возвращает пул.
func NewPostgresStorage(dsn string) (*PostgresStorage, error) {
	pool, err := pgxpool.New(context.TODO(), dsn)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	// Для миграций используем временное *sql.DB (драйвер pgx)
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sql db: %w", err)
	}
	defer sqlDB.Close()
	if err := applyMigrations(sqlDB); err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	}

	return &PostgresStorage{pool: pool}, nil
}

// applyMigrations применяет встроенные SQL-миграции через migrate.
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

// Save сохраняет пару id->original_url.
// При нарушении уникальности возвращает соответствующую ошибку.
func (s *PostgresStorage) Save(id, originalURL string) error {
	_, err := s.pool.Exec(context.TODO(),
		"INSERT INTO short_urls (id, original_url) VALUES ($1, $2)", id, originalURL)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			switch pgErr.ConstraintName {
			case "short_urls_pkey":
				return ErrAlreadyExists
			case "idx_short_urls_original_url":
				return ErrOriginalURLDuplicate
			default:
				return fmt.Errorf("insert: %w", err)
			}
		}
		return fmt.Errorf("insert: %w", err)
	}
	return nil
}

// SaveBatch сохраняет множество записей с использованием pgx.Batch.
// Отправляет все INSERT-запросы одним пакетом для повышения производительности.
func (s *PostgresStorage) SaveBatch(urls map[string]string) error {
	if len(urls) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for id, originalURL := range urls {
		batch.Queue(`
			INSERT INTO short_urls (id, original_url)
			VALUES ($1, $2)
			ON CONFLICT (id) DO NOTHING
		`, id, originalURL)
	}

	ctx := context.TODO()
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()

	for range urls {
		_, err := br.Exec()
		if err != nil {
			return fmt.Errorf("batch exec: %w", err)
		}
	}
	return nil
}

// Load возвращает оригинальный URL по id. Возвращает ErrNotFound, если запись отсутствует.
func (s *PostgresStorage) Load(id string) (string, error) {
	var originalURL string
	err := s.pool.QueryRow(context.TODO(),
		"SELECT original_url FROM short_urls WHERE id = $1", id).Scan(&originalURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("select: %w", err)
	}
	return originalURL, nil
}

// FindByOriginal возвращает id по оригинальному URL. Возвращает ErrNotFound, если URL не найден.
func (s *PostgresStorage) FindByOriginal(originalURL string) (string, error) {
	var id string
	err := s.pool.QueryRow(context.TODO(),
		"SELECT id FROM short_urls WHERE original_url = $1", originalURL).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("select: %w", err)
	}
	return id, nil
}

// Ping проверяет доступность базы данных.
func (s *PostgresStorage) Ping() error {
	return s.pool.Ping(context.TODO())
}

// Close закрывает пул соединений.
func (s *PostgresStorage) Close() error {
	s.pool.Close()
	return nil
}
