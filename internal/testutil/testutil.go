// Package testutil предоставляет вспомогательные функции для тестов.
package testutil

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/audit"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/handlers"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/rs/zerolog"
)

// TestUserID — фиктивный идентификатор пользователя, используемый в тестах.
const TestUserID = "test-user-123"

// NewRequestWithUserID создаёт HTTP-запрос с контекстом, содержащим TestUserID.
// Использует константу userIDKey из пакета handlers.
func NewRequestWithUserID(method, path string, body []byte) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewBuffer(body))
	ctx := context.WithValue(req.Context(), handlers.UserIDKey(), TestUserID)
	return req.WithContext(ctx)
}

// NewTestHandler создаёт хендлер с заглушкой для аудита.
func NewTestHandler(shortener *service.Shortener, baseURL string, logger zerolog.Logger) *handlers.ShortenHandler {
	return handlers.NewShortenHandler(shortener, baseURL, logger, audit.NewManager())
}
