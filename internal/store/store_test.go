package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type data struct {
	Items []string          `json:"items"`
	Tags  map[string]string `json:"tags"`
}

func fill(d *data) {
	if d.Items == nil {
		d.Items = []string{}
	}
	if d.Tags == nil {
		d.Tags = map[string]string{}
	}
}

func TestUpdateIsAllOrNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "db.json")
	s, err := Open(path, fill)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Update(func(d *data) error { d.Items = append(d.Items, "a"); return nil }); err != nil {
		t.Fatal(err)
	}
	refused := errors.New("refusé")
	err = s.Update(func(d *data) error {
		d.Items = append(d.Items, "b")
		d.Tags["x"] = "y"
		return refused
	})
	if !errors.Is(err, refused) {
		t.Fatalf("Update = %v", err)
	}
	got := s.Snapshot()
	if len(got.Items) != 1 || len(got.Tags) != 0 || s.Version() != 1 {
		t.Errorf("a failed update changed the data: %+v, version %d", got, s.Version())
	}
	again, err := Open(path, fill)
	if err != nil {
		t.Fatal(err)
	}
	if items := again.Snapshot().Items; len(items) != 1 || items[0] != "a" {
		t.Errorf("reopened = %v", items)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("the temporary file must be renamed away")
	}
}

func TestSnapshotIsACopy(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db.json"), fill)
	if err != nil {
		t.Fatal(err)
	}
	c := s.Snapshot()
	c.Tags["x"] = "y"
	if len(s.Snapshot().Tags) != 0 {
		t.Error("changing a snapshot changed the store")
	}
}

func TestOpenBrokenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.json")
	if err := os.WriteFile(path, []byte("{pas du json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path, fill); err == nil {
		t.Error("a broken database must stop the start, never be overwritten")
	}
}

// TestBackups: one copy a day of the file before its first change, the oldest
// pruned, and a restore that can itself be undone.
func TestBackups(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "db.json"), fill)
	if err != nil {
		t.Fatal(err)
	}
	s.KeepBackups(filepath.Join(dir, "backups"), 2)
	day := time.Date(2026, 11, 1, 10, 0, 0, 0, time.Local)
	s.now = func() time.Time { return day }
	set := func(v string) {
		t.Helper()
		if err := s.Update(func(d *data) error { d.Items = []string{v}; return nil }); err != nil {
			t.Fatal(err)
		}
	}
	set("a") // no file yet: nothing to save
	set("b") // the day's first change of an existing file: « a » is saved
	for i, v := range []string{"c", "d", "e"} {
		day = day.AddDate(0, 0, i+1)
		set(v)
	}
	list, err := s.Backups()
	if err != nil || len(list) != 2 {
		t.Fatalf("Backups = %v, %v; want the 2 newest", list, err)
	}
	if err := s.Restore("../db.json"); err != ErrNoBackup {
		t.Errorf("a name outside the folder must be refused, got %v", err)
	}
	if err := s.Restore(list[1].Name); err != nil {
		t.Fatal(err)
	}
	if got := s.Snapshot().Items[0]; got != "c" {
		t.Errorf("after restoring %s: %q, want c", list[1].Name, got)
	}
	after, _ := s.Backups()
	if len(after) != 3 || !strings.Contains(after[1].Name+after[2].Name, "before-restore") {
		t.Errorf("the file before the restore must be kept: %v", after)
	}
}
