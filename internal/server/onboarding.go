package server

import "net/http"

// handleOnboarding ticks a step of the first-run checklist (done, skipped,
// todo again), or opens the list again (show: true) or puts it away (false).
func (s *Server) handleOnboarding(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Step  string `json:"step"`
		State string `json:"state"`
		Show  *bool  `json:"show"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	var err error
	if req.Step != "" {
		err = s.book.SetOnboardingStep(req.Step, req.State)
	}
	if err == nil && req.Show != nil {
		err = s.book.ShowOnboarding(*req.Show)
	}
	if err != nil {
		s.failEdit(w, r, err)
		return
	}
	writeJSON(w, ok)
}
