package handlers

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// mustParseCIDR парсит CIDR или роняет тест — упрощает задание подсетей в таблице.
func mustParseCIDR(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("bad cidr %q: %v", cidr, err)
	}
	return n
}

func TestInternalOnly(t *testing.T) {
	logger := zerolog.Nop()
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		subnet     *net.IPNet
		headerIP   string
		wantStatus int
	}{
		{
			name:       "nil subnet denies all",
			subnet:     nil,
			headerIP:   "127.0.0.1",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "empty header denies",
			subnet:     mustParseCIDR(t, "127.0.0.0/8"),
			headerIP:   "",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "invalid IP denies",
			subnet:     mustParseCIDR(t, "127.0.0.0/8"),
			headerIP:   "not-an-ip",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "IP outside subnet denies",
			subnet:     mustParseCIDR(t, "127.0.0.0/8"),
			headerIP:   "10.0.0.1",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "IP inside subnet allows",
			subnet:     mustParseCIDR(t, "127.0.0.0/8"),
			headerIP:   "127.0.0.1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "IPv4-mapped IPv6 normalized and allowed",
			subnet:     mustParseCIDR(t, "127.0.0.0/8"),
			headerIP:   "::ffff:127.0.0.1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "IPv6 inside IPv6 subnet allows",
			subnet:     mustParseCIDR(t, "::1/128"),
			headerIP:   "::1",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := InternalOnly(tt.subnet, logger)(next)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.headerIP != "" {
				req.Header.Set("X-Real-IP", tt.headerIP)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}
