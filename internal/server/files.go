package server

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/coddyum/kanso/internal/folder"
)

// failFolder answers a failed folder operation: 400 and the user's language
// for a refused path or name, code for what the disk refused.
func (s *Server) failFolder(w http.ResponseWriter, r *http.Request, err error, code int) {
	var refused folder.Refused
	if errors.As(err, &refused) {
		http.Error(w, s.tr(r).T(refused.Key, "path", refused.Path), http.StatusBadRequest)
		return
	}
	fail(w, code, err)
}

// handleList shows a folder (?dir=), each file with what the app read in it.
func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("dir")
	if _, err := s.root.Abs(rel); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	dir, entries, err := s.root.List(rel)
	if err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	m, err := s.model()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	m.Describe(entries)
	writeJSON(w, map[string]any{"dir": dir, "entries": entries})
}

// handleFile serves a file of the folder. Anything but a PDF, an image or a
// text runs sandboxed, without access to the app.
func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	abs, err := s.root.Abs(r.PathValue("path"))
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	switch strings.ToLower(filepath.Ext(abs)) {
	case ".pdf", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".txt":
	default:
		w.Header().Set("Content-Security-Policy", "sandbox")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, st.Name(), st.ModTime(), f)
}

type pathRequest struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	ToDir string `json:"toDir"`
}

func (s *Server) handleMkdir(w http.ResponseWriter, r *http.Request) {
	var req pathRequest
	if !readJSON(w, r, &req) {
		return
	}
	p, err := s.root.Mkdir(req.Path, req.Name)
	if err != nil {
		s.failFolder(w, r, err, http.StatusConflict)
		return
	}
	writeJSON(w, map[string]string{"path": p})
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	var req pathRequest
	if !readJSON(w, r, &req) {
		return
	}
	p, err := s.root.Rename(req.Path, req.Name)
	if err != nil {
		s.failFolder(w, r, err, http.StatusConflict)
		return
	}
	writeJSON(w, map[string]string{"path": p})
}

func (s *Server) handleMove(w http.ResponseWriter, r *http.Request) {
	var req pathRequest
	if !readJSON(w, r, &req) {
		return
	}
	p, err := s.root.Move(req.Path, req.ToDir)
	if err != nil {
		s.failFolder(w, r, err, http.StatusConflict)
		return
	}
	writeJSON(w, map[string]string{"path": p})
}

func (s *Server) handleReveal(w http.ResponseWriter, r *http.Request) { s.open(w, r, true) }
func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request)   { s.open(w, r, false) }

// open shows a file in the Finder, or opens it with its usual app.
func (s *Server) open(w http.ResponseWriter, r *http.Request, reveal bool) {
	var req pathRequest
	if !readJSON(w, r, &req) {
		return
	}
	if err := s.root.Open(req.Path, reveal); err != nil {
		s.failFolder(w, r, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, ok)
}

// handleUpload writes the dropped files in ?dir=, or for a due date in
// YEAR/Vendor/Month (?vendor=&date=), reusing the folders that already exist.
// With ?key= the first file is linked to that due date: it becomes its invoice.
// A file with an unusable name is skipped.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var dir string
	var err error
	if q.Get("vendor") != "" {
		dir, err = s.root.TargetDir(q.Get("vendor"), q.Get("date"), s.tr(r).List("folder.months"))
	} else {
		dir, err = s.root.Abs(q.Get("dir"))
	}
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		http.Error(w, s.tr(r).T("error.noTargetDir"), http.StatusBadRequest)
		return
	}
	mr, err := r.MultipartReader()
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	saved := []string{}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		if _, err := folder.CleanName(part.FileName()); part.FileName() == "" || err != nil {
			continue
		}
		p, err := s.root.Write(dir, part.FileName(), part)
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		saved = append(saved, p)
	}
	if key := q.Get("key"); key != "" && len(saved) > 0 {
		if hash, err := folder.HashFile(filepath.Join(s.root.Dir(), filepath.FromSlash(saved[0]))); err == nil {
			if err := s.book.Link(key, hash); err != nil {
				log.Printf("upload: link %s: %v", key, err)
			}
		}
	}
	writeJSON(w, map[string]any{"dir": s.root.Rel(dir), "saved": saved})
}
