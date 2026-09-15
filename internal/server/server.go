// Package server предоставляет HTTP-сервер с маршрутизацией на базе chi.
package server

import (
	"context"
	"net/http"
)

// Server оборачивает стандартный http.Server.
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
// Блокирует выполнение до остановки сервера или возникновения ошибки.
func (s *Server) Run() error {
	return s.httpServer.ListenAndServe()
}

// RunTLS запускает HTTPS-сервер с использованием указанных сертификата и ключа.
// Блокирует выполнение до остановки сервера или возникновения ошибки.
func (s *Server) RunTLS(certFile, keyFile string) error {
	return s.httpServer.ListenAndServeTLS(certFile, keyFile)
}

// Shutdown gracefully останавливает сервер с заданным контекстом.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
