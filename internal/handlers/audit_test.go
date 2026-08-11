package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/audit"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAuditWriter struct {
	mu    sync.Mutex
	Count int
	Last  audit.Event
}

func (w *mockAuditWriter) Write(ctx context.Context, event audit.Event) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Count++
	w.Last = event
	return nil
}

func (w *mockAuditWriter) getLast() audit.Event {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Last
}

func (w *mockAuditWriter) getCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Count
}

// waitForAudit ожидает, пока количество событий достигнет ожидаемого.
func waitForAudit(t *testing.T, w *mockAuditWriter, expected int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if w.getCount() >= expected {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected %d audit events, got %d", expected, w.getCount())
}

func TestAuditShorten(t *testing.T) {
	logger := zerolog.Nop()
	mockWriter := &mockAuditWriter{}
	auditManager := audit.NewManager()
	auditManager.SetSyncMode(true) // синхронный режим
	auditManager.AddWriter(mockWriter)

	store := storage.NewInMemoryStorage()
	shortener := service.NewShortener(store)
	handler := NewShortenHandler(shortener, "http://localhost:8080/", logger, auditManager)

	req := newRequestWithUserID(http.MethodPost, "/", []byte("https://ya.ru"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	// Проверяем, что событие записалось синхронно
	assert.Equal(t, 1, mockWriter.getCount())
	event := mockWriter.getLast()
	assert.Equal(t, "shorten", event.Action)
	assert.Equal(t, testUserID, event.UserID)
	assert.Equal(t, "https://ya.ru", event.URL)
	assert.NotZero(t, event.Ts)
}

func TestAuditShortenJSON(t *testing.T) {
	logger := zerolog.Nop()
	mockWriter := &mockAuditWriter{}
	auditManager := audit.NewManager()
	auditManager.SetSyncMode(true)
	auditManager.AddWriter(mockWriter)

	store := storage.NewInMemoryStorage()
	shortener := service.NewShortener(store)
	handler := NewShortenHandler(shortener, "http://localhost:8080/", logger, auditManager)

	req := newRequestWithUserID(http.MethodPost, "/api/shorten", []byte(`{"url":"https://ya.ru"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.HandleShortenJSON(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, 1, mockWriter.getCount())
	event := mockWriter.getLast()
	assert.Equal(t, "shorten", event.Action)
	assert.Equal(t, testUserID, event.UserID)
	assert.Equal(t, "https://ya.ru", event.URL)
	assert.NotZero(t, event.Ts)
}

func TestAuditRedirect(t *testing.T) {
	logger := zerolog.Nop()
	mockWriter := &mockAuditWriter{}
	auditManager := audit.NewManager()
	auditManager.SetSyncMode(true)
	auditManager.AddWriter(mockWriter)

	store := storage.NewInMemoryStorage()
	shortener := service.NewShortener(store)

	shortURL, err := shortener.Create("https://ya.ru", "http://localhost:8080/", testUserID)
	require.NoError(t, err)
	id := strings.TrimPrefix(shortURL, "http://localhost:8080/")
	require.NotEmpty(t, id)

	handler := NewShortenHandler(shortener, "http://localhost:8080/", logger, auditManager)

	req := newRequestWithUserID(http.MethodGet, "/"+id, nil)
	rr := httptest.NewRecorder()
	handler.Redirect(rr, req)

	assert.Equal(t, http.StatusTemporaryRedirect, rr.Code)
	assert.Equal(t, 1, mockWriter.getCount())
	event := mockWriter.getLast()
	assert.Equal(t, "follow", event.Action)
	assert.Equal(t, testUserID, event.UserID)
	assert.Equal(t, "https://ya.ru", event.URL)
	assert.NotZero(t, event.Ts)
}

func TestAuditShortenDuplicate(t *testing.T) {
	logger := zerolog.Nop()
	mockWriter := &mockAuditWriter{}
	auditManager := audit.NewManager()
	auditManager.SetSyncMode(true)
	auditManager.AddWriter(mockWriter)

	store := storage.NewInMemoryStorage()
	shortener := service.NewShortener(store)

	_, err := shortener.Create("https://ya.ru", "http://localhost:8080/", testUserID)
	require.NoError(t, err)

	handler := NewShortenHandler(shortener, "http://localhost:8080/", logger, auditManager)

	req := newRequestWithUserID(http.MethodPost, "/", []byte("https://ya.ru"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Equal(t, 1, mockWriter.getCount())
	event := mockWriter.getLast()
	assert.Equal(t, "shorten", event.Action)
	assert.Equal(t, testUserID, event.UserID)
	assert.Equal(t, "https://ya.ru", event.URL)
	assert.NotZero(t, event.Ts)
}

func TestAuditNoWriter(t *testing.T) {
	logger := zerolog.Nop()
	auditManager := audit.NewManager()

	store := storage.NewInMemoryStorage()
	shortener := service.NewShortener(store)
	handler := NewShortenHandler(shortener, "http://localhost:8080/", logger, auditManager)

	req := newRequestWithUserID(http.MethodPost, "/", []byte("https://ya.ru"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}
