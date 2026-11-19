package server

import (
	"errors"
	"net/http"
	"time"

	"github.com/coddyum/kanso/internal/day"
	"github.com/coddyum/kanso/internal/ledger"
	"github.com/coddyum/kanso/internal/logo"
	"github.com/coddyum/kanso/internal/vendor"
)

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	m, err := s.model()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, struct {
		Root      string `json:"root"`
		Extractor bool   `json:"extractor"` // the PDF reader is there
		Demo      bool   `json:"demo"`      // the folder is the demo's: made-up data
		ledger.State
	}{s.root.Dir(), s.index.CanReadPDF(), s.demo, m.State(s.currentRates(), s.tr(r))})
}

// handleCalendar returns the events between ?from= and ?to=, at most 400 days apart.
func (s *Server) handleCalendar(w http.ResponseWriter, r *http.Request) {
	from, ok1 := day.Parse(r.URL.Query().Get("from"))
	to, ok2 := day.Parse(r.URL.Query().Get("to"))
	if !ok1 || !ok2 || to.Before(from) || to.Sub(from) > 400*24*time.Hour {
		fail(w, http.StatusBadRequest, errors.New("from and to must be days at most 400 days apart"))
		return
	}
	m, err := s.model()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, m.Calendar(from, to, s.tr(r)))
}

func (s *Server) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	m, err := s.model()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, m.Analytics(s.currentRates(), s.tr(r)))
}

// handleReminders lists the notifications the Mac app schedules; none for
// the demo's made-up payments.
func (s *Server) handleReminders(w http.ResponseWriter, r *http.Request) {
	if s.demo {
		writeJSON(w, []ledger.Reminder{})
		return
	}
	m, err := s.model()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, m.Reminders(s.currentRates(), s.now(), s.tr(r)))
}

// handleSig is polled by the page: a new folder signature means reload the
// data, a new boot means reload the page.
func (s *Server) handleSig(w http.ResponseWriter, r *http.Request) {
	sig, err := s.index.Signature()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]string{"sig": sig, "boot": s.boot})
}

// handleLogo serves the icon of ?d=, a domain; 404 when it has none.
func (s *Server) handleLogo(w http.ResponseWriter, r *http.Request) {
	data := s.logos.Get(vendor.CleanDomain(r.URL.Query().Get("d")))
	if len(data) == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", logo.ContentType(data))
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'")
	w.Header().Set("Cache-Control", "max-age=604800")
	_, _ = w.Write(data)
}
