// Package intake brings folders of invoices kept elsewhere on the Mac into
// the user's folder. A drop is first copied aside (in .kanso/intake, never in
// sight), read and sorted: the user sees where each file would go before
// anything lands in the folder. Then the files are copied in — never moved
// from where they came, never over another file — and that copy can be undone.
package intake

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/coddyum/kanso/internal/brand"
	"github.com/coddyum/kanso/internal/extract"
	"github.com/coddyum/kanso/internal/folder"
)

// maxFiles is the most files one drop may hold.
const maxFiles = 5000

// ErrUnknown: no drop has this id, or it was already copied or cancelled.
var ErrUnknown = errors.New("intake: unknown drop")

// ErrTooMany: the drop holds more than maxFiles files.
var ErrTooMany = errors.New("intake: too many files")

// Item is one file of a drop and where it goes.
type Item struct {
	Path      string   `json:"path"` // its path in what was dropped: « Factures 2024/OVH/f-03.pdf »
	Size      int64    `json:"size"`
	Hash      string   `json:"hash"`
	Kind      string   `json:"kind"`
	Vendor    string   `json:"vendor,omitempty"`
	Date      string   `json:"date,omitempty"`
	Amount    *float64 `json:"amount,omitempty"`
	Currency  string   `json:"currency,omitempty"`
	Target    string   `json:"target"`              // where it goes in the folder; "" for a duplicate
	Sorted    bool     `json:"sorted"`              // filed by year, vendor and month; otherwise kept in its own folders
	Duplicate string   `json:"duplicate,omitempty"` // the file with the same content: in the folder, or earlier in the drop
	Staged    string   `json:"staged"`              // its copy aside, in the drop's folder
}

// Batch is one drop: its files, and once copied what was written, for the undo.
type Batch struct {
	ID      string    `json:"id"`
	Created time.Time `json:"created"`
	Items   []Item    `json:"items"`
	Copied  time.Time `json:"copied,omitzero"`
	Written []string  `json:"written,omitempty"` // the files copied into the folder, relative
	Hashes  []string  `json:"hashes,omitempty"`  // their content, to remove only what was not changed since
	Dirs    []string  `json:"dirs,omitempty"`    // the folders the copy created, deepest first
	Undone  bool      `json:"undone,omitempty"`
}

// Desk is where the drops of a folder wait: .kanso/intake.
type Desk struct {
	root folder.Root
	dir  string
	read func(paths []string) map[string]string // the text of PDFs (the folder's index reads them)
	now  func() time.Time
}

// NewDesk works for root; read gives the text of the files it is handed.
func NewDesk(root folder.Root, read func(paths []string) map[string]string) *Desk {
	return &Desk{root: root, dir: filepath.Join(root.Dir(), brand.DataDir, "intake"), read: read, now: time.Now}
}

// Incoming is one file being received: its path in what was dropped, and its content.
type Incoming struct {
	Path string
	Body io.Reader
}

// Receive copies a drop aside, file by file, as next hands them; next
// answers io.EOF at the end. Nothing is in the folder yet.
func (d *Desk) Receive(next func() (Incoming, error)) (*Batch, error) {
	b := &Batch{ID: newID(), Created: d.now(), Items: []Item{}}
	files := filepath.Join(d.dir, b.ID)
	if err := os.MkdirAll(files, 0o755); err != nil {
		return nil, fmt.Errorf("intake: %w", err)
	}
	for {
		in, err := next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err == nil && len(b.Items) >= maxFiles {
			err = ErrTooMany
		}
		if err != nil {
			_ = os.RemoveAll(files)
			return nil, err
		}
		rel := cleanPath(in.Path)
		if rel == "" {
			continue
		}
		staged := fmt.Sprintf("%05d%s", len(b.Items), strings.ToLower(path.Ext(rel)))
		size, err := copyTo(filepath.Join(files, staged), in.Body)
		if err != nil {
			_ = os.RemoveAll(files)
			return nil, err
		}
		b.Items = append(b.Items, Item{Path: rel, Size: size, Staged: staged})
	}
	return b, d.save(b)
}

// cleanPath keeps a dropped path usable: slash-separated, no hidden part, no
// « .. », each name cleaned; "" for a file to leave out (.DS_Store…).
func cleanPath(p string) string {
	var parts []string
	for _, seg := range strings.Split(strings.ReplaceAll(p, "\\", "/"), "/") {
		if seg == "" || seg == "." || seg == ".." {
			continue
		}
		if folder.Skip(seg) {
			return ""
		}
		name, err := folder.CleanName(seg)
		if err != nil {
			return ""
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, "/")
}

func copyTo(dst string, src io.Reader) (int64, error) {
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, fmt.Errorf("intake: %w", err)
	}
	n, err := io.Copy(f, src)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return 0, fmt.Errorf("intake: %w", err)
	}
	return n, nil
}

// sortable are the kinds of document filed by year, vendor and month.
var sortable = map[string]bool{"invoice": true, "receipt": true, "order": true, "statement": true}

// Plan reads each file of a drop and says where it goes: an invoice whose
// vendor and date were read goes to YEAR/Vendor/Month (a new month folder
// named by months, the user's language), any other file keeps
// the folders it was dropped in; a file whose content is already in the
// folder (known: hash → path) or earlier in the drop is a duplicate, left out.
func (d *Desk) Plan(b *Batch, known map[string]string, months []string, extractor func(rel, text string) extract.Extracted) error {
	files := filepath.Join(d.dir, b.ID)
	abs := make([]string, len(b.Items))
	for i, it := range b.Items {
		abs[i] = filepath.Join(files, it.Staged)
	}
	texts := d.read(abs)
	seen := map[string]string{}
	taken := map[string]bool{} // the targets already given in this drop
	for i := range b.Items {
		it := &b.Items[i]
		hash, err := folder.HashFile(abs[i])
		if err != nil {
			return err
		}
		it.Hash = hash
		info := extractor(it.Path, texts[abs[i]])
		it.Kind, it.Vendor, it.Date, it.Amount, it.Currency = info.Kind, info.Vendor, info.Date, info.Amount, info.Currency
		it.Target, it.Sorted, it.Duplicate = "", false, ""
		if p, ok := known[hash]; ok {
			it.Duplicate = p
			continue
		}
		if p, ok := seen[hash]; ok {
			it.Duplicate = p
			continue
		}
		seen[hash] = it.Path
		dir := path.Dir(it.Path)
		if sortable[it.Kind] && it.Vendor != "" && it.Date != "" {
			if planned, err := d.root.PlanDir(it.Vendor, it.Date, months); err == nil {
				dir, it.Sorted = d.root.Rel(planned), true
			}
		}
		if dir == "." {
			dir = ""
		}
		it.Target = freeName(d.root.Dir(), dir, path.Base(it.Path), taken)
		taken[strings.ToLower(it.Target)] = true
	}
	return d.save(b)
}

// freeName is dir/name, or dir/« name (2).ext » when a file of the folder or
// of this drop already has that name.
func freeName(root, dir, name string, taken map[string]bool) string {
	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		candidate := name
		if i > 1 {
			candidate = fmt.Sprintf("%s (%d)%s", base, i, ext)
		}
		rel := path.Join(dir, candidate)
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel))); errors.Is(err, os.ErrNotExist) && !taken[strings.ToLower(rel)] {
			return rel
		}
	}
}

// Copy puts the files of a planned drop into the folder: each to its target,
// created with its folders, never over another file (a name taken since gets
// « (2) »). The copies aside go; the drop keeps what it wrote, for Undo.
func (d *Desk) Copy(id string) (*Batch, error) {
	b, err := d.load(id)
	if err != nil {
		return nil, err
	}
	if !b.Copied.IsZero() {
		return nil, ErrUnknown
	}
	files := filepath.Join(d.dir, b.ID)
	taken := map[string]bool{}
	for _, it := range b.Items {
		if it.Target == "" {
			continue
		}
		rel := it.Target
		if _, err := os.Lstat(filepath.Join(d.root.Dir(), filepath.FromSlash(rel))); err == nil || taken[strings.ToLower(rel)] {
			rel = freeName(d.root.Dir(), path.Dir(rel), path.Base(rel), taken)
		}
		dst, err := d.root.Abs(rel)
		if err != nil {
			continue
		}
		b.Dirs = append(b.Dirs, missingDirs(d.root.Dir(), filepath.Dir(dst))...)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return b, d.fail(b, err)
		}
		// A hard link cannot replace a file: what is there stays there.
		if err := os.Link(filepath.Join(files, it.Staged), dst); err != nil {
			return b, d.fail(b, err)
		}
		taken[strings.ToLower(rel)] = true
		b.Written = append(b.Written, rel)
		b.Hashes = append(b.Hashes, it.Hash)
	}
	sort.Slice(b.Dirs, func(i, j int) bool { return len(b.Dirs[i]) > len(b.Dirs[j]) })
	b.Copied = d.now()
	_ = os.RemoveAll(files)
	return b, d.save(b)
}

// fail keeps what was already written (so Undo can remove it) and returns err.
func (d *Desk) fail(b *Batch, err error) error {
	b.Copied = d.now()
	_ = d.save(b)
	return fmt.Errorf("intake: copy: %w", err)
}

// missingDirs lists the folders between root and dir that do not exist yet, relative.
func missingDirs(root, dir string) []string {
	var out []string
	for dir != root && strings.HasPrefix(dir, root+string(filepath.Separator)) {
		if _, err := os.Lstat(dir); err == nil {
			break
		}
		rel, _ := filepath.Rel(root, dir)
		out = append(out, filepath.ToSlash(rel))
		dir = filepath.Dir(dir)
	}
	return out
}

// Undo removes what a copy wrote, except a file changed since (its content
// is no longer the one copied), then the folders it created when they are
// empty. It returns how many files were removed and how many were kept.
func (d *Desk) Undo(id string) (removed, kept int, err error) {
	b, err := d.load(id)
	if err != nil {
		return 0, 0, err
	}
	if b.Copied.IsZero() || b.Undone {
		return 0, 0, ErrUnknown
	}
	for i, rel := range b.Written {
		abs, err := d.root.Abs(rel)
		if err != nil {
			continue
		}
		if _, err := os.Lstat(abs); errors.Is(err, os.ErrNotExist) {
			continue // already gone
		}
		if h, err := folder.HashFile(abs); err != nil || h != b.Hashes[i] {
			kept++
			continue
		}
		if os.Remove(abs) == nil {
			removed++
		}
	}
	for _, rel := range b.Dirs {
		if abs, err := d.root.Abs(rel); err == nil {
			_ = os.Remove(abs) // only an empty folder goes
		}
	}
	b.Undone = true
	return removed, kept, d.save(b)
}

// Pending is a drop not copied yet.
func (d *Desk) Pending(id string) (*Batch, error) {
	b, err := d.load(id)
	if err != nil || !b.Copied.IsZero() {
		return nil, ErrUnknown
	}
	return b, nil
}

// Cancel forgets a drop not copied yet, with its copies aside.
func (d *Desk) Cancel(id string) error {
	b, err := d.load(id)
	if err != nil {
		return err
	}
	if !b.Copied.IsZero() {
		return ErrUnknown
	}
	_ = os.RemoveAll(filepath.Join(d.dir, b.ID))
	return os.Remove(filepath.Join(d.dir, b.ID+".json"))
}

// Last is the latest drop copied and not undone, nil when there is none.
func (d *Desk) Last() *Batch {
	var last *Batch
	for _, b := range d.list() {
		if !b.Copied.IsZero() && !b.Undone && (last == nil || b.Copied.After(last.Copied)) {
			last = b
		}
	}
	return last
}

// Tidy forgets the drops left waiting for a day, and keeps the 10 latest copies.
func (d *Desk) Tidy() {
	var copied []*Batch
	for _, b := range d.list() {
		switch {
		case b.Copied.IsZero() && d.now().Sub(b.Created) > 24*time.Hour:
			_ = os.RemoveAll(filepath.Join(d.dir, b.ID))
			_ = os.Remove(filepath.Join(d.dir, b.ID+".json"))
		case !b.Copied.IsZero():
			copied = append(copied, b)
		}
	}
	sort.Slice(copied, func(i, j int) bool { return copied[i].Copied.After(copied[j].Copied) })
	for _, b := range copied[min(10, len(copied)):] {
		_ = os.Remove(filepath.Join(d.dir, b.ID+".json"))
	}
}

func (d *Desk) list() []*Batch {
	des, err := os.ReadDir(d.dir)
	if err != nil {
		return nil
	}
	var out []*Batch
	for _, de := range des {
		if id, ok := strings.CutSuffix(de.Name(), ".json"); ok {
			if b, err := d.load(id); err == nil {
				out = append(out, b)
			}
		}
	}
	return out
}

func (d *Desk) load(id string) (*Batch, error) {
	if _, err := hex.DecodeString(id); len(id) != 16 || err != nil || strings.ToLower(id) != id {
		return nil, ErrUnknown
	}
	data, err := os.ReadFile(filepath.Join(d.dir, id+".json"))
	if err != nil {
		return nil, ErrUnknown
	}
	var b Batch
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, ErrUnknown
	}
	return &b, nil
}

func (d *Desk) save(b *Batch) error {
	data, err := json.Marshal(b)
	if err != nil {
		return err
	}
	tmp := filepath.Join(d.dir, b.ID+".json.tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("intake: %w", err)
	}
	return os.Rename(tmp, filepath.Join(d.dir, b.ID+".json"))
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
