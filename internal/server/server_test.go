package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newServer(t *testing.T) http.Handler {
	h, _ := newServerIn(t)
	return h
}

// newServerIn is a server on a fresh folder, with a home of its own for the config file.
func newServerIn(t *testing.T) (h http.Handler, home string) {
	t.Helper()
	root, home := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "2026"), 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{
		Root: root, CacheDir: t.TempDir(), Port: 4000,
		Static: os.DirFS("../../web"), Home: home,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s.Handler(), home
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
		{"an image of another website", "GET", "127.0.0.1:4000", "/api/logo?d=example.com", "", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"the page itself", "GET", "127.0.0.1:4000", "/api/ls?dir=2026", "", map[string]string{"Sec-Fetch-Site": "same-origin"}, 200},
		{"typed in the address bar", "GET", "127.0.0.1:4000", "/", "", map[string]string{"Sec-Fetch-Site": "none"}, 200},
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

func TestSettings(t *testing.T) {
	h, home := newServerIn(t)
	inc := map[string]string{"X-Inc": "1", "X-Lang": "en"}
	other := t.TempDir()
	cases := []struct {
		name, method, path, body string
		want                     int
	}{
		{"read", "GET", "/api/settings", "", 200},
		{"an hour that does not exist", "POST", "/api/settings", `{"notify":{"hour":25}}`, 400},
		{"reminders at 8", "POST", "/api/settings", `{"notify":{"payments":true,"hour":8}}`, 200},
		{"no download of icons", "POST", "/api/settings", `{"privacy":{"rates":true,"logos":false}}`, 200},
		{"a creation day that is not a day", "POST", "/api/admin", `{"urssaf":"off","created":"10/05/2026"}`, 400},
		{"a backup outside the folder", "POST", "/api/backups/restore", `{"name":"../db.json"}`, 400},
		{"the Mac app reports", "POST", "/api/app-status", `{"notifications":"authorized","loginItem":true}`, 200},
		{"diagnostic", "GET", "/api/diagnostic", "", 200},
		{"a relative folder", "POST", "/api/folder", `{"path":"Documents/X"}`, 400},
		{"a folder that does not exist", "POST", "/api/folder", `{"path":"` + filepath.Join(other, "nope") + `"}`, 400},
		{"another folder", "POST", "/api/folder", `{"path":"` + other + `"}`, 200},
	}
	for _, c := range cases {
		if got := do(h, c.method, "127.0.0.1:4000", c.path, c.body, inc); got.Code != c.want {
			t.Errorf("%s: %d %s, want %d", c.name, got.Code, strings.TrimSpace(got.Body.String()), c.want)
		}
	}
	body := do(h, "GET", "127.0.0.1:4000", "/api/settings", "", inc).Body.String()
	for _, want := range []string{`"hour":8`, `"logos":false`, `"loginItem":true`} {
		if !strings.Contains(body, want) {
			t.Errorf("settings lack %s: %s", want, body)
		}
	}
	if got := do(h, "POST", "127.0.0.1:4000", "/api/settings", `{"notify":{"hour":-1}}`, inc).Body.String(); strings.Contains(got, "error.") || strings.TrimSpace(got) == "" {
		t.Errorf("a refusal is a sentence, never a key: %q", got)
	}
	if data, err := os.ReadFile(filepath.Join(home, "Library", "Application Support", "Kanso", "config.json")); err != nil || !strings.Contains(string(data), other) {
		t.Errorf("the new folder must be saved in the config file: %s, %v", data, err)
	}
}
