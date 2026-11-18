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
	"github.com/coddyum/kanso/internal/ledger"
	"github.com/coddyum/kanso/internal/logo"
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
}

// Server answers the web page and the Mac app.
type Server struct {
	root   folder.Root
	index  *folder.Index
	book   *ledger.Book
	rates  *rates.Source
	logos  *logo.Source
	static fs.FS
	port   int
	boot   string // when this process started: the page reloads itself when it changes (a new version)
	now    func() time.Time

	mu       sync.Mutex
	cached   *ledger.Model
	cacheKey string // folder signature | database version | day
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
	sum := sha256.Sum256([]byte(cfg.Root))
	cacheFile := filepath.Join(cfg.CacheDir, "index-"+hex.EncodeToString(sum[:4])+".json")
	return &Server{
		root:   folder.NewRoot(cfg.Root, vendors),
		index:  folder.NewIndex(cfg.Root, cacheFile, cfg.PDFText, vendors),
		book:   book,
		rates:  rates.NewSource(cfg.CacheDir),
		logos:  logo.NewSource(cfg.CacheDir),
		static: cfg.Static,
		port:   cfg.Port,
		boot:   time.Now().Format(time.RFC3339Nano),
		now:    time.Now,
	}, nil
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

// guard refuses foreign Host headers (DNS rebinding) and writes without the
// X-Inc header (a form posted by another website cannot set it).
func guard(port int, next http.Handler) http.Handler {
	allowed := map[string]bool{
		fmt.Sprintf("localhost:%d", port): true,
		fmt.Sprintf("127.0.0.1:%d", port): true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowed[r.Host] {
			http.Error(w, "hôte refusé", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Header.Get("X-Inc") != "1" {
			http.Error(w, "en-tête X-Inc manquant", http.StatusForbidden)
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
