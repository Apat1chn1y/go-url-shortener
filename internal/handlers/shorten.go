// Package handlers содержит HTTP-обработчики для сервиса сокращения URL.
package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// URLShortener определяет контракт бизнес-логики, необходимый обработчикам.
type URLShortener interface {
	Create(originalURL, baseURL string) (string, error)
	Get(id string) (string, error)
}

// ShortenHandler привязывает HTTP-запросы к сервису сокращения URL.
type ShortenHandler struct {
	shortener URLShortener
	baseURL   string
}

// NewShortenHandler создаёт новый обработчик с заданным сервисом и базовым URL.
func NewShortenHandler(shortener URLShortener, baseURL string) *ShortenHandler {
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

// shortenRequest представляет тело запроса для JSON-эндпоинта.
type shortenRequest struct {
	URL string `json:"url"`
}

// shortenResponse представляет тело ответа для JSON-эндпоинта.
type shortenResponse struct {
	Result string `json:"result"`
}

// HandleShortenJSON обрабатывает POST /api/shorten – создаёт короткий URL из JSON.
// Ожидает тело: {"url":"<original_url>"}
// При успехе возвращает статус 201 и JSON: {"result":"<short_url>"}
func (h *ShortenHandler) HandleShortenJSON(w http.ResponseWriter, r *http.Request) {

	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Bad Request: Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	var req shortenRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Bad Request: invalid JSON", http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		http.Error(w, "Bad Request: url field is empty", http.StatusBadRequest)
		return
	}

	shortURL, err := h.shortener.Create(req.URL, h.baseURL)
	if err != nil {
		http.Error(w, "Bad Request: "+err.Error(), http.StatusBadRequest)
		return
	}

	resp := shortenResponse{Result: shortURL}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
