// Package handlers предоставляет HTTP-обработчики и маршрутизацию для сервиса сокращения URL.
package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// NewRouter создаёт и настраивает маршрутизатор chi со всеми необходимыми эндпоинтами.
// Принимает подготовленный обработчик ShortenHandler и возвращает http.Handler.
func NewRouter(h *ShortenHandler) http.Handler {
	r := chi.NewRouter()

	// Регистрируем единственные разрешённые маршруты
	r.Post("/", h.Create)
	r.Get("/{id}", h.Redirect)

	// Перехват всех остальных запросов (неправильный метод, путь, отсутствие id)
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Bad Request", http.StatusBadRequest)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Bad Request", http.StatusBadRequest)
	})

	return r
}
