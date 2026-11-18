// Package server is the HTTP side of the app: it reads requests, calls the
// ledger and the folder, and writes JSON. It listens on 127.0.0.1 only.
package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/coddyum/kanso/internal/brand"
	"github.com/coddyum/kanso/internal/day"
	"github.com/coddyum/kanso/internal/folder"
	"github.com/coddyum/kanso/internal/i18n"
	"github.com/coddyum/kanso/internal/ledger"
	"github.com/coddyum/kanso/internal/logo"
	"github.com/coddyum/kanso/internal/netgate"
	"github.com/coddyum/kanso/internal/rates"
	"github.com/coddyum/kanso/internal/vendor"
)

// Config is what the server needs to start.
type Config struct {
	Root     string // the synced folder, absolute
	CacheDir string // what can be rebuilt: PDF texts, rates, logos
	PDFText  string // the PDFKit helper; "" to list PDFs without reading them
	Port     int    // the only Host the server answers: localhost or 127.0.0.1 on this port
	Static   fs.FS  // the web page
	Home     string // the user's home: where the config file is (a change of folder is saved there)
	Version  string // the build, shown in the diagnostic
	Restart  func() // stops the server so its launcher starts it again; nil when nothing restarts it
}

// Server answers the web page and the Mac app.
type Server struct {
	root    folder.Root
	index   *folder.Index
	book    *ledger.Book
	rates   *rates.Source
	logos   *logo.Source
	texts   *i18n.Catalog
	static  fs.FS
	port    int
	boot    string // when this process started: the page reloads itself when it changes (a new version)
	now     func() time.Time
	home    string
	version string
	restart func()

	mu       sync.Mutex
	cached   *ledger.Model
	cacheKey string    // folder signature | database version | day
	app      AppStatus // what the Mac app last reported
}

// New opens the database and the user's vendors of the folder.
func New(cfg Config) (*Server, error) {
	data := filepath.Join(cfg.Root, brand.DataDir)
	vendors := vendor.NewCatalog()
	if err := vendors.LoadLocal(filepath.Join(data, "vendors.json")); err != nil {
		return nil, err
	}
	book, err := ledger.Open(filepath.Join(data, "db.json"), vendors)
	if err != nil {
		return nil, err
	}
	texts, err := i18n.Load(cfg.Static)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(cfg.Root))
	cacheFile := filepath.Join(cfg.CacheDir, "index-"+hex.EncodeToString(sum[:4])+".json")
	gate := netgate.New()
	s := &Server{
		root:   folder.NewRoot(cfg.Root, vendors),
		index:  folder.NewIndex(cfg.Root, cacheFile, cfg.PDFText, vendors),
		book:   book,
		texts:  texts,
		rates:  rates.NewSource(cfg.CacheDir, gate),
		static: cfg.Static,
		port:   cfg.Port,
		boot:   time.Now().Format(time.RFC3339Nano),
		now:    time.Now,
		home:   cfg.Home, version: cfg.Version, restart: cfg.Restart,
	}
	s.logos = logo.NewSource(cfg.CacheDir, gate, s.knownDomain)
	return s, nil
}

// lang is the language of a request: the one the page sends (X-Lang), the one
// chosen in the app, the Mac's (Accept-Language), English when none is spoken.
func (s *Server) lang(r *http.Request) string {
	if code := r.Header.Get("X-Lang"); i18n.Known(code) {
		return code
	}
	if code := s.book.Lang(); code != "" {
		return code
	}
	if code := i18n.Match(r.Header.Get("Accept-Language")); code != "" {
		return code
	}
	return "en"
}

// tr is the language of a request, for the texts the server writes.
func (s *Server) tr(r *http.Request) i18n.Lang { return s.texts.Lang(s.lang(r)) }

// knownDomain says whether the user's data names domain: a payment, an
// invoice, a suggestion or a billing page. The icon of no other site is
// fetched, and none when the user turned the download of icons off.
func (s *Server) knownDomain(domain string) bool {
	if !s.book.Privacy().Logos {
		return false
	}
	m, err := s.model()
	return err == nil && m.Domains()[domain]
}

// Handler routes every request, behind the guard.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", s.handleState)
	mux.HandleFunc("GET /api/calendar", s.handleCalendar)
	mux.HandleFunc("GET /api/analytics", s.handleAnalytics)
	mux.HandleFunc("GET /api/reminders", s.handleReminders)
	mux.HandleFunc("GET /api/sig", s.handleSig)
	mux.HandleFunc("GET /api/logo", s.handleLogo)

	mux.HandleFunc("GET /api/ls", s.handleList)
	mux.HandleFunc("GET /files/{path...}", s.handleFile)
	mux.HandleFunc("POST /api/mkdir", s.handleMkdir)
	mux.HandleFunc("POST /api/upload", s.handleUpload)
	mux.HandleFunc("POST /api/rename", s.handleRename)
	mux.HandleFunc("POST /api/move", s.handleMove)
	mux.HandleFunc("POST /api/reveal", s.handleReveal)
	mux.HandleFunc("POST /api/open", s.handleOpen)

	mux.HandleFunc("POST /api/payments", s.handleSavePayment)
	mux.HandleFunc("DELETE /api/payments/{id}", s.handleDeletePayment)
	mux.HandleFunc("POST /api/todos", s.handleSaveTodo)
	mux.HandleFunc("DELETE /api/todos/{id}", s.handleDeleteTodo)
	mux.HandleFunc("POST /api/dismiss", s.handleDismiss)
	mux.HandleFunc("POST /api/override", s.handleOverride)
	mux.HandleFunc("POST /api/reserve", s.handleReserve)
	mux.HandleFunc("POST /api/projects", s.handleSaveProject)
	mux.HandleFunc("DELETE /api/projects/{id}", s.handleDeleteProject)
	mux.HandleFunc("POST /api/assign", s.handleAssign)
	mux.HandleFunc("POST /api/deadline", s.handleDeadline)
	mux.HandleFunc("POST /api/admin", s.handleAdmin)
	mux.HandleFunc("GET /api/settings", s.handleSettings)
	mux.HandleFunc("POST /api/settings", s.handleSaveSettings)
	mux.HandleFunc("POST /api/app-status", s.handleAppStatus)
	mux.HandleFunc("GET /api/diagnostic", s.handleDiagnostic)
	mux.HandleFunc("POST /api/backups/restore", s.handleRestore)
	mux.HandleFunc("POST /api/export", s.handleExport)
	mux.HandleFunc("POST /api/folder", s.handleFolder)
	mux.HandleFunc("GET /api/locale", s.handleLocale)
	mux.HandleFunc("POST /api/locale", s.handleSetLocale)

	mux.Handle("GET /", noCache(http.FileServerFS(s.static)))
	return guard(s.port, mux)
}

// model returns the model of the folder and the database, rebuilt only when
// one of them changed or the day turned: switching screens costs nothing.
func (s *Server) model() (*ledger.Model, error) {
	sig, err := s.index.Signature()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	today := day.Of(s.now())
	key := func() string { return fmt.Sprintf("%s|%d|%s", sig, s.book.Version(), today.Format(day.Format)) }
	if s.cached != nil && s.cacheKey == key() {
		return s.cached.Again(), nil
	}
	docs, err := s.index.Scan()
	if err != nil {
		return nil, err
	}
	m, err := s.book.Model(docs, today)
	if err != nil {
		return nil, err
	}
	s.cached, s.cacheKey = m, key()
	return m, nil
}

// guard refuses foreign Host headers (DNS rebinding), anything another website
// asks the browser to load (Sec-Fetch-Site: an <img> of a page could make the
// server fetch an icon), and writes without the X-Inc header (a form posted by
// another website cannot set it). The Mac app sends no Sec-Fetch-Site.
func guard(port int, next http.Handler) http.Handler {
	allowed := map[string]bool{
		fmt.Sprintf("localhost:%d", port): true,
		fmt.Sprintf("127.0.0.1:%d", port): true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowed[r.Host] {
			http.Error(w, "host refused", http.StatusForbidden)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			http.Error(w, "request from another website refused", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Header.Get("X-Inc") != "1" {
			http.Error(w, "X-Inc header missing", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}
