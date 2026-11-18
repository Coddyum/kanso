package netgate

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestAllows(t *testing.T) {
	cases := []struct {
		p    Purpose
		url  string
		want bool
	}{
		{Rates(), "https://www.ecb.europa.eu/stats/eurofxref/eurofxref-daily.xml", true},
		{Rates(), "http://www.ecb.europa.eu/stats/eurofxref/eurofxref-daily.xml", false},
		{Rates(), "https://ecb.europa.eu.evil.example/x", false},
		{Rates(), "https://www.ecb.europa.eu:8443/x", false},
		{Logo("notion.so"), "https://notion.so/favicon.ico", true},
		{Logo("notion.so"), "https://www.notion.so/images/icon.png", true},
		{Logo("www.notion.so"), "https://notion.so/favicon.ico", true},
		{Logo("notion.so"), "https://www.google.com/s2/favicons?domain=notion.so", false},
		{Logo("notion.so"), "https://evilnotion.so/favicon.ico", false},
		{Logo("notion.so"), "https://cdn.example.com/notion.so/icon.png", false},
		{Logo("notion.so"), "https://user@notion.so/", false}, // privacy:ok (a URL with a user part, not an address)
		{Logo(""), "https://example.com/", false},
	}
	for _, c := range cases {
		if got := c.p.Allows(c.url); got != c.want {
			t.Errorf("%s allows %s = %v, want %v", c.p.name, c.url, got, c.want)
		}
	}
}

// fakeSite answers without any network: /away redirects off the brand's site, /www to its www.
type fakeSite struct{}

func (fakeSite) RoundTrip(r *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	switch r.URL.Path {
	case "/away":
		http.Redirect(rec, r, "https://elsewhere.example/icon.png", http.StatusFound)
	case "/www":
		http.Redirect(rec, r, "https://www.notion.so/ok", http.StatusFound)
	case "/gone":
		rec.WriteHeader(http.StatusNotFound)
	default:
		_, _ = rec.WriteString("icon of " + r.URL.Host)
	}
	return rec.Result(), nil
}

func TestGet(t *testing.T) {
	g := &Gate{client: &http.Client{Transport: fakeSite{}}}
	p := Logo("notion.so")
	if body, err := g.Get(p, "https://notion.so/www", 100); err != nil || string(body) != "icon of www.notion.so" {
		t.Errorf("redirect inside the brand's site: %q, %v", body, err)
	}
	if _, err := g.Get(p, "https://notion.so/away", 100); !errors.Is(err, ErrRefused) {
		t.Errorf("redirect off the brand's site must be refused, got %v", err)
	}
	if _, err := g.Get(p, "https://example.com/", 100); !errors.Is(err, ErrRefused) {
		t.Errorf("a host outside the list must be refused, got %v", err)
	}
	if _, err := g.Get(p, "https://notion.so/gone", 100); err == nil {
		t.Error("a 404 must be an error")
	}
	if body, _ := g.Get(p, "https://notion.so/ok", 4); string(body) != "icon" {
		t.Errorf("the limit must cut the body, got %q", body)
	}
}

// outgoing are the calls that open a connection, by package.
var outgoing = map[string]map[string]bool{
	"net/http": {"Get": true, "Head": true, "Post": true, "PostForm": true, "NewRequest": true, "NewRequestWithContext": true,
		"Client": true, "DefaultClient": true, "Transport": true, "DefaultTransport": true},
	"net": {"Dial": true, "DialTimeout": true, "Dialer": true, "DialTCP": true, "DialUDP": true, "DialIP": true, "DialUnix": true,
		"LookupHost": true, "LookupIP": true, "LookupAddr": true, "Resolver": true},
}

// forbiddenImports open connections whatever the call.
var forbiddenImports = map[string]bool{"net/smtp": true, "net/rpc": true, "net/rpc/jsonrpc": true, "crypto/tls": true, "net/http/httputil": true}

// forbiddenCommands download things when run by os/exec.
var forbiddenCommands = map[string]bool{"curl": true, "wget": true, "nc": true, "ssh": true, "scp": true, "rsync": true, "ftp": true}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("go.mod not found above the package: %v", err)
	}
	return root
}

// walk calls fn on every file of the repository with one of exts, tests and hidden folders left out.
func walk(t *testing.T, exts []string, fn func(rel string, data []byte)) {
	t.Helper()
	root := repoRoot(t)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "bin" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		for _, ext := range exts {
			if strings.HasSuffix(path, ext) && !strings.HasSuffix(path, "_test.go") {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, _ := filepath.Rel(root, path)
				fn(filepath.ToSlash(rel), data)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestOnlyDoor fails when Go code outside this package can open a connection:
// a new download must go through Get, with its own Purpose, reviewed here.
func TestOnlyDoor(t *testing.T) {
	walk(t, []string{".go"}, func(rel string, data []byte) {
		if strings.HasPrefix(rel, "internal/netgate/") {
			return
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, rel, data, parser.SkipObjectResolution)
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			return
		}
		names := map[string]string{} // local name → import path
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if forbiddenImports[path] {
				t.Errorf("%s imports %s: downloads go through internal/netgate", rel, path)
			}
			name := path[strings.LastIndex(path, "/")+1:]
			if imp.Name != nil {
				name = imp.Name.Name
			}
			names[name] = path
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if id, ok := n.X.(*ast.Ident); ok && outgoing[names[id.Name]][n.Sel.Name] {
					t.Errorf("%s: %s.%s opens a connection: go through internal/netgate", fset.Position(n.Pos()), id.Name, n.Sel.Name)
				}
			case *ast.CallExpr:
				sel, ok := n.Fun.(*ast.SelectorExpr)
				if !ok || len(n.Args) == 0 || !strings.HasPrefix(sel.Sel.Name, "Command") {
					return true
				}
				arg := n.Args[0]
				if sel.Sel.Name == "CommandContext" && len(n.Args) > 1 {
					arg = n.Args[1]
				}
				if lit, ok := arg.(*ast.BasicLit); ok {
					if cmd, _ := strconv.Unquote(lit.Value); forbiddenCommands[filepath.Base(cmd)] {
						t.Errorf("%s: runs %s: downloads go through internal/netgate", fset.Position(n.Pos()), cmd)
					}
				}
			}
			return true
		})
	})
}

var (
	// The page loads nothing from outside: no script, style, font, image or call to another site.
	reWebOut = regexp.MustCompile(`(?i)(?:src|href)\s*=\s*["']?(?:https?:)?//|url\(\s*["']?(?:https?:)?//|@import|XMLHttpRequest|WebSocket|EventSource|sendBeacon|fetch\(\s*["'` + "`" + `]https?:`)
	// The Mac app talks to its own server only.
	reSwiftURL  = regexp.MustCompile(`"(?:https?|wss?)://([^/":]+)`)
	reSwiftConn = regexp.MustCompile(`\b(?:NWConnection|NWPathMonitor|URLSessionWebSocketTask|CFStream|SCNetworkReachability)\b`)
)

// TestPageAndAppStayLocal: the page and the Mac app reach nothing but the local server.
func TestPageAndAppStayLocal(t *testing.T) {
	walk(t, []string{".html", ".css", ".js"}, func(rel string, data []byte) {
		if !strings.HasPrefix(rel, "web/") {
			return
		}
		for i, line := range strings.Split(string(data), "\n") {
			if reWebOut.MatchString(line) {
				t.Errorf("%s:%d reaches outside the Mac: %s", rel, i+1, strings.TrimSpace(line))
			}
		}
	})
	walk(t, []string{".swift"}, func(rel string, data []byte) {
		for i, line := range strings.Split(string(data), "\n") {
			for _, m := range reSwiftURL.FindAllStringSubmatch(line, -1) {
				if m[1] != "127.0.0.1" && m[1] != "localhost" {
					t.Errorf("%s:%d reaches %s: the app talks to its own server only", rel, i+1, m[1])
				}
			}
			if reSwiftConn.MatchString(line) {
				t.Errorf("%s:%d opens a connection of its own: %s", rel, i+1, strings.TrimSpace(line))
			}
		}
	})
}
