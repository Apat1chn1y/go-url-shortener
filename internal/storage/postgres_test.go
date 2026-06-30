package storage

import (
	"context"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresStorage(t *testing.T) {
	_ = godotenv.Load() // загружаем .env
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		t.Skip("DATABASE_DSN not set, skipping integration test")
	}
	store, err := NewPostgresStorage(dsn)
	require.NoError(t, err)
	defer store.Close()

	// Тест Save/Load
	err = store.Save("test123", "https://example.com")
	require.NoError(t, err)

	url, err := store.Load("test123")
	assert.NoError(t, err)
	assert.Equal(t, "https://example.com", url)

	// Повторное сохранение того же ID – ошибка ErrAlreadyExists
	err = store.Save("test123", "https://example.com")
	assert.ErrorIs(t, err, ErrAlreadyExists)

	// Сохранение нового ID с тем же оригинальным URL – ошибка уникальности original_url
	err = store.Save("test456", "https://example.com")
	assert.ErrorIs(t, err, ErrOriginalURLDuplicate)

	// Проверка FindByOriginal
	foundID, err := store.FindByOriginal("https://example.com")
	assert.NoError(t, err)
	assert.Equal(t, "test123", foundID)

	// Ping
	assert.NoError(t, store.Ping())

	// Удалим записи (чистка)
	_, _ = store.pool.Exec(context.Background(), "DELETE FROM short_urls WHERE id = $1 OR id = $2", "test123", "test456")
}
