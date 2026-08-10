package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/audit"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/rs/zerolog"
)

// testUserID — фиктивный идентификатор пользователя, используемый в тестах внутри пакета.
const testUserID = "test-user-123"

// newRequestWithUserID создаёт HTTP-запрос с контекстом, содержащим testUserID.
func newRequestWithUserID(method, path string, body []byte) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewBuffer(body))
	ctx := context.WithValue(req.Context(), userIDKey, testUserID)
	return req.WithContext(ctx)
}

// newTestHandler создаёт хендлер с заглушкой для аудита.
func newTestHandler(shortener *service.Shortener, baseURL string, logger zerolog.Logger) *ShortenHandler {
	return NewShortenHandler(shortener, baseURL, logger, audit.NewManager())
}
