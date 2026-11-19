package intake

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coddyum/kanso/internal/extract"
	"github.com/coddyum/kanso/internal/folder"
	"github.com/coddyum/kanso/internal/vendor"
)

// drop hands the files of a fake drop one by one: path → content.
func drop(files [][2]string) func() (Incoming, error) {
	i := 0
	return func() (Incoming, error) {
		if i == len(files) {
			return Incoming{}, io.EOF
		}
		f := files[i]
		i++
		return Incoming{Path: f[0], Body: strings.NewReader(f[1])}, nil
	}
}

// texts reads each staged file's content as its text: the tests write invoices as plain text.
func texts(paths []string) map[string]string {
	out := map[string]string{}
	for _, p := range paths {
		data, _ := os.ReadFile(p)
		out[p] = string(data)
	}
	return out
}

func TestDropCopyUndo(t *testing.T) {
	root := t.TempDir()
	vendors := vendor.NewCatalog()
	r := folder.NewRoot(root, vendors)
	// Already in the folder: an invoice (its content will be dropped again) and the target name of another.
	must(t, os.MkdirAll(filepath.Join(root, "2026", "OVHcloud", "Mars"), 0o755))
	must(t, os.WriteFile(filepath.Join(root, "2026", "OVHcloud", "Mars", "already.pdf"), []byte("old"), 0o644))
	must(t, os.MkdirAll(filepath.Join(root, "Contrats"), 0o755))
	must(t, os.WriteFile(filepath.Join(root, "Contrats", "bail.pdf"), []byte("another lease"), 0o644))
	invoice := "Facture OVH\nDate : 12/03/2026\nTotal TTC : 5,99 €"
	d := NewDesk(r, texts)
	b, err := d.Receive(drop([][2]string{
		{"Factures/OVH/f-03.pdf", invoice},
		{"Factures/OVH/copie de f-03.pdf", invoice}, // the same content twice in the drop
		{"Factures/old.pdf", "old"},                 // already in the folder
		{"Contrats/bail.pdf", "a lease"},            // not an invoice: keeps its folders, the name is taken
		{"Factures/.DS_Store", "x"},                 // left out
		{"../../etc/passwd", "root"},                // no way out of the drop
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Items) != 5 {
		t.Fatalf("received %d files, want 5 (.DS_Store left out)", len(b.Items))
	}
	if b.Items[4].Path != "etc/passwd" {
		t.Errorf("« .. » must not climb out: %q", b.Items[4].Path)
	}
	known := map[string]string{}
	if h, err := folder.HashFile(filepath.Join(root, "2026", "OVHcloud", "Mars", "already.pdf")); err == nil {
		known[h] = "2026/OVHcloud/Mars/already.pdf"
	}
	must(t, d.Plan(b, known, nil, func(rel, text string) extract.Extracted { return extract.Extract(vendors, rel, text) }))
	want := []struct{ target, dup string }{
		{"2026/OVHcloud/Mars/f-03.pdf", ""},
		{"", "Factures/OVH/f-03.pdf"},
		{"", "2026/OVHcloud/Mars/already.pdf"},
		{"Contrats/bail (2).pdf", ""},
		{"etc/passwd", ""},
	}
	for i, w := range want {
		if got := b.Items[i]; got.Target != w.target || got.Duplicate != w.dup {
			t.Errorf("item %d (%s): target %q dup %q, want %q %q", i, got.Path, got.Target, got.Duplicate, w.target, w.dup)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "2026", "OVHcloud", "Mars", "f-03.pdf")); err == nil {
		t.Fatal("planning must write nothing in the folder")
	}

	copied, err := d.Copy(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(copied.Written) != 3 {
		t.Fatalf("written = %q", copied.Written)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "Contrats", "bail.pdf")); string(data) != "another lease" {
		t.Error("a file of the folder must never be overwritten")
	}
	if _, err := d.Copy(b.ID); !errors.Is(err, ErrUnknown) {
		t.Errorf("a drop is copied once: %v", err)
	}
	if last := d.Last(); last == nil || last.ID != b.ID {
		t.Errorf("last = %+v", last)
	}

	// The user changes one of the copies: undo keeps it.
	must(t, os.WriteFile(filepath.Join(root, "Contrats", "bail (2).pdf"), []byte("edited"), 0o644))
	removed, kept, err := d.Undo(b.ID)
	if err != nil || removed != 2 || kept != 1 {
		t.Fatalf("undo = %d removed, %d kept, %v", removed, kept, err)
	}
	if _, err := os.Stat(filepath.Join(root, "etc")); err == nil {
		t.Error("a folder the copy created and left empty goes")
	}
	if _, err := os.Stat(filepath.Join(root, "2026", "OVHcloud", "Mars", "already.pdf")); err != nil {
		t.Error("what was in the folder before stays")
	}
	if d.Last() != nil {
		t.Error("an undone copy cannot be undone again")
	}
}

func TestCancelAndTidy(t *testing.T) {
	root := t.TempDir()
	d := NewDesk(folder.NewRoot(root, vendor.NewCatalog()), texts)
	b, err := d.Receive(drop([][2]string{{"a.pdf", "x"}}))
	if err != nil {
		t.Fatal(err)
	}
	must(t, d.Cancel(b.ID))
	if _, err := os.Stat(filepath.Join(d.dir, b.ID)); err == nil {
		t.Error("a cancelled drop leaves nothing aside")
	}
	if err := d.Cancel("../../x"); !errors.Is(err, ErrUnknown) {
		t.Errorf("an id that is not one: %v", err)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
