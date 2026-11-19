package server

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/coddyum/kanso/internal/extract"
	"github.com/coddyum/kanso/internal/intake"
)

// maxUploadFile is the largest file of a drop, as for a file dropped one by one.
const maxUploadFile = 500 << 20

// handleIntake receives a drop of folders: a multipart body where each file
// comes after a « path » field, its path in what was dropped. The files are
// copied aside, read and sorted; the answer is where each would go. Nothing
// is in the folder before handleIntakeCopy.
func (s *Server) handleIntake(w http.ResponseWriter, r *http.Request) {
	mr, err := r.MultipartReader()
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	path := ""
	b, err := s.desk.Receive(func() (intake.Incoming, error) {
		for {
			part, err := mr.NextPart()
			if err != nil {
				return intake.Incoming{}, err
			}
			if part.FormName() == "path" {
				data, _ := io.ReadAll(io.LimitReader(part, 4096))
				path = string(data)
				continue
			}
			name := path
			if name == "" {
				name = part.FileName()
			}
			path = ""
			return intake.Incoming{Path: name, Body: io.LimitReader(part, maxUploadFile)}, nil
		}
	})
	if errors.Is(err, intake.ErrTooMany) {
		http.Error(w, s.tr(r).T("error.intakeTooMany"), http.StatusBadRequest)
		return
	}
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	docs, err := s.index.Scan()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	known := make(map[string]string, len(docs))
	for _, d := range docs {
		known[d.Hash] = d.Path
	}
	err = s.desk.Plan(b, known, func(rel, text string) extract.Extracted { return extract.Extract(s.vendors, rel, text) })
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, b)
}

// handleIntakeCopy copies a drop's files into the folder, where its plan said.
func (s *Server) handleIntakeCopy(w http.ResponseWriter, r *http.Request) {
	b, err := s.desk.Copy(r.PathValue("id"))
	if errors.Is(err, intake.ErrUnknown) {
		http.Error(w, s.tr(r).T("error.intakeUnknown"), http.StatusBadRequest)
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	_ = s.book.SetOnboardingStep("invoices", "done")
	writeJSON(w, map[string]any{"id": b.ID, "written": len(b.Written)})
}

// handleIntakeUndo removes what a drop's copy wrote, except the files changed since.
func (s *Server) handleIntakeUndo(w http.ResponseWriter, r *http.Request) {
	removed, kept, err := s.desk.Undo(r.PathValue("id"))
	if errors.Is(err, intake.ErrUnknown) {
		http.Error(w, s.tr(r).T("error.intakeUnknown"), http.StatusBadRequest)
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]int{"removed": removed, "kept": kept})
}

// handleIntakeCancel forgets a drop that was not copied.
func (s *Server) handleIntakeCancel(w http.ResponseWriter, r *http.Request) {
	if err := s.desk.Cancel(r.PathValue("id")); err != nil {
		http.Error(w, s.tr(r).T("error.intakeUnknown"), http.StatusBadRequest)
		return
	}
	writeJSON(w, ok)
}

// lastIntake is the latest copy that can still be undone, for Settings and the checklist.
type lastIntake struct {
	ID      string `json:"id"`
	Written int    `json:"written"`
	Copied  string `json:"copied"`
}

func (s *Server) lastIntake() *lastIntake {
	b := s.desk.Last()
	if b == nil {
		return nil
	}
	return &lastIntake{ID: b.ID, Written: len(b.Written), Copied: b.Copied.Format(time.RFC3339)}
}
