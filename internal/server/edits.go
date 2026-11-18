package server

import (
	"net/http"

	"github.com/coddyum/kanso/internal/deadlines"
	"github.com/coddyum/kanso/internal/i18n"
	"github.com/coddyum/kanso/internal/ledger"
)

func (s *Server) handleSavePayment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ledger.Payment
		AmountChanged bool `json:"amountChanged"` // the owner typed a new amount: it wins until the next invoice
	}
	if !readJSON(w, r, &req) {
		return
	}
	p, err := s.book.SavePayment(req.Payment, req.AmountChanged)
	if err != nil {
		s.failEdit(w, r, err)
		return
	}
	writeJSON(w, p)
}

func (s *Server) handleDeletePayment(w http.ResponseWriter, r *http.Request) {
	if err := s.book.DeletePayment(r.PathValue("id")); err != nil {
		s.failEdit(w, r, err)
		return
	}
	writeJSON(w, ok)
}

func (s *Server) handleSaveTodo(w http.ResponseWriter, r *http.Request) {
	var t ledger.Todo
	if !readJSON(w, r, &t) {
		return
	}
	t, err := s.book.SaveTodo(t)
	if err != nil {
		s.failEdit(w, r, err)
		return
	}
	writeJSON(w, t)
}

func (s *Server) handleDeleteTodo(w http.ResponseWriter, r *http.Request) {
	if err := s.book.DeleteTodo(r.PathValue("id")); err != nil {
		s.failEdit(w, r, err)
		return
	}
	writeJSON(w, ok)
}

// handleDismiss marks a due date as « no invoice », or hides a suggestion.
func (s *Server) handleDismiss(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key string `json:"key"`
		On  bool   `json:"on"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if err := s.book.Dismiss(req.Key, req.On); err != nil {
		s.failEdit(w, r, err)
		return
	}
	writeJSON(w, ok)
}

// handleOverride saves the corrections of an invoice; a null override resets it.
func (s *Server) handleOverride(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       string           `json:"id"`
		Override *ledger.Override `json:"override"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if err := s.book.SetOverride(req.ID, req.Override); err != nil {
		s.failEdit(w, r, err)
		return
	}
	writeJSON(w, ok)
}

// handleReserve saves the balance of the account; a null amount removes it.
func (s *Server) handleReserve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Amount *float64 `json:"amount"`
		Date   string   `json:"date"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if err := s.book.SetReserve(req.Amount, req.Date); err != nil {
		s.failEdit(w, r, err)
		return
	}
	writeJSON(w, ok)
}

func (s *Server) handleSaveProject(w http.ResponseWriter, r *http.Request) {
	var p ledger.Project
	if !readJSON(w, r, &p) {
		return
	}
	p, err := s.book.SaveProject(p)
	if err != nil {
		s.failEdit(w, r, err)
		return
	}
	writeJSON(w, p)
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	if err := s.book.DeleteProject(r.PathValue("id")); err != nil {
		s.failEdit(w, r, err)
		return
	}
	writeJSON(w, ok)
}

// handleAssign files a service or one invoice in a project.
func (s *Server) handleAssign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind    string `json:"kind"` // service | invoice
		Ref     string `json:"ref"`
		Project string `json:"project"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if err := s.book.Assign(req.Kind, req.Ref, req.Project); err != nil {
		s.failEdit(w, r, err)
		return
	}
	writeJSON(w, ok)
}

// handleDeadline ticks an administrative due date as done, or unticks it.
func (s *Server) handleDeadline(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key  string `json:"key"`
		Done bool   `json:"done"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if err := s.book.SetDone(req.Key, req.Done); err != nil {
		s.failEdit(w, r, err)
		return
	}
	writeJSON(w, ok)
}

// handleAdmin saves which administrative due dates apply.
func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	var req deadlines.Settings
	if !readJSON(w, r, &req) {
		return
	}
	if err := s.book.SetSettings(req); err != nil {
		s.failEdit(w, r, err)
		return
	}
	writeJSON(w, req)
}

// handleLocale says which language the page speaks (the request's, without
// the page's own X-Lang) and which one the user chose ("" follows the Mac).
func (s *Server) handleLocale(w http.ResponseWriter, r *http.Request) {
	r.Header.Del("X-Lang")
	writeJSON(w, map[string]any{"lang": s.lang(r), "chosen": s.book.Lang(), "langs": i18n.Langs})
}

// handleSetLocale chooses the app's language; "" follows the Mac again.
func (s *Server) handleSetLocale(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Lang string `json:"lang"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if err := s.book.SetLang(req.Lang); err != nil {
		s.failEdit(w, r, err)
		return
	}
	writeJSON(w, ok)
}
