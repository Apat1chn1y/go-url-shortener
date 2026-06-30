// Package handlers_test содержит интеграционные тесты для обработчиков с использованием моков.
package handlers_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/handlers"
	storagemocks "github.com/Apat1chn1y/go-url-shortener.git/internal/mocks/storage"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestCreateAndRedirectFileIntegration проверяет полный цикл с мок-хранилищем.
// В реальном коде это может быть файловое хранилище, но здесь мы используем мок.
func TestCreateAndRedirectFileIntegration(t *testing.T) {
	mockStore := storagemocks.NewStorage(t)

	// Ожидаем, что при создании будет вызов FindByOriginal (нет дубликата) и Save.
	mockStore.EXPECT().
		FindByOriginal("https://example.com").
		Return("", storage.ErrNotFound).
		Once()

	// Save должен быть вызван с любым ID и правильным URL, вернуть nil.
	mockStore.EXPECT().
		Save(mock.Anything, "https://example.com").
		Run(func(id, url string) {
			// Сохраняем сгенерированный ID для дальнейшего использования в редиректе.
			// В моке мы не можем сохранить реально, но можем вернуть его в Load.
		}).
		Return(nil).
		Once()

	// Ожидаем, что при редиректе будет вызван Load с тем же ID.
	// Так как мы не знаем ID заранее, используем mock.Anything и возвращаем оригинальный URL.
	mockStore.EXPECT().
		Load(mock.Anything).
		Return("https://example.com", nil).
		Once()

	shortener := service.NewShortener(mockStore)
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

	// Извлекаем ID (для проверки, что он не пустой)
	id := shortURL[len("http://localhost:8080/"):]
	require.NotEmpty(t, id)

	// Выполняем редирект (используем тот же хендлер, т.к. мок хранит состояние только в ожиданиях)
	reqRedirect := httptest.NewRequest(http.MethodGet, "/"+id, nil)
	rrRedirect := httptest.NewRecorder()
	handler.Redirect(rrRedirect, reqRedirect)

	assert.Equal(t, http.StatusTemporaryRedirect, rrRedirect.Code)
	assert.Equal(t, "https://example.com", rrRedirect.Header().Get("Location"))
}

// TestCreateAndRedirectIntegration проверяет полный цикл с in-memory моком.
func TestCreateAndRedirectIntegration(t *testing.T) {
	mockStore := storagemocks.NewStorage(t)

	mockStore.EXPECT().
		FindByOriginal("https://example.com").
		Return("", storage.ErrNotFound).
		Once()

	mockStore.EXPECT().
		Save(mock.Anything, "https://example.com").
		Return(nil).
		Once()

	mockStore.EXPECT().
		Load(mock.Anything).
		Return("https://example.com", nil).
		Once()

	shortener := service.NewShortener(mockStore)
	handler := handlers.NewShortenHandler(shortener, "http://localhost:8080/")

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("https://example.com"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	shortURL := rr.Body.String()
	assert.Contains(t, shortURL, "http://localhost:8080/")

	id := shortURL[len("http://localhost:8080/"):]
	require.NotEmpty(t, id)

	reqRedirect := httptest.NewRequest(http.MethodGet, "/"+id, nil)
	rrRedirect := httptest.NewRecorder()
	handler.Redirect(rrRedirect, reqRedirect)

	assert.Equal(t, http.StatusTemporaryRedirect, rrRedirect.Code)
	assert.Equal(t, "https://example.com", rrRedirect.Header().Get("Location"))
}
