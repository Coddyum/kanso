package folder

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/coddyum/kanso/internal/day"
	"github.com/coddyum/kanso/internal/norm"
	"github.com/coddyum/kanso/internal/vendor"
)

// maxUpload is the largest file the app writes; a bigger one is refused, never cut.
const maxUpload = 500 << 20

// Refused is a path or a name the folder does not accept: the user's to fix.
// Any other error of an operation is the disk's (a file already there…).
type Refused string

func (e Refused) Error() string { return string(e) }

// Root is the synced folder, for the operations the app makes on its files.
// Every path it takes is relative to the folder; hidden files and anything
// outside the folder are refused.
type Root struct {
	dir     string
	vendors *vendor.Catalog
}

// NewRoot works in dir, an absolute path.
func NewRoot(dir string, vendors *vendor.Catalog) Root { return Root{dir: dir, vendors: vendors} }

// Dir is the folder's absolute path.
func (r Root) Dir() string { return r.dir }

// Abs turns a relative path of the folder into an absolute one, refusing
// anything that leaves the folder or names a hidden file.
func (r Root) Abs(rel string) (string, error) {
	clean := path.Clean("/" + strings.ReplaceAll(rel, "\\", "/"))
	for _, seg := range strings.Split(clean, "/") {
		if strings.HasPrefix(seg, ".") {
			return "", Refused("chemin refusé : " + rel)
		}
	}
	abs := filepath.Join(r.dir, filepath.FromSlash(clean))
	if abs != r.dir && !strings.HasPrefix(abs, r.dir+string(filepath.Separator)) {
		return "", Refused("chemin refusé : " + rel)
	}
	return abs, nil
}

// Rel is the relative, slash-separated path of abs; "" for the folder itself.
func (r Root) Rel(abs string) string {
	rel, err := filepath.Rel(r.dir, abs)
	if err != nil || rel == "." {
		return ""
	}
	return filepath.ToSlash(rel)
}

// CleanName keeps a file or folder name usable on the Mac: no path, no
// leading dot, no colon, at most 200 bytes.
func CleanName(name string) (string, error) {
	name = strings.TrimSpace(filepath.Base(strings.ReplaceAll(name, "\\", "/")))
	name = strings.TrimLeft(name, ".")
	name = strings.ReplaceAll(name, ":", "-")
	if name == "" || name == "/" || len(name) > 200 {
		return "", Refused("nom invalide")
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
	Items     int       `json:"items,omitempty"` // a folder: how many visible things it holds
	Kind      string    `json:"kind,omitempty"`
	Hash      string    `json:"hash,omitempty"`
	InvoiceID string    `json:"invoiceId,omitempty"`
}

// List returns what a folder holds, folders first, years newest first, months
// in calendar order; Kind, Hash and InvoiceID are the caller's to fill.
func (r Root) List(rel string) (string, []Entry, error) {
	abs, err := r.Abs(rel)
	if err != nil {
		return "", nil, err
	}
	des, err := os.ReadDir(abs)
	if err != nil {
		return "", nil, fmt.Errorf("dossier introuvable : %s", rel)
	}
	entries := []Entry{}
	for _, de := range des {
		if Skip(de.Name()) {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		e := Entry{Name: de.Name(), Path: r.Rel(filepath.Join(abs, de.Name())), Dir: de.IsDir(), Size: info.Size(), Mod: info.ModTime()}
		if e.Dir {
			if sub, err := os.ReadDir(filepath.Join(abs, de.Name())); err == nil {
				for _, s := range sub {
					if !Skip(s.Name()) {
						e.Items++
					}
				}
			}
		}
		entries = append(entries, e)
	}
	sort.SliceStable(entries, func(i, j int) bool { return lessEntry(entries[i], entries[j]) })
	return r.Rel(abs), entries, nil
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
	mx, my := day.MonthOf(x.Name), day.MonthOf(y.Name)
	if mx > 0 && my > 0 && mx != my {
		return mx < my
	}
	return strings.ToLower(x.Name) < strings.ToLower(y.Name)
}

// Mkdir creates a folder named name in parent and returns its path.
func (r Root) Mkdir(parent, name string) (string, error) {
	dir, err := r.Abs(parent)
	name, err2 := CleanName(name)
	if err = errors.Join(err, err2); err != nil {
		return "", err
	}
	target := filepath.Join(dir, name)
	if err := os.Mkdir(target, 0o755); err != nil {
		return "", fmt.Errorf("création de « %s » impossible : %w", name, err)
	}
	return r.Rel(target), nil
}

// Rename gives a file or folder a new name in the same folder; never the root, never over another file.
func (r Root) Rename(rel, name string) (string, error) {
	src, err := r.Abs(rel)
	name, err2 := CleanName(name)
	if err = errors.Join(err, err2); err != nil || src == r.dir {
		return "", errors.Join(err, Refused("renommage refusé"))
	}
	dst := filepath.Join(filepath.Dir(src), name)
	if err := moveNoClobber(src, dst); err != nil {
		return "", err
	}
	return r.Rel(dst), nil
}

// Move puts a file or folder in another folder; never the root, never a folder into itself.
func (r Root) Move(rel, toDir string) (string, error) {
	src, err := r.Abs(rel)
	dir, err2 := r.Abs(toDir)
	if err = errors.Join(err, err2); err != nil || src == r.dir {
		return "", errors.Join(err, Refused("déplacement refusé"))
	}
	if dir == src || strings.HasPrefix(dir, src+string(filepath.Separator)) {
		return "", Refused("un dossier ne peut pas aller dans lui-même")
	}
	dst := filepath.Join(dir, filepath.Base(src))
	if err := moveNoClobber(src, dst); err != nil {
		return "", err
	}
	return r.Rel(dst), nil
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

// Open opens a file with its usual app, or shows it in the Finder when reveal is set.
func (r Root) Open(rel string, reveal bool) error {
	abs, err := r.Abs(rel)
	if err != nil {
		return err
	}
	args := []string{abs}
	if reveal {
		args = []string{"-R", abs}
	}
	if err := exec.Command("open", args...).Run(); err != nil {
		return fmt.Errorf("open : %w", err)
	}
	return nil
}

// Write creates name in the folder dir (absolute), or « name (2).ext » when it
// is taken, and returns its relative path. A file over 500 MB is refused.
func (r Root) Write(dir, name string, src io.Reader) (string, error) {
	name, err := CleanName(name)
	if err != nil {
		return "", err
	}
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
		n, err := io.Copy(f, io.LimitReader(src, maxUpload+1))
		if err == nil && n > maxUpload {
			err = errors.New("fichier de plus de 500 Mo")
		}
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(p)
			return "", fmt.Errorf("écriture de %s : %w", candidate, err)
		}
		return r.Rel(p), nil
	}
	return "", fmt.Errorf("trop de fichiers nommés %s", name)
}

// TargetDir is YEAR/Vendor/Month for a due date, created when missing; existing
// folders are reused whatever their case or spelling (« aout », « OVH »…).
func (r Root) TargetDir(vendorName, date string) (string, error) {
	t, ok := day.Parse(date)
	if !ok {
		t = time.Now()
	}
	year := strconv.Itoa(t.Year())
	dir := filepath.Join(r.dir, findChild(r.dir, year, func(n string) bool { return n == year }))
	known, isKnown := r.vendors.Find(vendorName)
	vendorFolder, err := CleanName(vendorName)
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, findChild(dir, vendorFolder, func(n string) bool {
		if v, ok := r.vendors.Find(n); ok && isKnown {
			return v.Name == known.Name
		}
		return norm.Key(n) == norm.Key(vendorName)
	}))
	month := int(t.Month())
	dir = filepath.Join(dir, findChild(dir, day.MonthFolders[month-1], func(n string) bool {
		return day.IsTimeName(n) && day.MonthOf(strings.Fields(norm.Words(n))[0]) == month
	}))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("création de %s : %w", dir, err)
	}
	return dir, nil
}

// findChild is the first visible subfolder of dir that match accepts, or fallback.
func findChild(dir, fallback string, match func(name string) bool) string {
	des, err := os.ReadDir(dir)
	if err != nil {
		return fallback
	}
	for _, de := range des {
		if de.IsDir() && !Skip(de.Name()) && match(de.Name()) {
			return de.Name()
		}
	}
	return fallback
}
