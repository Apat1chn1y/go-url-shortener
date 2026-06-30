package handlers

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	storagemocks "github.com/Apat1chn1y/go-url-shortener.git/internal/mocks/storage"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestGzipMiddleware проверяет работу gzip middleware.
func TestGzipMiddleware(t *testing.T) {
	// Вспомогательная функция для настройки мока с общими ожиданиями
	setupMock := func(m *storagemocks.Storage) {
		// Ожидаем, что при создании ссылки будет вызов FindByOriginal и Save.
		// Используем Maybe(), так как в некоторых тестах эти вызовы могут не происходить.
		m.EXPECT().FindByOriginal(mock.Anything).Return("", storage.ErrNotFound).Maybe()
		m.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Maybe()
	}

	// ---- Тест: сжатый ответ для JSON эндпоинта ----
	t.Run("response compressed for /api/shorten with Accept-Encoding: gzip", func(t *testing.T) {
		mockStore := storagemocks.NewStorage(t)
		setupMock(mockStore)

		logger := zerolog.Nop()
		shortener := service.NewShortener(mockStore)
		handler := NewShortenHandler(shortener, "http://localhost:8080/")
		router := NewRouter(handler, logger)

		body := `{"url":"https://ya.ru"}`
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Encoding", "gzip")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))

		// Разжимаем тело
		reader, err := gzip.NewReader(rr.Body)
		require.NoError(t, err)
		defer reader.Close()
		uncompressed, err := io.ReadAll(reader)
		require.NoError(t, err)

		var resp shortenResponse
		err = json.Unmarshal(uncompressed, &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Result, "http://localhost:8080/")
	})

	// ---- Тест: сжатый ответ для plain text (не должен сжиматься, т.к. тип не поддерживается) ----
	t.Run("plain text response not compressed", func(t *testing.T) {
		mockStore := storagemocks.NewStorage(t)
		setupMock(mockStore)

		logger := zerolog.Nop()
		shortener := service.NewShortener(mockStore)
		handler := NewShortenHandler(shortener, "http://localhost:8080/")
		router := NewRouter(handler, logger)

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("https://ya.ru"))
		req.Header.Set("Content-Type", "text/plain")
		req.Header.Set("Accept-Encoding", "gzip")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.NotEqual(t, "gzip", rr.Header().Get("Content-Encoding"))
	})

	// ---- Тест: сжатый запрос (Content-Encoding: gzip) для JSON эндпоинта ----
	t.Run("request compressed with Content-Encoding: gzip", func(t *testing.T) {
		mockStore := storagemocks.NewStorage(t)
		setupMock(mockStore)

		logger := zerolog.Nop()
		shortener := service.NewShortener(mockStore)
		handler := NewShortenHandler(shortener, "http://localhost:8080/")
		router := NewRouter(handler, logger)

		var buf bytes.Buffer
		gzWriter := gzip.NewWriter(&buf)
		_, err := gzWriter.Write([]byte(`{"url":"https://ya.ru"}`))
		require.NoError(t, err)
		err = gzWriter.Close()
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/api/shorten", &buf)
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Encoding", "gzip")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	// ---- Тест: некорректный gzip в запросе ----
	t.Run("invalid gzip request returns 400", func(t *testing.T) {
		mockStore := storagemocks.NewStorage(t)

		logger := zerolog.Nop()
		shortener := service.NewShortener(mockStore)
		handler := NewShortenHandler(shortener, "http://localhost:8080/")
		router := NewRouter(handler, logger)

		req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewBufferString("not gzip"))
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid gzip body")
	})
}
