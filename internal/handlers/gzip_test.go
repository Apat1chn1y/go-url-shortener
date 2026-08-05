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

func TestGzipMiddleware(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("response compressed for /api/shorten with Accept-Encoding: gzip", func(t *testing.T) {
		mockStore := storagemocks.NewMockStorage(t)
		mockStore.EXPECT().FindByOriginal("https://ya.ru").Return("", storage.ErrNotFound).Once()
		mockStore.EXPECT().SaveForUser(mock.Anything, "https://ya.ru", mock.Anything).Return(nil).Once()

		shortener := service.NewShortener(mockStore)
		handler := NewShortenHandler(shortener, "http://localhost:8080/", logger)
		router := NewRouter(handler, logger, []byte("test-key"))

		body := `{"url":"https://ya.ru"}`
		req := NewRequestWithUserID(http.MethodPost, "/api/shorten", []byte(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Encoding", "gzip")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))

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

	t.Run("plain text response not compressed", func(t *testing.T) {
		mockStore := storagemocks.NewMockStorage(t)
		mockStore.EXPECT().FindByOriginal("https://ya.ru").Return("", storage.ErrNotFound).Once()
		mockStore.EXPECT().SaveForUser(mock.Anything, "https://ya.ru", mock.Anything).Return(nil).Once()

		shortener := service.NewShortener(mockStore)
		handler := NewShortenHandler(shortener, "http://localhost:8080/", logger)
		router := NewRouter(handler, logger, []byte("test-key"))

		req := NewRequestWithUserID(http.MethodPost, "/", []byte("https://ya.ru"))
		req.Header.Set("Content-Type", "text/plain")
		req.Header.Set("Accept-Encoding", "gzip")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.NotEqual(t, "gzip", rr.Header().Get("Content-Encoding"))
	})

	t.Run("request compressed with Content-Encoding: gzip", func(t *testing.T) {
		mockStore := storagemocks.NewMockStorage(t)
		mockStore.EXPECT().FindByOriginal("https://ya.ru").Return("", storage.ErrNotFound).Once()
		mockStore.EXPECT().SaveForUser(mock.Anything, "https://ya.ru", mock.Anything).Return(nil).Once()

		shortener := service.NewShortener(mockStore)
		handler := NewShortenHandler(shortener, "http://localhost:8080/", logger)
		router := NewRouter(handler, logger, []byte("test-key"))

		var buf bytes.Buffer
		gzWriter := gzip.NewWriter(&buf)
		_, err := gzWriter.Write([]byte(`{"url":"https://ya.ru"}`))
		require.NoError(t, err)
		err = gzWriter.Close()
		require.NoError(t, err)

		req := NewRequestWithUserID(http.MethodPost, "/api/shorten", buf.Bytes())
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Encoding", "gzip")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("invalid gzip request returns 400", func(t *testing.T) {
		mockStore := storagemocks.NewMockStorage(t)
		shortener := service.NewShortener(mockStore)
		handler := NewShortenHandler(shortener, "http://localhost:8080/", logger)
		router := NewRouter(handler, logger, []byte("test-key"))

		req := NewRequestWithUserID(http.MethodPost, "/api/shorten", []byte("not gzip"))
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid gzip body")
	})
}
