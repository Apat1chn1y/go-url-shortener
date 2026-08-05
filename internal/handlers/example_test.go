package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/audit"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/handlers"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
	"github.com/rs/zerolog"
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

// ExampleShortenHandler_HandleShortenJSON демонстрирует создание короткой ссылки через JSON API.
func ExampleShortenHandler_HandleShortenJSON() {
	logger := zerolog.Nop()
	store := storage.NewInMemoryStorage()
	shortener := service.NewShortener(store)
	handler := handlers.NewShortenHandler(shortener, "http://localhost:8080/", logger, audit.NewManager())

	body := []byte(`{"url":"https://practicum.yandex.ru"}`)
	req := httptest.NewRequest("POST", "/api/shorten", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.HandleShortenJSON(rr, req)

	resp := rr.Result()
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	var result map[string]string
	json.Unmarshal(data, &result)

	fmt.Printf("Status: %d, Short URL: %s\n", resp.StatusCode, result["result"])
}

// ExampleShortenHandler_Redirect демонстрирует редирект по короткой ссылке.
func ExampleShortenHandler_Redirect() {
	logger := zerolog.Nop()
	store := storage.NewInMemoryStorage()
	shortener := service.NewShortener(store)
	handler := handlers.NewShortenHandler(shortener, "http://localhost:8080/", logger, audit.NewManager())

	// Создаём короткую ссылку
	shortURL, _ := shortener.Create("https://ya.ru", "http://localhost:8080/", handlers.TestUserID)
	id := shortURL[len("http://localhost:8080/"):]

	// Запрашиваем редирект
	req := httptest.NewRequest("GET", "/"+id, nil)
	rr := httptest.NewRecorder()
	handler.Redirect(rr, req)

	resp := rr.Result()
	defer resp.Body.Close()

	fmt.Printf("Status: %d, Location: %s\n", resp.StatusCode, resp.Header.Get("Location"))
	// Пример вывода: Status: 307, Location: https://ya.ru
}

// ExampleShortenHandler_HandleBatchShorten демонстрирует массовое создание ссылок.
func ExampleShortenHandler_HandleBatchShorten() {
	logger := zerolog.Nop()
	store := storage.NewInMemoryStorage()
	shortener := service.NewShortener(store)
	handler := handlers.NewShortenHandler(shortener, "http://localhost:8080/", logger, audit.NewManager())

	body := []byte(`[{"correlation_id":"1","original_url":"https://ya.ru"},{"correlation_id":"2","original_url":"https://google.com"}]`)
	req := httptest.NewRequest("POST", "/api/shorten/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.HandleBatchShorten(rr, req)

	resp := rr.Result()
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	var results []map[string]string
	json.Unmarshal(data, &results)

	fmt.Printf("Status: %d\n", resp.StatusCode)
	for _, r := range results {
		fmt.Printf("ID: %s, URL: %s\n", r["correlation_id"], r["short_url"])
	}
}
