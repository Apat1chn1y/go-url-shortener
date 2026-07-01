package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/rs/zerolog"
)

// URLShortener определяет контракт бизнес-логики, необходимый обработчикам.
type URLShortener interface {
	Create(originalURL, baseURL string) (string, error)
	Get(id string) (string, error)
	Ping() error
	CreateBatch(items []service.BatchItem, baseURL string) ([]service.BatchResult, error)
	FindByOriginal(originalURL string) (string, error)
}

// ShortenHandler привязывает HTTP-запросы к сервису сокращения URL.
type ShortenHandler struct {
	shortener URLShortener
	baseURL   string
	logger    zerolog.Logger
}

// NewShortenHandler создаёт новый обработчик с заданным сервисом и базовым URL.
func NewShortenHandler(shortener URLShortener, baseURL string, logger zerolog.Logger) *ShortenHandler {
	return &ShortenHandler{
		shortener: shortener,
		baseURL:   baseURL,
		logger:    logger,
	}
}

// Create обрабатывает POST / – создаёт короткий URL из plain text.
func (h *ShortenHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "text/plain" {
		http.Error(w, http.StatusText(http.StatusBadRequest)+": Content-Type must be text/plain", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest)+": empty or invalid body", http.StatusBadRequest)
		return
	}
	originalURL := string(body)

	shortURL, err := h.shortener.Create(originalURL, h.baseURL)
	if err != nil {
		if errors.Is(err, service.ErrEmptyURL) {
			http.Error(w, http.StatusText(http.StatusBadRequest)+": "+err.Error(), http.StatusBadRequest)
			return
		}
		if errors.Is(err, service.ErrMaxAttemptsExceeded) {
			h.logger.Error().
				Err(err).
				Str("url", originalURL).
				Msg("failed to generate unique ID")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if errors.Is(err, service.ErrURLAlreadyExists) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(shortURL))
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

// Redirect обрабатывает GET /{id} – редирект на оригинальный URL.
func (h *ShortenHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/")
	if id == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest)+": empty id", http.StatusBadRequest)
		return
	}
	originalURL, err := h.shortener.Get(id)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest)+": URL not found", http.StatusBadRequest)
		return
	}
	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

// HandleShortenJSON обрабатывает POST /api/shorten – создаёт короткий URL из JSON.
func (h *ShortenHandler) HandleShortenJSON(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		http.Error(w, http.StatusText(http.StatusBadRequest)+": Content-Type must be application/json", http.StatusBadRequest)
		return
	}
	var req shortenRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest)+": invalid JSON", http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest)+": url field is empty", http.StatusBadRequest)
		return
	}

	shortURL, err := h.shortener.Create(req.URL, h.baseURL)
	if err != nil {
		if errors.Is(err, service.ErrEmptyURL) {
			http.Error(w, http.StatusText(http.StatusBadRequest)+": "+err.Error(), http.StatusBadRequest)
			return
		}
		if errors.Is(err, service.ErrMaxAttemptsExceeded) {
			h.logger.Error().
				Err(err).
				Str("url", req.URL).
				Msg("failed to generate unique ID")
		}
		if errors.Is(err, service.ErrURLAlreadyExists) {
			resp := shortenResponse{Result: shortURL}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(resp)
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	resp := shortenResponse{Result: shortURL}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// Ping обрабатывает GET /ping – проверяет соединение с хранилищем.
func (h *ShortenHandler) Ping(w http.ResponseWriter, r *http.Request) {
	if err := h.shortener.Ping(); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// HandleBatchShorten обрабатывает POST /api/shorten/batch.
func (h *ShortenHandler) HandleBatchShorten(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		http.Error(w, http.StatusText(http.StatusBadRequest)+": Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	var req []struct {
		CorrelationID string `json:"correlation_id"`
		OriginalURL   string `json:"original_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest)+": invalid JSON", http.StatusBadRequest)
		return
	}
	if len(req) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest)+": empty batch", http.StatusBadRequest)
		return
	}

	items := make([]service.BatchItem, len(req))
	for i, v := range req {
		items[i] = service.BatchItem{
			CorrelationID: v.CorrelationID,
			OriginalURL:   v.OriginalURL,
		}
	}

	results, err := h.shortener.CreateBatch(items, h.baseURL)
	if err != nil {
		if errors.Is(err, service.ErrEmptyURL) ||
			errors.Is(err, service.ErrEmptyOriginalURL) {
			http.Error(w, http.StatusText(http.StatusBadRequest)+": "+err.Error(), http.StatusBadRequest)
			return
		}
		if errors.Is(err, service.ErrMaxAttemptsExceeded) {
			h.logger.Error().
				Err(err).
				Msg("failed to generate unique ID for batch")
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	resp := make([]struct {
		CorrelationID string `json:"correlation_id"`
		ShortURL      string `json:"short_url"`
	}, len(results))
	for i, v := range results {
		resp[i].CorrelationID = v.CorrelationID
		resp[i].ShortURL = v.ShortURL
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

// Структуры для JSON-запросов/ответов.
type shortenRequest struct {
	URL string `json:"url"`
}
type shortenResponse struct {
	Result string `json:"result"`
}
