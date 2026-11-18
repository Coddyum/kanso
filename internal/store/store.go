// Package store keeps one value in a JSON file: read once at start, written
// atomically after every change.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// Store holds a T and its file.
type Store[T any] struct {
	path    string
	fill    func(*T) // gives every list and map a value: the front reads [] and {}, never null
	mu      sync.Mutex
	data    T
	version int // incremented by every write
}

// Open reads path, or starts from the zero T when the file does not exist yet.
func Open[T any](path string, fill func(*T)) (*Store[T], error) {
	s := &Store[T]{path: path, fill: fill}
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
	s.data = next
	s.version++
	return nil
}
