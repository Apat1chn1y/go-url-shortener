package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/audit"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/handlers"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAndRedirectIntegration(t *testing.T) {
	logger := zerolog.Nop()
	store := storage.NewInMemoryStorage()
	shortener := service.NewShortener(store)
	handler := handlers.NewShortenHandler(shortener, "http://localhost:8080/", logger, audit.NewManager())

	req := handlers.NewRequestWithUserID(http.MethodPost, "/", []byte("https://example.com"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	shortURL := rr.Body.String()
	assert.Contains(t, shortURL, "http://localhost:8080/")

	id := strings.TrimPrefix(shortURL, "http://localhost:8080/")
	require.NotEmpty(t, id)

	reqRedirect := httptest.NewRequest(http.MethodGet, "/"+id, nil)
	rrRedirect := httptest.NewRecorder()
	handler.Redirect(rrRedirect, reqRedirect)

	assert.Equal(t, http.StatusTemporaryRedirect, rrRedirect.Code)
	assert.Equal(t, "https://example.com", rrRedirect.Header().Get("Location"))
}

func TestCreateAndRedirectFileIntegration(t *testing.T) {
	logger := zerolog.Nop()
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "storage.json")
	err := os.WriteFile(storagePath, []byte("[]"), 0644)
	require.NoError(t, err)

	store, err := storage.NewFileStorage(storagePath)
	require.NoError(t, err)
	shortener := service.NewShortener(store)
	handler := handlers.NewShortenHandler(shortener, "http://localhost:8080/", logger, audit.NewManager())

	req := handlers.NewRequestWithUserID(http.MethodPost, "/", []byte("https://example.com"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	shortURL := rr.Body.String()
	assert.Contains(t, shortURL, "http://localhost:8080/")

	id := strings.TrimPrefix(shortURL, "http://localhost:8080/")
	require.NotEmpty(t, id)

	store2, err := storage.NewFileStorage(storagePath)
	require.NoError(t, err)
	shortener2 := service.NewShortener(store2)
	handler2 := handlers.NewShortenHandler(shortener2, "http://localhost:8080/", logger, audit.NewManager())

	reqRedirect := httptest.NewRequest(http.MethodGet, "/"+id, nil)
	rrRedirect := httptest.NewRecorder()
	handler2.Redirect(rrRedirect, reqRedirect)

	assert.Equal(t, http.StatusTemporaryRedirect, rrRedirect.Code)
	assert.Equal(t, "https://example.com", rrRedirect.Header().Get("Location"))
}
