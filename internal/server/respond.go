package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/coddyum/kanso/internal/ledger"
)

var ok = map[string]bool{"ok": true}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, err error) {
	http.Error(w, err.Error(), code)
}

// failEdit answers a failed edit: 400 and the user's language when the user
// can fix it, 500 when the disk failed.
func (s *Server) failEdit(w http.ResponseWriter, r *http.Request, err error) {
	var invalid ledger.Invalid
	if errors.As(err, &invalid) {
		http.Error(w, s.tr(r).T(string(invalid)), http.StatusBadRequest)
		return
	}
	fail(w, http.StatusInternalServerError, err)
}

// readJSON decodes a body of at most 1 MB into v, or answers 400.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		fail(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return false
	}
	return true
}
