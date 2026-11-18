package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func newServer(t *testing.T) http.Handler {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "2026"), 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{
		Root: root, CacheDir: t.TempDir(), Port: 4000,
		Static: fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s.Handler()
}

func do(h http.Handler, method, host, path, body string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Host = host
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestGuard(t *testing.T) {
	h := newServer(t)
	inc := map[string]string{"X-Inc": "1"}
	cases := []struct {
		name, method, host, path, body string
		header                         map[string]string
		want                           int
	}{
		{"page", "GET", "localhost:4000", "/", "", nil, 200},
		{"another host (DNS rebinding)", "GET", "evil.example:4000", "/api/ls", "", nil, 403},
		{"another port", "GET", "127.0.0.1:4001", "/api/ls", "", nil, 403},
		{"write without the header (a form of another site)", "POST", "127.0.0.1:4000", "/api/todos", `{"text":"x"}`, nil, 403},
		{"write with the header", "POST", "127.0.0.1:4000", "/api/todos", `{"text":"x"}`, inc, 200},
		{"empty text", "POST", "127.0.0.1:4000", "/api/todos", `{"text":" "}`, inc, 400},
		{"broken JSON", "POST", "127.0.0.1:4000", "/api/todos", `{`, inc, 400},
		{"hidden file", "GET", "localhost:4000", "/files/.kanso/db.json", "", nil, 400},
		{"folder listing", "GET", "localhost:4000", "/api/ls?dir=2026", "", nil, 200},
		{"missing folder", "GET", "localhost:4000", "/api/ls?dir=1999", "", nil, 404},
		{"calendar too long", "GET", "localhost:4000", "/api/calendar?from=2026-01-01&to=2028-01-01", "", nil, 400},
	}
	for _, c := range cases {
		if got := do(h, c.method, c.host, c.path, c.body, c.header).Code; got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
}
