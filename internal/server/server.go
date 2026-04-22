// Package server предоставляет HTTP-сервер с маршрутизацией.
package server

import (
	"net/http"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/handlers"
)

// Server представляет HTTP-сервер для сервиса сокращения URL.
type Server struct {
	httpServer *http.Server
	handler    *handlers.ShortenHandler
}

// New создаёт новый экземпляр Server с заданным адресом и обработчиком.
// Выполняет настройку маршрутов: POST / и GET /{id}.
func New(addr string, handler *handlers.ShortenHandler) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /", handler.Create)
	mux.HandleFunc("GET /{id}", handler.Redirect)

	return &Server{
		httpServer: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
		handler: handler,
	}
}

// Run запускает HTTP-сервер и начинает обрабатывать запросы.
// Блокирует выполнение до остановки сервера или возникновения ошибки.
func (s *Server) Run() error {
	return s.httpServer.ListenAndServe()
}
