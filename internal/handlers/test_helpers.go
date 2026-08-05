package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
)

const TestUserID = "test-user-123"

// newRequestWithUserID создаёт HTTP-запрос с контекстом, содержащим testUserID.
// Использует константу userIDKey из middleware_auth.go.
func NewRequestWithUserID(method, path string, body []byte) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewBuffer(body))
	ctx := context.WithValue(req.Context(), userIDKey, TestUserID)
	return req.WithContext(ctx)
}
