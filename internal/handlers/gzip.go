// Package handlers предоставляет middleware для поддержки gzip.
package handlers

import (
	"compress/gzip"
	"net/http"
	"strings"
)

// conditionalGzipResponseWriter оборачивает http.ResponseWriter и включает сжатие
// только для типов application/json или text/html.
type conditionalGzipResponseWriter struct {
	http.ResponseWriter
	writer      *gzip.Writer
	wroteHeader bool
	shouldGzip  bool
}

// WriteHeader перехватывает код статуса и принимает решение о сжатии.
func (w *conditionalGzipResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true

	// Не сжимаем, если обработчик уже установил Content-Encoding
	if w.Header().Get("Content-Encoding") != "" {
		w.ResponseWriter.WriteHeader(code)
		return
	}

	ct := w.Header().Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") || strings.HasPrefix(ct, "text/html") {
		w.shouldGzip = true
		w.Header().Del("Content-Length")
		w.Header().Set("Content-Encoding", "gzip")
	}
	w.ResponseWriter.WriteHeader(code)
}

// Write записывает тело ответа, сжимая его при необходимости.
func (w *conditionalGzipResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.shouldGzip {
		return w.writer.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

// Close закрывает gzip.Writer, если сжатие было включено.
func (w *conditionalGzipResponseWriter) Close() error {
	if w.shouldGzip {
		return w.writer.Close()
	}
	return nil
}

// GzipMiddleware обрабатывает сжатые запросы (Content-Encoding: gzip) и
// сжимает ответы (Accept-Encoding: gzip) для типов application/json и text/html.
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ----- Разжатие тела запроса -----
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			originalBody := r.Body
			gzReader, err := gzip.NewReader(originalBody)
			if err != nil {
				http.Error(w, "Bad Request: invalid gzip body", http.StatusBadRequest)
				return
			}

			r.Body = gzReader
			defer func() {
				gzReader.Close()
				originalBody.Close()
			}()
		}

		// ----- Сжатие ответа -----
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			gzWriter := gzip.NewWriter(w)
			wrapped := &conditionalGzipResponseWriter{
				ResponseWriter: w,
				writer:         gzWriter,
				wroteHeader:    false,
				shouldGzip:     false,
			}
			defer wrapped.Close()
			next.ServeHTTP(wrapped, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}
