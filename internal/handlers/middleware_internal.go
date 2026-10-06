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
// Если trustedSubnet пуст или некорректен — доступ запрещён для всех запросов (403).
// Если IP-адрес отсутствует или не входит в подсеть — 403 Forbidden.
func InternalOnly(trustedSubnet string, logger zerolog.Logger) func(http.Handler) http.Handler {
	var subnet *net.IPNet
	if trustedSubnet != "" {
		_, parsed, err := net.ParseCIDR(trustedSubnet)
		if err != nil {
			logger.Error().
				Err(err).
				Str("cidr", trustedSubnet).
				Msg("invalid trusted_subnet CIDR, access will be denied for all")
		} else {
			subnet = parsed
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Если подсеть не задана или некорректна — запрещаем всё.
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
			if ip == nil || !subnet.Contains(ip) {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
