package server

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/coddyum/kanso/internal/config"
)

// handleDemo starts or stops the demo mode: the setting is saved, then the
// server restarts on the demo folder (made-up data, rebuilt from today) or
// back on the user's folder, which the demo never touches.
func (s *Server) handleDemo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		On bool `json:"on"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	cfg, err := config.Load(s.home)
	if err == nil && cfg.Root == "" {
		err = errors.New("demo: no folder in the settings: run kanso init")
	}
	if err == nil {
		cfg.Demo = req.On
		err = config.Save(s.home, cfg)
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	log.Printf("demo: %v, restarting", req.On)
	writeJSON(w, ok)
	if s.restart != nil {
		go func() {
			time.Sleep(300 * time.Millisecond)
			s.restart()
		}()
	}
}
