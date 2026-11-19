package ledger

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/coddyum/kanso/internal/day"
	"github.com/coddyum/kanso/internal/folder"
	"github.com/coddyum/kanso/internal/vendor"
)

// An undone copy of invoices takes back the subscription the app added from
// them, and only that one: an older one, one from before AutoAt, the owner's
// and an edited one stay; one still shown by invoices comes back by itself.
func TestDropAutoSince(t *testing.T) {
	b, err := Open(filepath.Join(t.TempDir(), "db.json"), vendor.NewCatalog())
	if err != nil {
		t.Fatal(err)
	}
	older := time.Now().Add(-time.Hour).Format(time.RFC3339)
	if err := b.store.Update(func(db *DB) error {
		db.Payments = []Payment{
			{ID: "old", Name: "Ancien", Amount: 3, Currency: "EUR", Cycle: "monthly", Start: "2026-01-01", Auto: true, AutoAt: older},
			{ID: "pre", Name: "Avant", Amount: 3, Currency: "EUR", Cycle: "monthly", Start: "2026-01-01", Auto: true},
			{ID: "own", Name: "Mien", Amount: 3, Currency: "EUR", Cycle: "monthly", Start: "2026-01-01"},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	copied := time.Now()
	var docs []folder.Doc
	for i := range 4 {
		date := d("2026-07-05").AddDate(0, i, 0).Format(day.Format)
		docs = append(docs, doc("2026/Nuage/"+date+".pdf", "invoice", "Nuage", date, 12, nil))
	}
	m, err := b.Model(docs, d("2026-10-20"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Added, []string{"Nuage"}) {
		t.Fatalf("added = %q, want Nuage from its four monthly invoices", m.Added)
	}

	dropped, err := b.DropAutoSince(copied)
	if err != nil || !slices.Equal(dropped, []string{"Nuage"}) {
		t.Fatalf("dropped = %q, %v; want only Nuage", dropped, err)
	}
	if names := paymentNames(b); !slices.Equal(names, []string{"Ancien", "Avant", "Mien"}) {
		t.Errorf("payments left = %q", names)
	}
	if b.store.Snapshot().Dismissed["suggest:nuage"] {
		t.Error("an undone copy must not dismiss the detection")
	}

	// The invoices are still there: Nuage comes back; once edited, it stays.
	m, err = b.Model(docs, d("2026-10-20"))
	if err != nil || !slices.Equal(m.Added, []string{"Nuage"}) {
		t.Fatalf("added again = %q, %v; want Nuage back", m.Added, err)
	}
	for _, p := range b.store.Snapshot().Payments {
		if p.Name == "Nuage" {
			if _, err := b.SavePayment(p, false); err != nil {
				t.Fatal(err)
			}
		}
	}
	if dropped, _ := b.DropAutoSince(copied); len(dropped) != 0 {
		t.Errorf("dropped %q: an edited subscription is the owner's", dropped)
	}
}

func paymentNames(b *Book) []string {
	var names []string
	for _, p := range b.store.Snapshot().Payments {
		names = append(names, p.Name)
	}
	slices.Sort(names)
	return names
}
