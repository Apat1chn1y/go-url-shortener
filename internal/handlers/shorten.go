package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/audit"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
	"github.com/rs/zerolog"
)

type URLShortener interface {
	Create(originalURL, baseURL, userID string) (string, error)
	Get(id string) (string, error)
	Ping() error
	CreateBatch(items []service.BatchItem, baseURL, userID string) ([]service.BatchResult, error)
	FindByOriginal(originalURL string) (string, error)
	GetUserURLs(userID string) ([]storage.UserURL, error)
	DeleteUserURLs(userID string, ids []string) error
}

type ShortenHandler struct {
	shortener URLShortener
	baseURL   string
	logger    zerolog.Logger
	audit     *audit.Manager
}

// DeleteUserURLs обрабатывает DELETE /api/user/urls.
func (h *ShortenHandler) DeleteUserURLs(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var ids []string
	if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
		http.Error(w, "Bad Request: invalid JSON", http.StatusBadRequest)
		return
	}
	if len(ids) == 0 {
		http.Error(w, "Bad Request: empty list", http.StatusBadRequest)
		return
	}

	// Асинхронное удаление
	go func() {
		if err := h.shortener.DeleteUserURLs(userID, ids); err != nil {
			h.logger.Error().Err(err).Str("user_id", userID).Interface("ids", ids).Msg("failed to delete URLs")
		}
	}()

	w.WriteHeader(http.StatusAccepted)
}

func NewShortenHandler(shortener URLShortener, baseURL string, logger zerolog.Logger, audit *audit.Manager) *ShortenHandler {
	return &ShortenHandler{
		shortener: shortener,
		baseURL:   baseURL,
		logger:    logger,
		audit:     audit,
	}
}

// Create обрабатывает POST / (создание короткого URL из plain text).
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
	userID := GetUserIDFromContext(r)

	shortURL, err := h.shortener.Create(originalURL, h.baseURL, userID)
	if err != nil {
		if errors.Is(err, service.ErrEmptyURL) {
			http.Error(w, http.StatusText(http.StatusBadRequest)+": "+err.Error(), http.StatusBadRequest)
			return
		}
		if errors.Is(err, service.ErrMaxAttemptsExceeded) {
			h.logger.Error().Err(err).Str("url", originalURL).Msg("failed to generate unique ID")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if errors.Is(err, service.ErrURLAlreadyExists) {
			// При конфликте тоже считаем успешным действием (пользователь пытался сократить)
			event := audit.Event{
				Ts:     time.Now().Unix(),
				Action: "shorten",
				UserID: userID,
				URL:    originalURL,
			}
			h.audit.Notify(event)

			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(shortURL))
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Успешное создание
	event := audit.Event{
		Ts:     time.Now().Unix(),
		Action: "shorten",
		UserID: userID,
		URL:    originalURL,
	}
	h.audit.Notify(event)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

func (h *ShortenHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/")
	if id == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest)+": empty id", http.StatusBadRequest)
		return
	}
	originalURL, err := h.shortener.Get(id)
	if err != nil {
		if errors.Is(err, storage.ErrGone) {
			http.Error(w, http.StatusText(http.StatusGone), http.StatusGone)
			return
		}
		http.Error(w, http.StatusText(http.StatusBadRequest)+": URL not found", http.StatusBadRequest)
		return
	}

	// Успешный редирект
	userID := GetUserIDFromContext(r)
	event := audit.Event{
		Ts:     time.Now().Unix(),
		Action: "follow",
		UserID: userID,
		URL:    originalURL,
	}
	h.audit.Notify(event)

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

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
	userID := GetUserIDFromContext(r)

	shortURL, err := h.shortener.Create(req.URL, h.baseURL, userID)
	if err != nil {
		if errors.Is(err, service.ErrEmptyURL) {
			http.Error(w, http.StatusText(http.StatusBadRequest)+": "+err.Error(), http.StatusBadRequest)
			return
		}
		if errors.Is(err, service.ErrMaxAttemptsExceeded) {
			h.logger.Error().Err(err).Str("url", req.URL).Msg("failed to generate unique ID")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if errors.Is(err, service.ErrURLAlreadyExists) {
			// Конфликт – тоже считаем действием
			event := audit.Event{
				Ts:     time.Now().Unix(),
				Action: "shorten",
				UserID: userID,
				URL:    req.URL,
			}
			h.audit.Notify(event)

			resp := shortenResponse{Result: shortURL}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(resp)
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Успешное создание
	event := audit.Event{
		Ts:     time.Now().Unix(),
		Action: "shorten",
		UserID: userID,
		URL:    req.URL,
	}
	h.audit.Notify(event)

	resp := shortenResponse{Result: shortURL}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *ShortenHandler) Ping(w http.ResponseWriter, r *http.Request) {
	if err := h.shortener.Ping(); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

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
	userID := GetUserIDFromContext(r)

	results, err := h.shortener.CreateBatch(items, h.baseURL, userID)
	if err != nil {
		if errors.Is(err, service.ErrEmptyURL) ||
			errors.Is(err, service.ErrEmptyOriginalURL) {
			http.Error(w, http.StatusText(http.StatusBadRequest)+": "+err.Error(), http.StatusBadRequest)
			return
		}
		if errors.Is(err, service.ErrMaxAttemptsExceeded) {
			h.logger.Error().Err(err).Msg("failed to generate unique ID for batch")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
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

// GetUserURLs обрабатывает GET /api/user/urls.
func (h *ShortenHandler) GetUserURLs(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userURLs, err := h.shortener.GetUserURLs(userID)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get user URLs")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if len(userURLs) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	// Формируем ответ с полными короткими URL
	type responseItem struct {
		ShortURL    string `json:"short_url"`
		OriginalURL string `json:"original_url"`
	}
	response := make([]responseItem, len(userURLs))
	for i, u := range userURLs {
		response[i] = responseItem{
			ShortURL:    h.baseURL + u.ID,
			OriginalURL: u.OriginalURL,
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

type shortenRequest struct {
	URL string `json:"url"`
}
type shortenResponse struct {
	Result string `json:"result"`
}
