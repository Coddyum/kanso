package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
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
