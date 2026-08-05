package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/audit"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
	"github.com/rs/zerolog"
)

func BenchmarkShortenHandler_Create(b *testing.B) {
	logger := zerolog.Nop()
	store := storage.NewInMemoryStorage()
	shortener := service.NewShortener(store)
	handler := NewShortenHandler(shortener, "http://localhost:8080/", logger, audit.NewManager())

	body := []byte("https://example.com")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := NewRequestWithUserID(http.MethodPost, "/", body)
		req.Header.Set("Content-Type", "text/plain")
		rr := httptest.NewRecorder()
		handler.Create(rr, req)
	}
}

func BenchmarkShortenHandler_HandleShortenJSON(b *testing.B) {
	logger := zerolog.Nop()
	store := storage.NewInMemoryStorage()
	shortener := service.NewShortener(store)
	handler := NewShortenHandler(shortener, "http://localhost:8080/", logger, audit.NewManager())

	body := []byte(`{"url":"https://example.com"}`)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := NewRequestWithUserID(http.MethodPost, "/api/shorten", body)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.HandleShortenJSON(rr, req)
	}
}

func BenchmarkShortenHandler_Redirect(b *testing.B) {
	logger := zerolog.Nop()
	store := storage.NewInMemoryStorage()
	shortener := service.NewShortener(store)
	handler := NewShortenHandler(shortener, "http://localhost:8080/", logger, audit.NewManager())

	// Создаём одну запись для редиректа
	shortURL, _ := shortener.Create("https://example.com", "http://localhost:8080/", TestUserID)
	id := shortURL[len("http://localhost:8080/"):]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := NewRequestWithUserID(http.MethodGet, "/"+id, nil)
		rr := httptest.NewRecorder()
		handler.Redirect(rr, req)
	}
}

func BenchmarkShortenHandler_HandleBatchShorten(b *testing.B) {
	logger := zerolog.Nop()
	store := storage.NewInMemoryStorage()
	shortener := service.NewShortener(store)
	handler := NewShortenHandler(shortener, "http://localhost:8080/", logger, audit.NewManager())

	body := []byte(`[{"correlation_id":"1","original_url":"https://ya.ru"},{"correlation_id":"2","original_url":"https://google.com"}]`)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := NewRequestWithUserID(http.MethodPost, "/api/shorten/batch", body)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.HandleBatchShorten(rr, req)
	}
}
