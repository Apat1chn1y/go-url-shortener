package storage

import (
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

	// Повторное сохранение – ошибка
	err = store.Save("test123", "https://example.com")
	assert.ErrorIs(t, err, ErrAlreadyExists)

	// Ping
	assert.NoError(t, store.Ping())

	// Удалим запись (не обязательно, но чистим)
	_, _ = store.db.Exec("DELETE FROM short_urls WHERE id = $1", "test123")
}
