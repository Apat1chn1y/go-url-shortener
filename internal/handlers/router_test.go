// Package handlers_test содержит интеграционные тесты маршрутизации и обработчиков.
package handlers_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/handlers"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockShortener – простая реализация handlers.URLShortener для интеграционных тестов.
// Позволяет предсказуемо создавать короткие ссылки и возвращать оригиналы.
type mockShortener struct {
	createFunc func(originalURL, baseURL string) (string, error)
	getFunc    func(id string) (string, error)
}

func (m *mockShortener) Create(originalURL, baseURL string) (string, error) {
	if m.createFunc != nil {
		return m.createFunc(originalURL, baseURL)
	}
	return baseURL + "abc123", nil
}

func (m *mockShortener) Get(id string) (string, error) {
	if m.getFunc != nil {
		return m.getFunc(id)
	}
	return "", storage.ErrNotFound
}

// TestRouterIntegration проверяет полную маршрутизацию сервера:
//   - POST /             – plain text → 201
//   - GET /{id}          – редирект 307
//   - POST /api/shorten  – JSON → 201
//   - Некорректные запросы – 400
func TestRouterIntegration(t *testing.T) {

	mockSvc := &mockShortener{
		createFunc: func(originalURL, baseURL string) (string, error) {

			return baseURL + "abc123", nil
		},
		getFunc: func(id string) (string, error) {
			if id == "abc123" {
				return "https://original.com", nil
			}
			return "", storage.ErrNotFound
		},
	}

	handler := handlers.NewShortenHandler(mockSvc, "http://localhost:8080/")

	logger := zerolog.Nop()

	router := handlers.NewRouter(handler, logger)

	ts := httptest.NewServer(router)
	defer ts.Close()

	t.Run("POST / plain text success", func(t *testing.T) {
		reqBody := bytes.NewBufferString("https://example.com")
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/", reqBody)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/plain")

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, "http://localhost:8080/abc123", string(body))
	})

	t.Run("POST /api/shorten JSON success", func(t *testing.T) {
		reqBody := bytes.NewBufferString(`{"url":"https://practicum.yandex.ru"}`)
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/shorten", reqBody)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var respJSON struct {
			Result string `json:"result"`
		}
		err = json.NewDecoder(resp.Body).Decode(&respJSON)
		require.NoError(t, err)
		assert.Equal(t, "http://localhost:8080/abc123", respJSON.Result)
	})

	t.Run("GET /abc123 redirect", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, ts.URL+"/abc123", nil)
		require.NoError(t, err)

		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // отключаем автоматический редирект
			},
		}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)
		assert.Equal(t, "https://original.com", resp.Header.Get("Location"))
	})

	t.Run("PUT / returns 400", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPut, ts.URL+"/", nil)
		require.NoError(t, err)
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("POST /unknown returns 400", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/unknown", nil)
		require.NoError(t, err)
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}
