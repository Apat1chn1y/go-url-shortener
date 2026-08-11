package handlers_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http/httptest"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/audit"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/handlers"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
	"github.com/rs/zerolog"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/testutil"
)

// ExampleShortenHandler_Create демонстрирует создание короткой ссылки через POST /.
func ExampleShortenHandler_Create() {
	logger := zerolog.Nop()
	store := storage.NewInMemoryStorage()
	shortener := service.NewShortener(store)
	handler := handlers.NewShortenHandler(shortener, "http://localhost:8080/", logger, audit.NewManager())

	body := []byte("https://example.com")
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	resp := rr.Result()
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	fmt.Printf("Status: %d, Short URL: %s\n", resp.StatusCode, data)
	// Пример вывода: Status: 201, Short URL: http://localhost:8080/abc123
}

func ExampleShortenHandler_Redirect() {
	logger := zerolog.Nop()
	store := storage.NewInMemoryStorage()
	shortener := service.NewShortener(store)
	handler := handlers.NewShortenHandler(shortener, "http://localhost:8080/", logger, audit.NewManager())

	shortURL, _ := shortener.Create("https://ya.ru", "http://localhost:8080/", testutil.TestUserID)
	id := shortURL[len("http://localhost:8080/"):]

	req := httptest.NewRequest("GET", "/"+id, nil)
	rr := httptest.NewRecorder()
	handler.Redirect(rr, req)

	resp := rr.Result()
	defer resp.Body.Close()

	fmt.Printf("Status: %d, Location: %s\n", resp.StatusCode, resp.Header.Get("Location"))
	// Пример вывода: Status: 307, Location: https://ya.ru
}
