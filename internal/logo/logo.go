// Package logo fetches the icon of a vendor's site once and keeps it in a cache.
package logo

import (
	"bytes"
	"html"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coddyum/kanso/internal/netgate"
)

var (
	reDomain  = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)
	reLinkTag = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	reAttr    = regexp.MustCompile(`(?is)\b(rel|href|sizes|type)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
)

// Source serves the icons kept in its cache folder, fetching the missing ones.
type Source struct {
	dir   string
	gate  *netgate.Gate
	allow func(domain string) bool // a domain the user's data names: no other is ever fetched
	locks sync.Map                 // domain → *sync.Mutex: one fetch per domain at a time, the others go on
}

// NewSource keeps its icons in cacheDir/logos and fetches, through gate, only the domains allow accepts.
func NewSource(cacheDir string, gate *netgate.Gate, allow func(domain string) bool) *Source {
	return &Source{dir: filepath.Join(cacheDir, "logos"), gate: gate, allow: allow}
}

// ValidDomain says whether d is a domain name worth fetching an icon for.
func ValidDomain(d string) bool { return reDomain.MatchString(d) }

// Get returns the icon of domain, or nil. An empty cached file remembers
// « no icon » for a week before the next try; a domain allow refuses is never fetched.
func (s *Source) Get(domain string) []byte {
	if !ValidDomain(domain) {
		return nil
	}
	l, _ := s.locks.LoadOrStore(domain, &sync.Mutex{})
	mu := l.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	file := filepath.Join(s.dir, domain)
	data, err := os.ReadFile(file)
	if err == nil && len(data) > 0 {
		return data
	}
	if st, err2 := os.Stat(file); err == nil && err2 == nil && time.Since(st.ModTime()) < 7*24*time.Hour {
		return nil
	}
	if !s.allow(domain) {
		return nil
	}
	data = s.fetch(domain)
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		log.Printf("logo: cache dir: %v", err)
		return data
	}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		log.Printf("logo: write %s: %v", domain, err)
	}
	return data
}

// ContentType names an icon's format; SVG is told apart from the text it looks like.
func ContentType(data []byte) string {
	if bytes.Contains(data[:min(len(data), 1024)], []byte("<svg")) {
		return "image/svg+xml"
	}
	return http.DetectContentType(data)
}

// fetch tries the icons the home page declares (largest first), then
// /favicon.ico: the brand's own site only, never a third-party icon service.
func (s *Source) fetch(domain string) []byte {
	p := netgate.Logo(domain)
	candidates := append(s.declaredIcons(p, domain), "https://"+domain+"/favicon.ico")
	for _, u := range candidates {
		if body, err := s.gate.Get(p, u, 1<<20); err == nil && len(body) > 60 && strings.HasPrefix(ContentType(body), "image/") {
			return body
		}
	}
	return nil
}

// declaredIcons reads the <link rel="…icon…"> tags of the home page, the
// best first: Apple touch icon, SVG, a sized icon, then the rest.
func (s *Source) declaredIcons(p netgate.Purpose, domain string) []string {
	base, _ := url.Parse("https://" + domain + "/")
	page, _ := s.gate.Get(p, base.String(), 512<<10)
	type icon struct {
		href  string
		score int
	}
	var icons []icon
	for _, tag := range reLinkTag.FindAllString(string(page), -1) {
		attrs := map[string]string{}
		for _, m := range reAttr.FindAllStringSubmatch(tag, -1) {
			attrs[strings.ToLower(m[1])] = html.UnescapeString(m[2] + m[3] + m[4])
		}
		rel := strings.ToLower(attrs["rel"])
		if !strings.Contains(rel, "icon") || strings.Contains(rel, "mask") || attrs["href"] == "" {
			continue
		}
		ref, err := url.Parse(attrs["href"])
		if err != nil {
			continue
		}
		score := 1
		switch {
		case strings.Contains(rel, "apple-touch"):
			score = 4
		case strings.Contains(attrs["type"], "svg") || strings.HasSuffix(ref.Path, ".svg"):
			score = 3
		case attrs["sizes"] != "" && attrs["sizes"] != "16x16" && attrs["sizes"] != "32x32":
			score = 2
		}
		icons = append(icons, icon{href: base.ResolveReference(ref).String(), score: score})
	}
	sort.SliceStable(icons, func(i, j int) bool { return icons[i].score > icons[j].score })
	out := make([]string, len(icons))
	for i, ic := range icons {
		out[i] = ic.href
	}
	return out
}
