package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseUserID_Valid(t *testing.T) {
	key := []byte("test-key")
	sig := sign("user-123", key)
	value := "user-123|" + sig

	userID, err := ParseUserID(value, key)
	require.NoError(t, err)
	assert.Equal(t, "user-123", userID)
}

func TestParseUserID_InvalidSignature(t *testing.T) {
	key := []byte("test-key")
	_, err := ParseUserID("user-123|badsignature", key)
	assert.Error(t, err)
}

func TestParseUserID_NoSeparator(t *testing.T) {
	_, err := ParseUserID("no-separator-here", []byte("test-key"))
	assert.Error(t, err)
}

func TestGetUserID_RoundTrip(t *testing.T) {
	key := []byte("test-key")
	rr := httptest.NewRecorder()
	SetUserCookie(rr, "user-123", key)

	// Извлекаем установленную куку и подсовываем её в запрос
	resp := rr.Result()
	require.Len(t, resp.Cookies(), 1)
	cookie := resp.Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)

	userID, err := GetUserID(req, key)
	require.NoError(t, err)
	assert.Equal(t, "user-123", userID)
}
