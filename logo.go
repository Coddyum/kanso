package main

import (
	"bytes"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	reDomain  = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)
	reLinkTag = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	reAttr    = regexp.MustCompile(`(?is)\b(rel|href|sizes|type)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	logoMu    sync.Mutex
)

// handleLogo serves a vendor's icon, fetched once and kept in the cache; an
// empty cached file remembers « no icon » for a week.
func (a *App) handleLogo(w http.ResponseWriter, r *http.Request) {
	d := cleanDomain(r.URL.Query().Get("d"))
	if !reDomain.MatchString(d) {
		http.NotFound(w, r)
		return
	}
	file := filepath.Join(a.cacheDir, "logos", d)
	logoMu.Lock()
	data, err := os.ReadFile(file)
	st, _ := os.Stat(file)
	if err != nil || (len(data) == 0 && time.Since(st.ModTime()) > 7*24*time.Hour) {
		data = fetchLogo(d)
		_ = os.MkdirAll(filepath.Dir(file), 0o755)
		_ = os.WriteFile(file, data, 0o644)
	}
	logoMu.Unlock()
	if len(data) == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", imageType(data))
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'")
	w.Header().Set("Cache-Control", "max-age=604800")
	_, _ = w.Write(data)
}

func imageType(data []byte) string {
	if bytes.Contains(data[:min(len(data), 1024)], []byte("<svg")) {
		return "image/svg+xml"
	}
	return http.DetectContentType(data)
}

// fetchLogo tries the icons the home page declares (largest first), then
// /favicon.ico, then Google's icon service.
func fetchLogo(domain string) []byte {
	client := &http.Client{Timeout: 6 * time.Second}
	candidates := append(declaredIcons(client, domain),
		"https://"+domain+"/favicon.ico",
		"https://www.google.com/s2/favicons?domain="+domain+"&sz=128",
	)
	for _, u := range candidates {
		if body := get(client, u, 1<<20); len(body) > 60 && isImage(body) {
			return body
		}
	}
	return nil
}

func isImage(b []byte) bool {
	return strings.HasPrefix(imageType(b), "image/")
}

func get(client *http.Client, u string, limit int64) []byte {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh) coddyum-inc")
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, limit))
	return body
}

// declaredIcons reads the <link rel="…icon…"> tags of the home page.
func declaredIcons(client *http.Client, domain string) []string {
	base, _ := url.Parse("https://" + domain + "/")
	page := get(client, base.String(), 512<<10)
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
