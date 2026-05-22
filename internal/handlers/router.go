// Package handlers предоставляет HTTP-обработчики и маршрутизацию для сервиса сокращения URL.
package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// NewRouter создаёт и настраивает маршрутизатор chi со всеми необходимыми эндпоинтами.
// Принимает подготовленный обработчик ShortenHandler и возвращает http.Handler.
func NewRouter(h *ShortenHandler, logger zerolog.Logger) http.Handler {
	r := chi.NewRouter()
	// Подключает логгирование
	r.Use(LoggingMiddleware(logger))

	// Регистрируем единственные разрешённые маршруты
	r.Post("/", h.Create)
	r.Get("/{id}", h.Redirect)
	r.Post("/api/shorten", h.HandleShortenJSON)

	// Перехват всех остальных запросов (неправильный метод, путь, отсутствие id)
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Bad Request", http.StatusBadRequest)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Bad Request", http.StatusBadRequest)
	})

	return r
}
