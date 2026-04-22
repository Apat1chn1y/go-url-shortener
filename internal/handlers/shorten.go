// Package handlers содержит HTTP-обработчики для сервиса сокращения URL.
package handlers

import (
	"io"
	"net/http"
	"strings"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
)

// ShortenHandler привязывает HTTP-запросы к сервису сокращения URL.
type ShortenHandler struct {
	shortener *service.Shortener
	baseURL   string
}

// NewShortenHandler создаёт новый обработчик для сокращения URL.
func NewShortenHandler(shortener *service.Shortener, baseURL string) *ShortenHandler {
	return &ShortenHandler{
		shortener: shortener,
		baseURL:   baseURL,
	}
}

// Create обрабатывает POST / – создаёт короткий URL.
// Ожидает тело запроса в формате text/plain с оригинальным URL.
// При успехе возвращает статус 201 и короткий URL в теле.
// При ошибке (неверный Content-Type, пустое тело, сбой сервиса) возвращает статус 400.
func (h *ShortenHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "text/plain" {
		http.Error(w, "Bad Request: Content-Type must be text/plain", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "Bad Request: empty or invalid body", http.StatusBadRequest)
		return
	}
	shortURL, err := h.shortener.Create(string(body), h.baseURL)
	if err != nil {
		http.Error(w, "Bad Request: "+err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

// Redirect обрабатывает GET /{id} – редирект на оригинальный URL.
// Извлекает идентификатор из пути запроса.
// При успехе возвращает статус 307 и заголовок Location с оригинальным URL.
// Если идентификатор пуст или не найден – возвращает статус 400.
func (h *ShortenHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/")
	if id == "" {
		http.Error(w, "Bad Request: empty id", http.StatusBadRequest)
		return
	}
	originalURL, err := h.shortener.Get(id)
	if err != nil {
		http.Error(w, "Bad Request: URL not found", http.StatusBadRequest)
		return
	}
	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
