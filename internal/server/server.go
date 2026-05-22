// Package server предоставляет HTTP-сервер с маршрутизацией на базе chi.
package server

import (
	"net/http"
)

type Server struct {
	httpServer *http.Server
}

// New создаёт новый экземпляр Server с заданным адресом и обработчиком.
func New(addr string, handler http.Handler) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
	}
}

// Run запускает HTTP-сервер и начинает обрабатывать запросы.
func (s *Server) Run() error {
	return s.httpServer.ListenAndServe()
}
