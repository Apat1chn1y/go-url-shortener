package handler

import (
	"io"
	"net/http"
	"strings"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
)

// привязывает запросы к сервису сокращения.
type Handler struct {
	service *service.ShortenerService
	baseURL string
}

// создаёт новый обработчик
func NewHandler(service *service.ShortenerService, baseURL string) *Handler {
	return &Handler{
		service: service,
		baseURL: baseURL,
	}
}

// реализует маршрутизацию
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/":
		h.handlePost(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/") && len(r.URL.Path) > 1:
		h.handleGet(w, r)
	default:
		http.Error(w, "Bad Request", http.StatusBadRequest)
	}
}

// обрабатывает POST – создание короткого URL.
func (h *Handler) handlePost(w http.ResponseWriter, r *http.Request) {

	if r.Header.Get("Content-Type") != "text/plain" {
		http.Error(w, "Bad Request: Content-Type must be text/plain", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "Bad Request: empty or invalid body", http.StatusBadRequest)
		return
	}
	originalURL := string(body)

	shortURL, err := h.service.CreateShortURL(originalURL, h.baseURL)
	if err != nil {
		http.Error(w, "Bad Request: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

// обрабатывает GET - редирект на оригинальный URL
func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {

	id := strings.TrimPrefix(r.URL.Path, "/")
	if id == "" {
		http.Error(w, "Bad Request: empty id", http.StatusBadRequest)
		return
	}

	originalURL, err := h.service.GetOriginalURL(id)
	if err != nil {
		http.Error(w, "Bad Request: URL not found", http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
