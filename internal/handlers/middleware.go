// Package handlers предоставляет HTTP-обработчики и middleware.
package handlers

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// responseWriterWrapper оборачивает http.ResponseWriter для перехвата кода статуса и размера тела ответа.
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int // код статуса HTTP-ответа
	bodySize   int // размер тела ответа в байтах
}

// WriteHeader перехватывает установку кода статуса и сохраняет его во wrapper.
// После сохранения вызывает оригинальный WriteHeader.
func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write перехватывает запись тела ответа, аккумулирует размер и вызывает оригинальный Write.
// Если WriteHeader не был вызван ранее, устанавливает statusCode = http.StatusOK.
func (rw *responseWriterWrapper) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bodySize += n
	return n, err
}

// LoggingMiddleware возвращает middleware-функцию, которая логирует каждый HTTP-запрос.
// Параметры лога: метод, URI, статус ответа, размер ответа (байт), длительность обработки.
// Уровень логирования – Info.
func LoggingMiddleware(logger zerolog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &responseWriterWrapper{ResponseWriter: w}
			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			logger.Info().
				Str("method", r.Method).
				Str("uri", r.RequestURI).
				Int("status", wrapped.statusCode).
				Int("response_size", wrapped.bodySize).
				Dur("duration", duration).
				Msg("HTTP request")
		})
	}
}
