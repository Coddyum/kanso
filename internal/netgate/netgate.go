// Package netgate is the only way out of the Mac. Every download of the app
// goes through Get, and Get lets out two kinds of request only: the ECB's
// daily rates, and the home page or icon of a brand's own site. A test of
// this package fails when any other code of the repository opens a connection.
package netgate

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coddyum/kanso/internal/brand"
)

// ECBHost serves the euro reference rates.
const ECBHost = "www.ecb.europa.eu"

// Purpose says why a request goes out, and so which hosts it may reach.
type Purpose struct {
	name   string
	allows func(host string) bool
}

// Rates is the download of the ECB's reference rates.
func Rates() Purpose {
	return Purpose{name: "rates", allows: func(host string) bool { return host == ECBHost }}
}

// Logo is the download of a brand's icon: its own domain and its subdomains
// only (« notion.so » lets out « www.notion.so », never a CDN or Google).
func Logo(domain string) Purpose {
	base := strings.TrimPrefix(strings.ToLower(domain), "www.")
	return Purpose{name: "logo " + base, allows: func(host string) bool {
		return base != "" && (host == base || strings.HasSuffix(host, "."+base))
	}}
}

// Allows says whether a request for p may reach rawURL: HTTPS, default port, an allowed host.
func (p Purpose) Allows(rawURL string) bool {
	u, err := url.Parse(rawURL)
	return err == nil && u.Scheme == "https" && u.Port() == "" && u.User == nil && p.allows(strings.ToLower(u.Hostname()))
}

// ErrRefused is returned for a request the gate does not let out.
var ErrRefused = errors.New("netgate: request refused")

// Gate sends the requests; its client is replaced only by tests.
type Gate struct {
	client *http.Client
}

// New is the gate the app uses: 6 seconds per request.
func New() *Gate { return &Gate{client: &http.Client{Timeout: 6 * time.Second}} }

// Get downloads rawURL for p, at most limit bytes; a redirect must be allowed
// for p too. Anything but a 200 is an error.
func (g *Gate) Get(p Purpose, rawURL string, limit int64) ([]byte, error) {
	if !p.Allows(rawURL) {
		return nil, fmt.Errorf("%w: %s for %s", ErrRefused, rawURL, p.name)
	}
	client := *g.client
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !p.Allows(req.URL.String()) {
			return fmt.Errorf("%w: redirect to %s for %s", ErrRefused, req.URL, p.name)
		}
		return nil
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh) "+brand.Name)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("netgate: %s: status %d", rawURL, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

// Local asks the app's own server on this Mac (127.0.0.1:port) for path:
// how `kanso init` and `kanso doctor` see whether it runs. It never leaves the Mac.
func (g *Gate) Local(port int, path string) ([]byte, error) {
	client := &http.Client{
		Timeout:       3 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/%s", port, strings.TrimPrefix(path, "/")))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("netgate: local %s: status %d", path, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}
