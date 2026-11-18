package folder

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coddyum/kanso/internal/vendor"
)

func newRoot(t *testing.T) Root {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return NewRoot(dir, vendor.NewCatalog())
}

func TestAbsRefuses(t *testing.T) {
	r := newRoot(t)
	for _, rel := range []string{".kanso/db.json", "2026/.cache", "a/../.git"} {
		var refused Refused
		if _, err := r.Abs(rel); !errors.As(err, &refused) {
			t.Errorf("Abs(%q) = %v, want a refusal", rel, err)
		}
	}
	for rel, want := range map[string]string{"": r.Dir(), "../../etc": r.Dir() + "/etc", "2026/Mars": r.Dir() + "/2026/Mars"} {
		if got, err := r.Abs(rel); err != nil || got != want {
			t.Errorf("Abs(%q) = %q, %v; want %q (always inside the folder)", rel, got, err, want)
		}
	}
}

func TestCleanName(t *testing.T) {
	for in, want := range map[string]string{"  facture.pdf ": "facture.pdf", "../x/y.pdf": "y.pdf", ".cache": "cache", "a:b": "a-b"} {
		if got, err := CleanName(in); err != nil || got != want {
			t.Errorf("CleanName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "...", strings.Repeat("a", 201)} {
		if _, err := CleanName(bad); err == nil {
			t.Errorf("CleanName(%q) must refuse", bad)
		}
	}
}

func TestWriteNeverOverwrites(t *testing.T) {
	r := newRoot(t)
	for i, want := range []string{"facture.pdf", "facture (2).pdf", "facture (3).pdf"} {
		got, err := r.Write(r.Dir(), "facture.pdf", strings.NewReader("contenu"))
		if err != nil || got != want {
			t.Errorf("write %d = %q, %v; want %q", i, got, err, want)
		}
	}
}

func TestRenameAndMove(t *testing.T) {
	r := newRoot(t)
	for _, p := range []string{"A", "B"} {
		if _, err := r.Mkdir("", p); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.Write(filepath.Join(r.Dir(), "A"), "x.pdf", strings.NewReader("1")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Write(filepath.Join(r.Dir(), "B"), "x.pdf", strings.NewReader("2")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Move("A/x.pdf", "B"); err == nil {
		t.Error("a move onto another file must fail")
	}
	if _, err := r.Move("A", "A"); err == nil {
		t.Error("a folder cannot go into itself")
	}
	if got, err := r.Rename("A/x.pdf", "y.pdf"); err != nil || got != "A/y.pdf" {
		t.Errorf("Rename = %q, %v", got, err)
	}
	if _, err := r.Rename("", "z"); err == nil {
		t.Error("the folder itself cannot be renamed")
	}
}

func TestListOrder(t *testing.T) {
	r := newRoot(t)
	for _, p := range []string{"2025", "2026", "Mars", "Janvier", "notes"} {
		if err := os.Mkdir(filepath.Join(r.Dir(), p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(r.Dir(), "a.pdf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.Dir(), ".DS_Store"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, entries, err := r.List("")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name)
	}
	if got, want := strings.Join(names, ","), "2026,2025,Janvier,Mars,notes,a.pdf"; got != want {
		t.Errorf("List = %s, want %s", got, want)
	}
}

func TestTargetDirReusesFolders(t *testing.T) {
	r := newRoot(t)
	if err := os.MkdirAll(filepath.Join(r.Dir(), "2026", "OVH", "aout"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := r.TargetDir("OVHcloud", "2026-08-14")
	if err != nil || got != filepath.Join(r.Dir(), "2026", "OVH", "aout") {
		t.Errorf("TargetDir = %q, %v; want the existing 2026/OVH/aout", got, err)
	}
	got, err = r.TargetDir("Nuage", "2026-03-02")
	if err != nil || got != filepath.Join(r.Dir(), "2026", "Nuage", "Mars") {
		t.Errorf("TargetDir new = %q, %v", got, err)
	}
}
