package handlers

import (
	"net"
	"net/http"

	"github.com/rs/zerolog"
)

// InternalOnly возвращает middleware, который пропускает запросы
// только с IP-адресом из доверенной подсети.
//
// IP-адрес клиента берётся из заголовка X-Real-IP.
// Если subnet == nil (доверенная подсеть не задана) — доступ запрещён
// для всех запросов (403). Валидность CIDR проверяется на старте в config.NewConfig.
// Если IP-адрес отсутствует или не входит в подсеть — 403 Forbidden.
func InternalOnly(subnet *net.IPNet, logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Если подсеть не задана — запрещаем всё.
			if subnet == nil {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			ipStr := r.Header.Get("X-Real-IP")
			if ipStr == "" {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			ip := net.ParseIP(ipStr)
			if ip == nil {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			// Нормализуем IPv4-mapped IPv6 (например, ::ffff:127.0.0.1) к 4-байтовой форме,
			// чтобы сравнение с IPv4-подсетью работало корректно.
			if v4 := ip.To4(); v4 != nil {
				ip = v4
			}
			if !subnet.Contains(ip) {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
