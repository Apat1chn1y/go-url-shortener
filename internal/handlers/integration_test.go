// Package handlers_test содержит интеграционные тесты для обработчиков.
package handlers_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/handlers"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateAndRedirectIntegration проверяет полный цикл работы сервиса:
// создание короткой ссылки и последующий редирект по ней.
// Использует реальное in-memory хранилище, без моков.
func TestCreateAndRedirectIntegration(t *testing.T) {
	// Реальное хранилище (in-memory)
	store := storage.NewInMemoryStorage()
	shortener := service.NewShortener(store)
	handler := handlers.NewShortenHandler(shortener, "http://localhost:8080/")

	// Создание короткой ссылки для оригинального URL
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("https://example.com"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	// Проверка успешность создания
	require.Equal(t, http.StatusCreated, rr.Code)
	shortURL := rr.Body.String()
	assert.Contains(t, shortURL, "http://localhost:8080/")

	// Извлечение идентификатора из короткого URL
	id := strings.TrimPrefix(shortURL, "http://localhost:8080/")
	require.NotEmpty(t, id)

	// Выполнение запроса на редирект по полученному ID
	reqRedirect := httptest.NewRequest(http.MethodGet, "/"+id, nil)
	rrRedirect := httptest.NewRecorder()
	handler.Redirect(rrRedirect, reqRedirect)

	// Проверка редиректа
	assert.Equal(t, http.StatusTemporaryRedirect, rrRedirect.Code)
	assert.Equal(t, "https://example.com", rrRedirect.Header().Get("Location"))
}
