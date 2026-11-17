package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsLoopbackHost(t *testing.T) {
	t.Parallel()
	tests := []struct {
		host string
		want bool
	}{
		// what a real browser sends when the user opens the served URL
		{"127.0.0.1:6275", true},
		{"localhost:6275", true},
		{"LocalHost:6275", true},
		{"[::1]:6275", true},
		{"127.0.0.1", true},
		{"localhost", true},
		{"[::1]", true},
		{"::1", true},
		{"127.94.0.1:6275", true}, // the whole 127.0.0.0/8 is loopback

		// what a rebound request carries
		{"evil.example:6275", false},
		{"evil.example", false},
		{"attacker.test:6275", false},
		// a name that merely CONTAINS a loopback name is not one
		{"localhost.evil.example:6275", false},
		{"notlocalhost:6275", false},
		{"evil.example:6275.localhost", false},
		// non-loopback addresses, including ones reachable on a LAN
		{"192.168.1.10:6275", false},
		{"0.0.0.0:6275", false},
		{"[2001:db8::1]:6275", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isLoopbackHost(tt.host); got != tt.want {
			t.Errorf("isLoopbackHost(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}

// The regression this exists for: a page on another origin re-resolves its own
// domain to 127.0.0.1 and reads every document mo is serving. The Host header
// is the only thing that tells the two apart.
func TestWithLoopbackHostOnly(t *testing.T) {
	t.Parallel()
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("document content")); err != nil {
			t.Error(err)
		}
	})
	handler := WithLoopbackHostOnly(next)

	t.Run("the user's own browser is served", func(t *testing.T) {
		t.Parallel()
		for _, host := range []string{"127.0.0.1:6275", "localhost:6275", "[::1]:6275"} {
			req := httptest.NewRequest("GET", "/_/api/groups", nil)
			req.Host = host
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("Host %q: got %d, want %d", host, rec.Code, http.StatusOK)
			}
		}
	})

	t.Run("a rebound request is refused before the handler runs", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest("GET", "/_/api/groups/g/files/abc/content", nil)
		req.Host = "evil.example:6275"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want %d", rec.Code, http.StatusForbidden)
		}
		if body := rec.Body.String(); body == "document content" {
			t.Fatal("the wrapped handler ran and served content to a rebound request")
		}
	})
}
