package main

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
)

// Doc is one file of the synced folder.
type Doc struct {
	Path    string    `json:"path"` // relative, slash-separated
	Name    string    `json:"name"`
	Dir     string    `json:"dir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mtime"`
	Hash    string    `json:"hash"`
	Info    Extracted `json:"info"`
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
	pdftext   string

	mu     sync.Mutex
	loaded bool
	stamps map[string]stamp     // path → size, mtime, hash
	texts  map[string]string    // hash → PDF text
	infos  map[string]Extracted // path + hash → what extract read, in memory only
}

type indexCache struct {
	Stamps map[string]stamp  `json:"stamps"`
	Texts  map[string]string `json:"texts"`
}

const maxText = 12000

func skipName(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "~$") || name == "Icon\r"
}

func isPDF(name string) bool { return strings.EqualFold(filepath.Ext(name), ".pdf") }

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
		if skipName(name) {
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
		return nil, "", fmt.Errorf("index: walk %s: %w", ix.root, err)
	}
	return docs, hex.EncodeToString(sig.Sum(nil))[:16], nil
}

func (ix *Index) signature() (string, error) {
	_, sig, err := ix.walk()
	return sig, err
}

// scan returns every file with its hash and what was read in it.
func (ix *Index) scan() ([]Doc, error) {
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
			hash, err := hashFile(filepath.Join(ix.root, filepath.FromSlash(d.Path)))
			if err != nil {
				d.Hash = "path:" + d.Path
				continue
			}
			st = stamp{Size: d.Size, Mod: d.ModTime.UnixNano(), Hash: hash}
			ix.stamps[d.Path] = st
			dirty = true
		}
		d.Hash = st.Hash
		if _, ok := ix.texts[d.Hash]; !ok && isPDF(d.Name) && ix.pdftext != "" {
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
	infos := make(map[string]Extracted, len(docs))
	for i := range docs {
		key := docs[i].Path + "\x00" + docs[i].Hash
		info, ok := ix.infos[key]
		if !ok {
			info = extract(docs[i].Path, ix.texts[docs[i].Hash])
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

// readTexts runs the PDFKit helper on the new PDFs, 12 files per call, 4 calls at once.
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

func runPDFText(bin string, paths []string) map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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

func hashFile(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", fmt.Errorf("index: open %s: %w", p, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("index: hash %s: %w", p, err)
	}
	return hex.EncodeToString(h.Sum(nil))[:32], nil
}

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

func (ix *Index) save() {
	data, err := json.Marshal(indexCache{Stamps: ix.stamps, Texts: ix.texts})
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(ix.cacheFile), 0o755); err != nil {
		log.Printf("index: cache dir: %v", err)
		return
	}
	if err := os.WriteFile(ix.cacheFile, data, 0o644); err != nil {
		log.Printf("index: write cache: %v", err)
	}
}
