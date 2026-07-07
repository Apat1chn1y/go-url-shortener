// Package auth предоставляет функции для генерации и проверки подписанных кук.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
)

const (
	CookieName   = "user_id"
	cookieMaxAge = 86400 * 30 // 30 дней
)

// GenerateUserID генерирует случайный идентификатор пользователя (16 байт в hex).
func GenerateUserID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// sign вычисляет HMAC-SHA256 подпись для данных с заданным ключом.
func sign(data string, key []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}

// verify проверяет подпись данных.
func verify(data, signature string, key []byte) bool {
	expected := sign(data, key)
	return hmac.Equal([]byte(signature), []byte(expected))
}

// SetUserCookie создаёт и устанавливает подписанную куку с userID.
func SetUserCookie(w http.ResponseWriter, userID string, key []byte) {
	sig := sign(userID, key)
	value := userID + "|" + sig
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   cookieMaxAge,
		HttpOnly: true,
		Secure:   false, // для локальной разработки;
	})
}

// GetUserID извлекает и проверяет куку; возвращает userID или пустую строку.
func GetUserID(r *http.Request, key []byte) (string, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return "", nil // куки нет
	}
	parts := splitCookieValue(cookie.Value)
	if len(parts) != 2 {
		return "", errors.New("invalid cookie format")
	}
	userID, sig := parts[0], parts[1]
	if !verify(userID, sig, key) {
		return "", errors.New("invalid signature")
	}
	return userID, nil
}

// splitCookieValue разделяет значение куки на userID и подпись.
func splitCookieValue(value string) []string {
	for i := 0; i < len(value); i++ {
		if value[i] == '|' {
			return []string{value[:i], value[i+1:]}
		}
	}
	return nil
}
