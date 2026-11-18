// Package rates converts amounts into euros with the reference rates of the
// European Central Bank, a public file fetched at most twice a day.
package rates

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/coddyum/kanso/internal/netgate"
)

const ecbURL = "https://" + netgate.ECBHost + "/stats/eurofxref/eurofxref-daily.xml"

// Rates are the euro reference rates: 1 EUR = PerEUR[currency].
type Rates struct {
	Date    string             `json:"date"`
	PerEUR  map[string]float64 `json:"perEUR"`
	Fetched time.Time          `json:"fetched"`
}

// ToEUR converts an amount; false when the rate of its currency is unknown.
func (r Rates) ToEUR(amount float64, currency string) (float64, bool) {
	if currency == "" || currency == "EUR" {
		return amount, true
	}
	if rate := r.PerEUR[currency]; rate > 0 {
		return amount / rate, true
	}
	return 0, false
}

var (
	reECBTime = regexp.MustCompile(`time='(\d{4}-\d{2}-\d{2})'`)
	reECBRate = regexp.MustCompile(`currency='([A-Z]{3})' rate='([\d.]+)'`)
)

// Source keeps the rates in memory and in a cache file: the last ones saved serve offline.
type Source struct {
	file string
	gate *netgate.Gate

	mu       sync.Mutex
	mem      Rates
	loaded   bool
	tried    time.Time // last fetch attempt: one every 30 minutes at most when offline
	fetching bool
}

// NewSource keeps its cache in cacheDir and downloads through gate.
func NewSource(cacheDir string, gate *netgate.Gate) *Source {
	return &Source{file: filepath.Join(cacheDir, "rates.json"), gate: gate}
}

// Cached returns the rates saved last, never downloading: what the app shows
// when the user turned the download of rates off.
func (s *Source) Cached() Rates {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loaded {
		s.loaded = true
		if data, err := os.ReadFile(s.file); err == nil {
			_ = json.Unmarshal(data, &s.mem)
		}
	}
	return s.mem
}

// Get returns the rates. Rates older than 12 hours are refreshed in the
// background, so a slow network never slows a page; only the very first
// call, with nothing cached yet, waits for the download.
func (s *Source) Get() Rates {
	s.mu.Lock()
	if !s.loaded {
		s.loaded = true
		if data, err := os.ReadFile(s.file); err == nil {
			_ = json.Unmarshal(data, &s.mem)
		}
	}
	stale := time.Since(s.mem.Fetched) >= 12*time.Hour && time.Since(s.tried) >= 30*time.Minute && !s.fetching
	if !stale {
		defer s.mu.Unlock()
		return s.mem
	}
	s.tried, s.fetching = time.Now(), true
	first := s.mem.PerEUR == nil
	s.mu.Unlock()
	if first {
		s.refresh()
	} else {
		go s.refresh()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mem
}

// refresh downloads the rates and saves them; a failure keeps the old ones.
func (s *Source) refresh() {
	fresh, ok := s.fetch()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fetching = false
	if !ok {
		return
	}
	s.mem = fresh
	data, err := json.Marshal(fresh)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.file), 0o755); err != nil {
		log.Printf("rates: cache dir: %v", err)
		return
	}
	if err := os.WriteFile(s.file, data, 0o644); err != nil {
		log.Printf("rates: write cache: %v", err)
	}
}

func (s *Source) fetch() (Rates, bool) {
	body, err := s.gate.Get(netgate.Rates(), ecbURL, 64<<10)
	if err != nil {
		return Rates{}, false
	}
	return parseECB(body)
}

// parseECB reads the ECB's daily XML; false when it holds no rate.
func parseECB(body []byte) (Rates, bool) {
	r := Rates{PerEUR: map[string]float64{}, Fetched: time.Now()}
	if m := reECBTime.FindSubmatch(body); m != nil {
		r.Date = string(m[1])
	}
	for _, m := range reECBRate.FindAllSubmatch(body, -1) {
		if v, err := strconv.ParseFloat(string(m[2]), 64); err == nil && v > 0 {
			r.PerEUR[string(m[1])] = v
		}
	}
	return r, len(r.PerEUR) > 0
}
