package lsx

import (
	"net/http"
	"testing"
)

func TestRemoteIPPrefersXForwardedFor(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		header     string
		want       string
	}{
		{
			name:       "remote addr host port",
			remoteAddr: "127.0.0.1:8080",
			want:       "127.0.0.1",
		},
		{
			name:       "forwarded single ip",
			remoteAddr: "127.0.0.1:8080",
			header:     "203.0.113.9",
			want:       "203.0.113.9",
		},
		{
			name:       "forwarded chain uses first client",
			remoteAddr: "127.0.0.1:8080",
			header:     "203.0.113.9, 198.51.100.7",
			want:       "203.0.113.9",
		},
		{
			name:       "empty forwarded falls back",
			remoteAddr: "127.0.0.1:8080",
			header:     " , ",
			want:       "127.0.0.1",
		},
		{
			name:       "unparseable remote addr fallback",
			remoteAddr: "localhost",
			want:       "localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "/", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.RemoteAddr = tt.remoteAddr
			req.Header.Set("X-Forwarded-For", tt.header)

			if got := remoteIP(req); got != tt.want {
				t.Fatalf("remoteIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
