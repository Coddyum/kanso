package demo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coddyum/kanso/internal/extract"
	"github.com/coddyum/kanso/internal/i18n"
	"github.com/coddyum/kanso/internal/ledger"
	"github.com/coddyum/kanso/internal/vendor"
	"github.com/coddyum/kanso/web"
)

func french(t *testing.T) i18n.Lang {
	c, err := i18n.Load(web.Files)
	if err != nil {
		t.Fatal(err)
	}
	return c.Lang("fr")
}

// TestInvoicesRead: every made-up invoice reads like a real one — its vendor, date, amount and currency.
func TestInvoicesRead(t *testing.T) {
	vendors := vendor.NewCatalog()
	d := time.Date(2026, 3, 8, 0, 0, 0, 0, time.Local)
	for _, s := range services {
		name, lines := s.invoice(d, 1)
		got := extract.Extract(vendors, "2026/"+s.folder+"/Mars/"+name, strings.Join(lines, "\n"))
		if !vendor.Matches(got.Vendor, s.name) || got.Date != "2026-03-08" || got.Amount == nil || *got.Amount != s.amount || got.Currency != s.currency || got.Kind != "invoice" {
			t.Errorf("%s reads as %+v (amount %v)", s.name, got, got.Amount)
		}
	}
}

func TestBuild(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Demo")
	today := time.Date(2026, 11, 19, 0, 0, 0, 0, time.Local)
	if err := Build(dir, today, french(t)); err != nil {
		t.Fatal(err)
	}
	if !Is(dir) {
		t.Fatal("the folder built is marked as a demo")
	}
	data, err := os.ReadFile(filepath.Join(dir, ".kanso", "db.json"))
	if err != nil {
		t.Fatal(err)
	}
	var db ledger.DB
	if err := json.Unmarshal(data, &db); err != nil {
		t.Fatal(err)
	}
	if len(db.Payments) != len(services)+1 || db.Reserve == nil || len(db.Projects) != 2 || db.Onboarding == nil || !db.Onboarding.Closed {
		t.Errorf("db = %d payments, reserve %v, %d projects", len(db.Payments), db.Reserve, len(db.Projects))
	}
	if _, err := os.Stat(filepath.Join(dir, "2026", "Figma", "Novembre")); err == nil {
		t.Error("Figma's latest invoice is left out: the missing screen has one")
	}
	if _, err := os.Stat(filepath.Join(dir, "2026", "Notion", "Novembre")); err != nil {
		t.Errorf("Notion's November invoice: %v", err)
	}
	// Built again: the demo is replaced, from scratch.
	if err := Build(dir, today, french(t)); err != nil {
		t.Fatal(err)
	}
	// A folder that is not a demo is never replaced.
	real := t.TempDir()
	if err := os.WriteFile(filepath.Join(real, "facture.pdf"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(real, today, french(t)); err == nil {
		t.Error("a folder that is not a demo must be left alone")
	}
	if _, err := os.Stat(filepath.Join(real, "facture.pdf")); err != nil {
		t.Error("its files stay")
	}
}
