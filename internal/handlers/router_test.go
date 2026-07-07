package handlers_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/handlers"
	mocks "github.com/Apat1chn1y/go-url-shortener.git/internal/mocks/handlers"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRouterIntegration(t *testing.T) {
	// Используем правильное имя функции для создания мока
	mockShortener := mocks.NewMockURLShortener(t)

	// Настраиваем ожидания
	mockShortener.EXPECT().
		Create("https://example.com", "http://localhost:8080/", mock.Anything).
		Return("http://localhost:8080/abc123", nil).
		Times(1)

	mockShortener.EXPECT().
		Create("https://practicum.yandex.ru", "http://localhost:8080/", mock.Anything).
		Return("http://localhost:8080/def456", nil).
		Times(1)

	mockShortener.EXPECT().
		Get("abc123").
		Return("https://original.com", nil).
		Times(1)

	// Дополнительные методы – Maybe
	mockShortener.EXPECT().Ping().Return(nil).Maybe()
	mockShortener.EXPECT().FindByOriginal(mock.Anything).Return("", nil).Maybe()
	mockShortener.EXPECT().GetUserURLs(mock.Anything).Return(nil, nil).Maybe()

	handler := handlers.NewShortenHandler(mockShortener, "http://localhost:8080/", zerolog.Nop())
	router := handlers.NewRouter(handler, zerolog.Nop(), []byte("test-key"))
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
		assert.Contains(t, resp.Header.Get("Content-Type"), "text/plain")
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), "http://localhost:8080/")
		assert.Greater(t, len(string(body)), len("http://localhost:8080/"))
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
		assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")

		var respJSON struct {
			Result string `json:"result"`
		}
		err = json.NewDecoder(resp.Body).Decode(&respJSON)
		require.NoError(t, err)
		assert.Contains(t, respJSON.Result, "http://localhost:8080/")
		assert.Greater(t, len(respJSON.Result), len("http://localhost:8080/"))
	})

	t.Run("GET /abc123 redirect", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, ts.URL+"/abc123", nil)
		require.NoError(t, err)

		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
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
