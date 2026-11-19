package server

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/coddyum/kanso/internal/brand"
	"github.com/coddyum/kanso/internal/config"
	"github.com/coddyum/kanso/internal/deadlines"
	"github.com/coddyum/kanso/internal/i18n"
	"github.com/coddyum/kanso/internal/ledger"
	"github.com/coddyum/kanso/internal/rates"
	"github.com/coddyum/kanso/internal/store"
)

// AppStatus is what the Mac app reports about itself: what only it can know.
type AppStatus struct {
	Notifications string    `json:"notifications"` // authorized | denied | notDetermined | provisional
	LoginItem     bool      `json:"loginItem"`     // opens at login
	Version       string    `json:"version"`
	Seen          time.Time `json:"seen"` // when it last reported; zero: never (no Mac app)
}

// currentRates are the rates the screens use: downloaded when the user allows
// it, otherwise the last ones saved, never fetched.
func (s *Server) currentRates() rates.Rates {
	if s.book.Privacy().Rates {
		return s.rates.Get()
	}
	return s.rates.Cached()
}

func (s *Server) appStatus() AppStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.app
}

// handleAppStatus records what the Mac app reports: notifications allowed, opening at login.
func (s *Server) handleAppStatus(w http.ResponseWriter, r *http.Request) {
	var st AppStatus
	if !readJSON(w, r, &st) {
		return
	}
	st.Seen = s.now()
	s.mu.Lock()
	s.app = st
	s.mu.Unlock()
	writeJSON(w, ok)
}

// settings is everything the Settings screen shows.
type settings struct {
	Lang    string             `json:"lang"`
	Chosen  string             `json:"chosen"` // "" follows the Mac
	Langs   []string           `json:"langs"`
	Root    string             `json:"root"`
	Port    int                `json:"port"`
	Admin   deadlines.Settings `json:"admin"`
	Notify  ledger.Notify      `json:"notify"`
	Privacy ledger.Privacy     `json:"privacy"`
	App     AppStatus          `json:"app"`
	Backups []store.Backup     `json:"backups"`
	Intake  *lastIntake        `json:"intake"` // the latest copy of a drop, which can be undone
	Version string             `json:"version"`
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	db := s.book.Snapshot()
	backups, err := s.book.Backups()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, settings{
		Lang: s.lang(r), Chosen: db.Lang, Langs: i18n.Langs, Root: s.root.Dir(), Port: s.port,
		Admin: db.Settings(), Notify: db.NotifySettings(), Privacy: db.PrivacySettings(),
		App: s.appStatus(), Backups: backups, Version: s.version, Intake: s.lastIntake(),
	})
}

// handleSaveSettings saves the reminders or the downloads allowed, whichever the request carries.
func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Notify  *ledger.Notify  `json:"notify"`
		Privacy *ledger.Privacy `json:"privacy"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.Notify != nil {
		if err := s.book.SetNotify(*req.Notify); err != nil {
			s.failEdit(w, r, err)
			return
		}
	}
	if req.Privacy != nil {
		if err := s.book.SetPrivacy(*req.Privacy); err != nil {
			s.failEdit(w, r, err)
			return
		}
	}
	writeJSON(w, ok)
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if err := s.book.Restore(req.Name); err != nil {
		s.failEdit(w, r, err)
		return
	}
	writeJSON(w, ok)
}

// handleExport writes the whole database (JSON) and every expense (CSV, in
// euros too) in .kanso/exports of the folder, then shows them in the Finder.
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	m, err := s.model()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	dir := filepath.Join(s.root.Dir(), brand.DataDir, "exports")
	stamp := s.now().Format("2006-01-02-150405")
	jsonFile := filepath.Join(dir, brand.ID+"-"+stamp+".json")
	csvFile := filepath.Join(dir, brand.ID+"-"+stamp+".csv")
	if err := writeExport(jsonFile, csvFile, s.book.Snapshot(), m.Analytics(s.currentRates(), s.tr(r)).Spend, s.tr(r)); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	_ = exec.Command("open", "-R", csvFile).Start()
	writeJSON(w, map[string]string{"json": jsonFile, "csv": csvFile})
}

func writeExport(jsonFile, csvFile string, db ledger.DB, spend []ledger.Spend, tr i18n.Lang) error {
	if err := os.MkdirAll(filepath.Dir(jsonFile), 0o755); err != nil {
		return fmt.Errorf("export: %w", err)
	}
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(jsonFile, data, 0o644); err != nil {
		return fmt.Errorf("export: %w", err)
	}
	f, err := os.Create(csvFile)
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}
	cw := csv.NewWriter(f)
	head := []string{"date", "service", "label", "kind", "amount", "currency", "eur", "project"}
	for i, k := range head {
		head[i] = tr.T("export." + k)
	}
	_ = cw.Write(head)
	projects := map[string]string{}
	for _, p := range db.Projects {
		projects[p.ID] = p.Name
	}
	money := func(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }
	for _, sp := range spend {
		row := []string{sp.Date, sp.Service, sp.Label, tr.T("amountKind." + sp.Kind), money(sp.Amount), sp.Currency, money(sp.EUR), projects[sp.Project]}
		for i, cell := range row {
			// A cell a spreadsheet would read as a formula is written as text.
			if cell != "" && strings.ContainsRune("=+-@", rune(cell[0])) && i != 4 && i != 6 {
				row[i] = "'" + cell
			}
		}
		_ = cw.Write(row)
	}
	cw.Flush()
	if err := errors.Join(cw.Error(), f.Close()); err != nil {
		return fmt.Errorf("export: %w", err)
	}
	return nil
}

// diagnostic is what the Settings screen and `kanso doctor` show to find what is wrong.
type diagnostic struct {
	Version    string    `json:"version"`
	Root       string    `json:"root"`
	FolderOK   bool      `json:"folderOK"` // the server can read the folder (macOS asked for the access)
	Invoices   int       `json:"invoices"`
	Files      int       `json:"files"`
	PDFReader  bool      `json:"pdfReader"`
	RatesDate  string    `json:"ratesDate"`
	Rates      bool      `json:"rates"`
	Logos      bool      `json:"logos"`
	App        AppStatus `json:"app"`
	Started    string    `json:"started"`
	Reminders  int       `json:"reminders"`
	Lang       string    `json:"lang"`
	ConfigFile string    `json:"configFile"`
}

func (s *Server) handleDiagnostic(w http.ResponseWriter, r *http.Request) {
	d := diagnostic{
		Version: s.version, Root: s.root.Dir(), PDFReader: s.index.CanReadPDF(),
		Rates: s.book.Privacy().Rates, Logos: s.book.Privacy().Logos, App: s.appStatus(),
		Started: s.boot, Lang: s.lang(r), ConfigFile: config.Path(s.home),
	}
	_, err := os.ReadDir(s.root.Dir())
	d.FolderOK = err == nil
	d.RatesDate = s.rates.Cached().Date
	if m, err := s.model(); err == nil {
		d.Invoices, d.Files = len(m.Invoices), len(m.Docs)
		d.Reminders = len(m.Reminders(s.currentRates(), s.now(), s.tr(r)))
	}
	writeJSON(w, d)
}

// handleFolder points the app at another folder, or moves the folder:
//   - move: the folder goes into req.Path (a folder), on the same disk only;
//   - otherwise req.Path is an existing folder the app uses from now on.
//
// The setting is saved, then the server restarts on the new folder (the
// launcher starts it again); the page reloads itself.
func (s *Server) handleFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
		Move bool   `json:"move"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	target, err := s.newRoot(strings.TrimSpace(req.Path), req.Move)
	if err != nil {
		s.failEdit(w, r, err)
		return
	}
	if err := config.Save(s.home, config.Config{Root: target, Port: s.port}); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	log.Printf("folder: %s → %s, restarting", s.root.Dir(), target)
	writeJSON(w, map[string]string{"root": target})
	if s.restart != nil {
		go func() {
			time.Sleep(300 * time.Millisecond)
			s.restart()
		}()
	}
}

// newRoot checks the folder asked for, and moves the current one there when asked to.
func (s *Server) newRoot(path string, move bool) (string, error) {
	if strings.HasPrefix(path, "~/") {
		path = filepath.Join(s.home, path[2:])
	}
	if !filepath.IsAbs(path) {
		return "", ledger.Invalid("error.folderAbsolute")
	}
	path = filepath.Clean(path)
	if st, err := os.Stat(path); err != nil || !st.IsDir() {
		return "", ledger.Invalid("error.folderMissing")
	}
	if !move {
		if err := os.MkdirAll(filepath.Join(path, brand.DataDir), 0o755); err != nil {
			return "", ledger.Invalid("error.folderNotWritable")
		}
		return path, nil
	}
	current := s.root.Dir()
	target := filepath.Join(path, filepath.Base(current))
	if path == current || strings.HasPrefix(path, current+string(filepath.Separator)) {
		return "", ledger.Invalid("error.intoItself")
	}
	if _, err := os.Lstat(target); err == nil {
		return "", ledger.Invalid("error.folderTaken")
	}
	if err := os.Rename(current, target); err != nil {
		if errors.Is(err, syscall.EXDEV) {
			return "", ledger.Invalid("error.otherDisk")
		}
		return "", fmt.Errorf("move %s: %w", current, err)
	}
	return target, nil
}
