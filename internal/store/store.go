// Package store keeps one value in a JSON file: read once at start, written
// atomically after every change.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Store holds a T and its file.
type Store[T any] struct {
	path    string
	fill    func(*T) // gives every list and map a value: the front reads [] and {}, never null
	mu      sync.Mutex
	data    T
	version int // incremented by every write

	backupDir string // "" until KeepBackups
	keep      int
	now       func() time.Time
}

// Open reads path, or starts from the zero T when the file does not exist yet.
func Open[T any](path string, fill func(*T)) (*Store[T], error) {
	s := &Store[T]{path: path, fill: fill, now: time.Now}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, fmt.Errorf("store: read %s: %w", path, err)
	default:
		if err := json.Unmarshal(data, &s.data); err != nil {
			return nil, fmt.Errorf("store: parse %s: %w", path, err)
		}
	}
	s.fill(&s.data)
	return s, nil
}

// Version changes with every write: what was computed from an older version is stale.
func (s *Store[T]) Version() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.version
}

// Snapshot returns a deep copy, safe to read and change without the lock.
func (s *Store[T]) Snapshot() T {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.copyLocked()
}

func (s *Store[T]) copyLocked() T {
	data, _ := json.Marshal(s.data)
	var c T
	_ = json.Unmarshal(data, &c)
	s.fill(&c)
	return c
}

// Update runs fn on a copy under the lock, then saves it atomically (a
// temporary file, then a rename) and keeps it. When fn or the write fails,
// nothing changes, in memory or on disk.
func (s *Store[T]) Update(fn func(*T) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.copyLocked()
	if err := fn(&next); err != nil {
		return err
	}
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return fmt.Errorf("store: encode: %w", err)
	}
	s.backupLocked()
	if err := s.writeLocked(data); err != nil {
		return err
	}
	s.data = next
	s.version++
	return nil
}

func (s *Store[T]) writeLocked(data []byte) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("store: mkdir: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("store: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("store: rename %s: %w", tmp, err)
	}
	return nil
}

// KeepBackups makes every day's first write save the file as it was before
// it, as dir/<name>-YYYY-MM-DD.json; only the keep newest copies stay.
func (s *Store[T]) KeepBackups(dir string, keep int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backupDir, s.keep = dir, keep
}

func (s *Store[T]) base() string { return strings.TrimSuffix(filepath.Base(s.path), ".json") }

// backupLocked copies the file before the day's first change. A failed copy
// is logged, never a reason to refuse the user's change.
func (s *Store[T]) backupLocked() {
	if s.backupDir == "" {
		return
	}
	target := filepath.Join(s.backupDir, s.base()+"-"+s.now().Format("2006-01-02")+".json")
	if _, err := os.Stat(target); err == nil {
		return
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err == nil {
		err = os.MkdirAll(s.backupDir, 0o755)
	}
	if err == nil {
		err = os.WriteFile(target, data, 0o644)
	}
	if err != nil {
		log.Printf("store: backup %s: %v", target, err)
		return
	}
	s.pruneLocked()
}

// pruneLocked removes the daily copies beyond the keep newest.
func (s *Store[T]) pruneLocked() {
	list, err := s.backupsLocked()
	if err != nil {
		return
	}
	for _, b := range list[min(len(list), s.keep):] {
		_ = os.Remove(filepath.Join(s.backupDir, b.Name))
	}
}

// Backup is a saved copy of the file.
type Backup struct {
	Name string    `json:"name"`
	Date time.Time `json:"date"` // when it was written
	Size int64     `json:"size"`
}

var reBackup = regexp.MustCompile(`^[a-z]+-\d{4}-\d{2}-\d{2}(-before-restore-\d{6})?\.json$`)

// Backups lists the saved copies, the newest first.
func (s *Store[T]) Backups() ([]Backup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.backupsLocked()
}

func (s *Store[T]) backupsLocked() ([]Backup, error) {
	if s.backupDir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(s.backupDir)
	if errors.Is(err, fs.ErrNotExist) {
		return []Backup{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: backups: %w", err)
	}
	list := []Backup{}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || e.IsDir() || !reBackup.MatchString(e.Name()) || !strings.HasPrefix(e.Name(), s.base()+"-") {
			continue
		}
		list = append(list, Backup{Name: e.Name(), Date: info.ModTime(), Size: info.Size()})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name > list[j].Name })
	return list, nil
}

// ErrNoBackup is a restore of a copy that does not exist.
var ErrNoBackup = errors.New("store: no such backup")

// Restore puts a saved copy back. The file as it is now is saved first, as
// <name>-YYYY-MM-DD-before-restore-HHMMSS.json, so a restore can be undone.
func (s *Store[T]) Restore(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.backupDir == "" || !reBackup.MatchString(name) || !strings.HasPrefix(name, s.base()+"-") {
		return ErrNoBackup
	}
	data, err := os.ReadFile(filepath.Join(s.backupDir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNoBackup
	}
	if err != nil {
		return fmt.Errorf("store: restore: %w", err)
	}
	var next T
	if err := json.Unmarshal(data, &next); err != nil {
		return fmt.Errorf("store: restore %s: %w", name, err)
	}
	s.fill(&next)
	if current, err := os.ReadFile(s.path); err == nil {
		safety := filepath.Join(s.backupDir, s.base()+"-"+s.now().Format("2006-01-02")+"-before-restore-"+s.now().Format("150405")+".json")
		if err := os.WriteFile(safety, current, 0o644); err != nil {
			return fmt.Errorf("store: restore: save the current file: %w", err)
		}
	}
	if err := s.writeLocked(data); err != nil {
		return err
	}
	s.data = next
	s.version++
	return nil
}
