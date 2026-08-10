package handlers

import (
	"context"
	"net/http"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/auth"
	"github.com/rs/zerolog"
)

type contextKey string

const userIDKey contextKey = "userID"

// UserIDKey возвращает ключ контекста для userID (используется в тестах).
func UserIDKey() contextKey {
	return userIDKey
}

// AuthMiddleware проверяет наличие валидной куки; если её нет или она невалидна,
// генерирует новую и устанавливает через Set-Cookie.
func AuthMiddleware(key []byte, logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, err := auth.GetUserID(r, key)
			if err != nil {
				logger.Warn().Err(err).Msg("invalid user cookie, will regenerate")
			}
			if userID == "" {
				// Генерируем новый ID
				newID, err := auth.GenerateUserID()
				if err != nil {
					logger.Error().Err(err).Msg("failed to generate user ID")
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				userID = newID
				auth.SetUserCookie(w, userID, key)
			}
			// Сохраняем userID в контексте запроса для дальнейшего использования
			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserIDFromContext извлекает userID из контекста запроса.
func GetUserIDFromContext(r *http.Request) string {
	if v := r.Context().Value(userIDKey); v != nil {
		return v.(string)
	}
	return ""
}
