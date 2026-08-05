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

const TestUserID = "test-user-123"

// newRequestWithUserID создаёт HTTP-запрос с контекстом, содержащим testUserID.
// Использует константу userIDKey из middleware_auth.go.
func NewRequestWithUserID(method, path string, body []byte) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewBuffer(body))
	ctx := context.WithValue(req.Context(), userIDKey, TestUserID)
	return req.WithContext(ctx)
}

// NewTestHandler создаёт хендлер с заглушкой для аудита.
func NewTestHandler(shortener *service.Shortener, baseURL string, logger zerolog.Logger) *ShortenHandler {
	return NewShortenHandler(shortener, baseURL, logger, audit.NewManager())
}
