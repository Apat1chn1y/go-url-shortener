package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

func NewRouter(h *ShortenHandler, logger zerolog.Logger, authKey []byte) http.Handler {
	r := chi.NewRouter()
	r.Use(GzipMiddleware)
	r.Use(LoggingMiddleware(logger))
	r.Use(AuthMiddleware(authKey, logger)) // добавляем аутентификацию

	r.Post("/", h.Create)
	r.Get("/{id}", h.Redirect)
	r.Post("/api/shorten", h.HandleShortenJSON)
	r.Post("/api/shorten/batch", h.HandleBatchShorten)
	r.Get("/ping", h.Ping)
	r.Get("/api/user/urls", h.GetUserURLs)
	r.Delete("/api/user/urls", h.DeleteUserURLs)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Bad Request", http.StatusBadRequest)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Bad Request", http.StatusBadRequest)
	})
	return r
}
