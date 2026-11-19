// Package folder is the synced folder: its index (every file, its hash and
// what was read in it) and the safe operations on its files.
package folder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coddyum/kanso/internal/extract"
	"github.com/coddyum/kanso/internal/vendor"
)

// Doc is one file of the synced folder.
type Doc struct {
	Path    string            `json:"path"` // relative, slash-separated
	Name    string            `json:"name"`
	Dir     string            `json:"dir"`
	Size    int64             `json:"size"`
	ModTime time.Time         `json:"mtime"`
	Hash    string            `json:"hash"`
	Info    extract.Extracted `json:"info"`
}

type stamp struct {
	Size int64  `json:"size"`
	Mod  int64  `json:"mod"`
	Hash string `json:"hash"`
}

// Index walks the folder and remembers, per file, its hash and its PDF text,
// so a file is hashed and read once, not at every request.
type Index struct {
	root      string
	cacheFile string
	pdftext   string // the reading helper (PDFKit, Vision for scans and pictures); "" when missing: files are listed without their text
	vendors   *vendor.Catalog

	mu     sync.Mutex
	loaded bool
	stamps map[string]stamp             // path → size, mtime, hash
	texts  map[string]string            // hash → PDF text
	infos  map[string]extract.Extracted // path + hash → what extract read, in memory only
}

type indexCache struct {
	Version int               `json:"version"`
	Stamps  map[string]stamp  `json:"stamps"`
	Texts   map[string]string `json:"texts"`
}

// cacheVersion 2: pictures and scanned PDFs are read by Vision. A text left
// empty by an older version (a scan PDFKit could not read) is read again.
const cacheVersion = 2

const maxText = 12000

// NewIndex indexes root, keeping the hashes and texts in cacheFile between runs.
func NewIndex(root, cacheFile, pdftext string, vendors *vendor.Catalog) *Index {
	return &Index{root: root, cacheFile: cacheFile, pdftext: pdftext, vendors: vendors}
}

// CanReadPDF says whether the reading helper was found.
func (ix *Index) CanReadPDF() bool { return ix.pdftext != "" }

// Skip says whether a file or folder is left out: hidden, an Office lock file, a Finder icon.
func Skip(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "~$") || name == "Icon\r"
}

func isPDF(name string) bool { return strings.EqualFold(filepath.Ext(name), ".pdf") }

// pictures are the image files the helper reads with Vision: photos of receipts, scans.
var pictures = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".heic": true, ".heif": true, ".tif": true, ".tiff": true, ".gif": true, ".webp": true, ".bmp": true}

// readable says whether the helper reads the file: a PDF (a scan through Vision too), or a picture.
func readable(name string) bool { return isPDF(name) || pictures[strings.ToLower(filepath.Ext(name))] }

// walk lists the files (hidden ones and the asset folders of saved web pages
// excepted) and a signature that changes whenever a file is added, removed,
// renamed or modified.
func (ix *Index) walk() ([]Doc, string, error) {
	var docs []Doc
	sig := sha256.New()
	err := filepath.WalkDir(ix.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == ix.root {
			return nil
		}
		name := d.Name()
		if Skip(name) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(ix.root, p)
		rel = filepath.ToSlash(rel)
		fmt.Fprintf(sig, "%s|%d|%d\n", rel, info.Size(), info.ModTime().UnixNano())
		if d.IsDir() {
			if strings.HasSuffix(name, "_files") {
				return filepath.SkipDir
			}
			return nil
		}
		dir := path.Dir(rel)
		if dir == "." {
			dir = ""
		}
		docs = append(docs, Doc{Path: rel, Name: name, Dir: dir, Size: info.Size(), ModTime: info.ModTime()})
		return nil
	})
	if err != nil {
		return nil, "", fmt.Errorf("folder: walk %s: %w", ix.root, err)
	}
	return docs, hex.EncodeToString(sig.Sum(nil))[:16], nil
}

// Signature changes whenever a file is added, removed, renamed or modified.
func (ix *Index) Signature() (string, error) {
	_, sig, err := ix.walk()
	return sig, err
}

// Scan returns every file with its hash and what was read in it; new or
// changed PDFs are read now, and the cache is saved when it changed.
func (ix *Index) Scan() ([]Doc, error) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	ix.load()
	docs, _, err := ix.walk()
	if err != nil {
		return nil, err
	}
	dirty := false
	seen := make(map[string]bool, len(docs))
	var toRead []int
	for i := range docs {
		d := &docs[i]
		seen[d.Path] = true
		st, ok := ix.stamps[d.Path]
		if !ok || st.Size != d.Size || st.Mod != d.ModTime.UnixNano() {
			hash, err := HashFile(filepath.Join(ix.root, filepath.FromSlash(d.Path)))
			if err != nil {
				d.Hash = "path:" + d.Path
				continue
			}
			st = stamp{Size: d.Size, Mod: d.ModTime.UnixNano(), Hash: hash}
			ix.stamps[d.Path] = st
			dirty = true
		}
		d.Hash = st.Hash
		if _, ok := ix.texts[d.Hash]; !ok && readable(d.Name) && ix.pdftext != "" {
			toRead = append(toRead, i)
		}
	}
	for p := range ix.stamps {
		if !seen[p] {
			delete(ix.stamps, p)
			dirty = true
		}
	}
	if len(toRead) > 0 {
		ix.readTexts(docs, toRead)
		dirty = true
	}
	infos := make(map[string]extract.Extracted, len(docs))
	for i := range docs {
		key := docs[i].Path + "\x00" + docs[i].Hash
		info, ok := ix.infos[key]
		if !ok {
			info = extract.Extract(ix.vendors, docs[i].Path, ix.texts[docs[i].Hash])
		}
		infos[key], docs[i].Info = info, info
	}
	ix.infos = infos
	if dirty {
		ix.prune()
		ix.save()
	}
	return docs, nil
}

// readTexts runs the reading helper on the new PDFs and pictures, 12 files per call, 4 calls at once.
func (ix *Index) readTexts(docs []Doc, idx []int) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, 4)
	for start := 0; start < len(idx); start += 12 {
		batch := idx[start:min(start+12, len(idx))]
		paths := make([]string, len(batch))
		for k, i := range batch {
			paths[k] = filepath.Join(ix.root, filepath.FromSlash(docs[i].Path))
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			texts := runPDFText(ix.pdftext, paths)
			mu.Lock()
			defer mu.Unlock()
			for k, i := range batch {
				t := texts[paths[k]]
				if len(t) > maxText {
					t = t[:maxText]
				}
				ix.texts[docs[i].Hash] = t
			}
		}()
	}
	wg.Wait()
}

// runPDFText reads blocks « \x1e path \n text » from the helper; a failed call logs and reads nothing.
func runPDFText(bin string, paths []string) map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second) // Vision takes a second or so a page
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, paths...).Output()
	if err != nil {
		log.Printf("pdftext: %v", err)
	}
	res := make(map[string]string, len(paths))
	for _, block := range strings.Split(string(out), "\x1e") {
		if p, text, ok := strings.Cut(block, "\n"); ok {
			res[p] = text
		}
	}
	return res
}

// HashFile is the identity of a file's content: 32 hex characters of its SHA-256.
func HashFile(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", fmt.Errorf("folder: open %s: %w", p, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("folder: hash %s: %w", p, err)
	}
	return hex.EncodeToString(h.Sum(nil))[:32], nil
}

// load reads the cache once; a missing or broken cache starts empty.
func (ix *Index) load() {
	if ix.loaded {
		return
	}
	ix.loaded = true
	ix.stamps, ix.texts = map[string]stamp{}, map[string]string{}
	data, err := os.ReadFile(ix.cacheFile)
	if err != nil {
		return
	}
	var c indexCache
	if json.Unmarshal(data, &c) != nil {
		return
	}
	if c.Stamps != nil {
		ix.stamps = c.Stamps
	}
	if c.Texts != nil {
		ix.texts = c.Texts
	}
	if c.Version < cacheVersion {
		for h, t := range ix.texts {
			if strings.TrimSpace(t) == "" {
				delete(ix.texts, h)
			}
		}
	}
}

// prune forgets the texts of files that are gone.
func (ix *Index) prune() {
	live := make(map[string]bool, len(ix.stamps))
	for _, st := range ix.stamps {
		live[st.Hash] = true
	}
	for h := range ix.texts {
		if !live[h] {
			delete(ix.texts, h)
		}
	}
}

// save writes the cache; a failure only costs reading the PDFs again next time.
func (ix *Index) save() {
	data, err := json.Marshal(indexCache{Version: cacheVersion, Stamps: ix.stamps, Texts: ix.texts})
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(ix.cacheFile), 0o755); err != nil {
		log.Printf("folder: cache dir: %v", err)
		return
	}
	if err := os.WriteFile(ix.cacheFile, data, 0o644); err != nil {
		log.Printf("folder: write cache: %v", err)
	}
}

// Texts reads the text of files that are not in the folder yet (a drop being
// sorted), with the same helper and limits as the folder's own files; a file
// it cannot read has no entry.
func (ix *Index) Texts(paths []string) map[string]string {
	out := map[string]string{}
	if ix.pdftext == "" {
		return out
	}
	var todo []string
	for _, p := range paths {
		if readable(p) {
			todo = append(todo, p)
		}
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for start := 0; start < len(todo); start += 12 {
		batch := todo[start:min(start+12, len(todo))]
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			texts := runPDFText(ix.pdftext, batch)
			mu.Lock()
			defer mu.Unlock()
			for _, p := range batch {
				t := texts[p]
				if len(t) > maxText {
					t = t[:maxText]
				}
				out[p] = t
			}
		}()
	}
	wg.Wait()
	return out
}
