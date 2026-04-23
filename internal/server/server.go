// Package server предоставляет HTTP-сервер с маршрутизацией на базе chi.
package server

import (
	"net/http"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/handlers"
	"github.com/go-chi/chi/v5"
)

// Server представляет HTTP-сервер для сервиса сокращения URL.
type Server struct {
	httpServer *http.Server
	handler    *handlers.ShortenHandler
}

// New создаёт новый экземпляр Server с заданным адресом и обработчиком.
// Использует chi.Router для маршрутизации: POST / и GET /{id}.
func New(addr string, handler *handlers.ShortenHandler) *Server {
	r := chi.NewRouter()
	// Регистрируем единственные разрешённые маршруты
	r.Post("/", handler.Create)
	r.Get("/{id}", handler.Redirect)
	// Перехват всех остальных запросов (неправильный метод, путь, отсутствие id)
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Bad Request", http.StatusBadRequest)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Bad Request", http.StatusBadRequest)
	})

	return &Server{
		httpServer: &http.Server{
			Addr:    addr,
			Handler: r,
		},
		handler: handler,
	}
}

// Run запускает HTTP-сервер и начинает обрабатывать запросы.
func (s *Server) Run() error {
	return s.httpServer.ListenAndServe()
}
