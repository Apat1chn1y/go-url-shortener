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
	_ = godotenv.Load()
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		t.Skip("DATABASE_DSN not set, skipping integration test")
	}

	store, err := NewPostgresStorage(dsn)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.TODO()
	const testUser1 = "user-1"
	const testUser2 = "user-2"

	// ---- Тест Save (без user) и Load ----
	err = store.Save("test123", "https://example.com")
	require.NoError(t, err)

	url, err := store.Load("test123")
	assert.NoError(t, err)
	assert.Equal(t, "https://example.com", url)

	// Повторное сохранение того же ID – ошибка ErrAlreadyExists
	err = store.Save("test123", "https://example.com")
	assert.ErrorIs(t, err, ErrAlreadyExists)

	// Сохранение нового ID с тем же оригинальным URL – ошибка уникальности
	err = store.Save("test456", "https://example.com")
	assert.ErrorIs(t, err, ErrOriginalURLDuplicate)

	// Проверка FindByOriginal
	foundID, err := store.FindByOriginal("https://example.com")
	assert.NoError(t, err)
	assert.Equal(t, "test123", foundID)

	// ---- Тест SaveForUser ----
	err = store.SaveForUser("test-user-1", "https://user1.ru", testUser1)
	require.NoError(t, err)

	// Проверяем, что можно загрузить
	loaded, err := store.Load("test-user-1")
	assert.NoError(t, err)
	assert.Equal(t, "https://user1.ru", loaded)

	// ---- Тест GetUserURLs ----
	// Сохраняем ещё несколько URL для разных пользователей
	err = store.SaveForUser("test-user-2", "https://user2.ru", testUser1)
	require.NoError(t, err)
	err = store.SaveForUser("test-user-3", "https://user3.ru", testUser2)
	require.NoError(t, err)

	user1URLs, err := store.GetUserURLs(testUser1)
	assert.NoError(t, err)
	assert.Len(t, user1URLs, 2)

	// Проверяем, что возвращены правильные URL (порядок не гарантирован, поэтому используем мапу)
	expected := map[string]string{
		"test-user-1": "https://user1.ru",
		"test-user-2": "https://user2.ru",
	}
	for _, u := range user1URLs {
		assert.Equal(t, expected[u.ID], u.OriginalURL)
	}

	user2URLs, err := store.GetUserURLs(testUser2)
	assert.NoError(t, err)
	assert.Len(t, user2URLs, 1)
	assert.Equal(t, "test-user-3", user2URLs[0].ID)
	assert.Equal(t, "https://user3.ru", user2URLs[0].OriginalURL)

	// ---- Тест SaveBatchForUser ----
	batch := map[string]string{
		"batch-1": "https://batch1.ru",
		"batch-2": "https://batch2.ru",
	}
	err = store.SaveBatchForUser(batch, testUser1)
	require.NoError(t, err)

	// Проверяем, что записи появились в GetUserURLs
	user1URLsAfterBatch, err := store.GetUserURLs(testUser1)
	assert.NoError(t, err)
	assert.Len(t, user1URLsAfterBatch, 4) // два старых + два из батча

	// Проверяем конкретные добавленные
	found := make(map[string]bool)
	for _, u := range user1URLsAfterBatch {
		if u.ID == "batch-1" && u.OriginalURL == "https://batch1.ru" {
			found["batch-1"] = true
		}
		if u.ID == "batch-2" && u.OriginalURL == "https://batch2.ru" {
			found["batch-2"] = true
		}
	}
	assert.True(t, found["batch-1"], "batch-1 not found")
	assert.True(t, found["batch-2"], "batch-2 not found")

	// ---- Тест Ping ----
	assert.NoError(t, store.Ping())

	// ---- Очистка ----
	_, _ = store.pool.Exec(ctx,
		"DELETE FROM short_urls WHERE id IN ($1, $2, $3, $4, $5, $6, $7, $8)",
		"test123", "test456", "test-user-1", "test-user-2", "test-user-3", "batch-1", "batch-2", "test-user-1")
}
