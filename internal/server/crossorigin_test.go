package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCrossOriginProtection(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		headers map[string]string
		want    int
	}{
		{
			name:    "cross-site POST from a browser is rejected",
			method:  http.MethodPost,
			headers: map[string]string{"Sec-Fetch-Site": "cross-site"},
			want:    http.StatusForbidden,
		},
		{
			name:    "cross-origin POST from a browser without Sec-Fetch-Site is rejected",
			method:  http.MethodPost,
			headers: map[string]string{"Origin": "https://evil.example"},
			want:    http.StatusForbidden,
		},
		{
			name:    "same-origin POST from the SPA is allowed",
			method:  http.MethodPost,
			headers: map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "http://localhost:6275"},
			want:    http.StatusAccepted,
		},
		{
			name:   "POST from the CLI without browser headers is allowed",
			method: http.MethodPost,
			want:   http.StatusAccepted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestState(t)
			handler := NewHandler(s)
			req := httptest.NewRequest(tt.method, "/_/api/shutdown", strings.NewReader(""))
			req.Host = "localhost:6275"
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("got status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
