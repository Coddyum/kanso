package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// safe turns a relative path of the folder into an absolute one, refusing
// anything that leaves the folder or names a hidden file.
func (a *App) safe(rel string) (string, error) {
	clean := path.Clean("/" + strings.ReplaceAll(rel, "\\", "/"))
	for _, seg := range strings.Split(clean, "/") {
		if strings.HasPrefix(seg, ".") {
			return "", fmt.Errorf("chemin refusé : %s", rel)
		}
	}
	abs := filepath.Join(a.root, filepath.FromSlash(clean))
	if abs != a.root && !strings.HasPrefix(abs, a.root+string(filepath.Separator)) {
		return "", fmt.Errorf("chemin refusé : %s", rel)
	}
	return abs, nil
}

func (a *App) rel(abs string) string {
	r, err := filepath.Rel(a.root, abs)
	if err != nil || r == "." {
		return ""
	}
	return filepath.ToSlash(r)
}

// cleanName keeps a file or folder name usable on the Mac.
func cleanName(name string) (string, error) {
	name = strings.TrimSpace(filepath.Base(strings.ReplaceAll(name, "\\", "/")))
	name = strings.TrimLeft(name, ".")
	name = strings.ReplaceAll(name, ":", "-")
	if name == "" || name == "/" || len(name) > 200 {
		return "", errors.New("nom invalide")
	}
	return name, nil
}

// Entry is one line of the folder view.
type Entry struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Dir       bool      `json:"dir"`
	Size      int64     `json:"size"`
	Mod       time.Time `json:"mod"`
	Items     int       `json:"items,omitempty"`
	Kind      string    `json:"kind,omitempty"`
	Hash      string    `json:"hash,omitempty"`
	InvoiceID string    `json:"invoiceId,omitempty"`
}

func (a *App) handleList(w http.ResponseWriter, r *http.Request) {
	dirRel := r.URL.Query().Get("dir")
	abs, err := a.safe(dirRel)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	des, err := os.ReadDir(abs)
	if err != nil {
		fail(w, http.StatusNotFound, fmt.Errorf("dossier introuvable : %s", dirRel))
		return
	}
	m, err := a.buildModel()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	docOf := map[string]Doc{}
	for _, d := range m.docs {
		docOf[d.Path] = d
	}
	invoiceOf := map[string]string{}
	for _, inv := range m.invoices {
		for _, f := range inv.Files {
			invoiceOf[f.Path] = inv.ID
		}
	}
	entries := []Entry{}
	for _, de := range des {
		if skipName(de.Name()) {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		e := Entry{Name: de.Name(), Path: a.rel(filepath.Join(abs, de.Name())), Dir: de.IsDir(), Size: info.Size(), Mod: info.ModTime()}
		if e.Dir {
			if sub, err := os.ReadDir(filepath.Join(abs, de.Name())); err == nil {
				for _, s := range sub {
					if !skipName(s.Name()) {
						e.Items++
					}
				}
			}
		} else {
			d := docOf[e.Path]
			e.Kind, e.Hash, e.InvoiceID = d.Info.Kind, d.Hash, invoiceOf[e.Path]
		}
		entries = append(entries, e)
	}
	sort.SliceStable(entries, func(i, j int) bool { return lessEntry(entries[i], entries[j]) })
	writeJSON(w, map[string]any{"dir": a.rel(abs), "entries": entries})
}

// lessEntry: folders first, years newest first, months in calendar order, then by name.
func lessEntry(x, y Entry) bool {
	if x.Dir != y.Dir {
		return x.Dir
	}
	xy, ex := strconv.Atoi(x.Name)
	yy, ey := strconv.Atoi(y.Name)
	if ex == nil && ey == nil {
		return xy > yy
	}
	mx, my := monthOf(x.Name), monthOf(y.Name)
	if mx > 0 && my > 0 && mx != my {
		return mx < my
	}
	return strings.ToLower(x.Name) < strings.ToLower(y.Name)
}

func (a *App) handleFile(w http.ResponseWriter, r *http.Request) {
	abs, err := a.safe(r.PathValue("path"))
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
		// A saved web page or an SVG runs without access to the app.
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

func (a *App) handleMkdir(w http.ResponseWriter, r *http.Request) {
	var req pathRequest
	if !readJSON(w, r, &req) {
		return
	}
	parent, err := a.safe(req.Path)
	name, err2 := cleanName(req.Name)
	if err = errors.Join(err, err2); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	target := filepath.Join(parent, name)
	if err := os.Mkdir(target, 0o755); err != nil {
		fail(w, http.StatusConflict, fmt.Errorf("création de « %s » impossible : %w", name, err))
		return
	}
	writeJSON(w, map[string]string{"path": a.rel(target)})
}

func (a *App) handleRename(w http.ResponseWriter, r *http.Request) {
	var req pathRequest
	if !readJSON(w, r, &req) {
		return
	}
	src, err := a.safe(req.Path)
	name, err2 := cleanName(req.Name)
	if err = errors.Join(err, err2); err != nil || src == a.root {
		fail(w, http.StatusBadRequest, errors.Join(err, errors.New("renommage refusé")))
		return
	}
	dst := filepath.Join(filepath.Dir(src), name)
	if err := moveNoClobber(src, dst); err != nil {
		fail(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, map[string]string{"path": a.rel(dst)})
}

func (a *App) handleMove(w http.ResponseWriter, r *http.Request) {
	var req pathRequest
	if !readJSON(w, r, &req) {
		return
	}
	src, err := a.safe(req.Path)
	dir, err2 := a.safe(req.ToDir)
	if err = errors.Join(err, err2); err != nil || src == a.root {
		fail(w, http.StatusBadRequest, errors.Join(err, errors.New("déplacement refusé")))
		return
	}
	if dir == src || strings.HasPrefix(dir, src+string(filepath.Separator)) {
		fail(w, http.StatusBadRequest, errors.New("un dossier ne peut pas aller dans lui-même"))
		return
	}
	dst := filepath.Join(dir, filepath.Base(src))
	if err := moveNoClobber(src, dst); err != nil {
		fail(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, map[string]string{"path": a.rel(dst)})
}

// moveNoClobber renames src to dst unless dst is another existing file
// (a change of case only is allowed on the Mac's case-insensitive disk).
func moveNoClobber(src, dst string) error {
	if src == dst {
		return nil
	}
	if dstInfo, err := os.Stat(dst); err == nil {
		srcInfo, err := os.Stat(src)
		if err != nil || !os.SameFile(srcInfo, dstInfo) {
			return fmt.Errorf("« %s » existe déjà", filepath.Base(dst))
		}
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("déplacement impossible : %w", err)
	}
	return nil
}

func (a *App) handleReveal(w http.ResponseWriter, r *http.Request) { a.runOpen(w, r, true) }
func (a *App) handleOpen(w http.ResponseWriter, r *http.Request)   { a.runOpen(w, r, false) }

// runOpen shows a file in the Finder, or opens it with its usual app.
func (a *App) runOpen(w http.ResponseWriter, r *http.Request, reveal bool) {
	var req pathRequest
	if !readJSON(w, r, &req) {
		return
	}
	abs, err := a.safe(req.Path)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	args := []string{abs}
	if reveal {
		args = []string{"-R", abs}
	}
	if err := exec.Command("open", args...).Run(); err != nil {
		fail(w, http.StatusInternalServerError, fmt.Errorf("open : %w", err))
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// handleUpload writes the dropped files in ?dir=, or for a due date in
// YEAR/Vendor/Month (?vendor=&date=), reusing the folders that already exist.
// With ?key= the first file is linked to that due date.
func (a *App) handleUpload(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var dirAbs string
	var err error
	if q.Get("vendor") != "" {
		dirAbs, err = a.targetDir(q.Get("vendor"), q.Get("date"))
	} else {
		dirAbs, err = a.safe(q.Get("dir"))
	}
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if st, err := os.Stat(dirAbs); err != nil || !st.IsDir() {
		fail(w, http.StatusBadRequest, errors.New("dossier de destination introuvable"))
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
		if part.FileName() == "" {
			continue
		}
		name, err := cleanName(part.FileName())
		if err != nil {
			continue
		}
		dst, err := writeUnique(dirAbs, name, part)
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		saved = append(saved, a.rel(dst))
	}
	if key := q.Get("key"); key != "" && len(saved) > 0 {
		if hash, err := hashFile(filepath.Join(a.root, filepath.FromSlash(saved[0]))); err == nil {
			_ = a.store.update(func(db *DB) error { db.Links[key] = hash; return nil })
		}
	}
	writeJSON(w, map[string]any{"dir": a.rel(dirAbs), "saved": saved})
}

// writeUnique creates name in dir, or « name (2).ext » if it is taken.
func writeUnique(dir, name string, src io.Reader) (string, error) {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; i < 1000; i++ {
		candidate := name
		if i > 1 {
			candidate = fmt.Sprintf("%s (%d)%s", base, i, ext)
		}
		p := filepath.Join(dir, candidate)
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("écriture de %s : %w", candidate, err)
		}
		_, err = io.Copy(f, io.LimitReader(src, 500<<20))
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(p)
			return "", fmt.Errorf("écriture de %s : %w", candidate, err)
		}
		return p, nil
	}
	return "", fmt.Errorf("trop de fichiers nommés %s", name)
}

// targetDir is YEAR/Vendor/Month for a due date, created when missing; existing
// folders are reused whatever their case or spelling (« aout », « OVH »…).
func (a *App) targetDir(vendorName, date string) (string, error) {
	t, ok := parseDay(date)
	if !ok {
		t = time.Now()
	}
	year := strconv.Itoa(t.Year())
	dir := filepath.Join(a.root, findChild(a.root, year, func(n string) bool { return n == year }))
	known, isKnown := knownIn(vendorName)
	vendorFolder, err := cleanName(vendorName)
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, findChild(dir, vendorFolder, func(n string) bool {
		if v, ok := knownIn(n); ok && isKnown {
			return v.Name == known.Name
		}
		return norm(n) == norm(vendorName)
	}))
	month := int(t.Month())
	dir = filepath.Join(dir, findChild(dir, monthFolders[month-1], func(n string) bool {
		return isTimeSegment(n) && monthOf(strings.Fields(words(n))[0]) == month
	}))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("création de %s : %w", dir, err)
	}
	return dir, nil
}

func findChild(dir, fallback string, match func(name string) bool) string {
	des, err := os.ReadDir(dir)
	if err != nil {
		return fallback
	}
	for _, de := range des {
		if de.IsDir() && !skipName(de.Name()) && match(de.Name()) {
			return de.Name()
		}
	}
	return fallback
}
