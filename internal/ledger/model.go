package ledger

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/coddyum/kanso/internal/deadlines"
	"github.com/coddyum/kanso/internal/folder"
	"github.com/coddyum/kanso/internal/i18n"
	"github.com/coddyum/kanso/internal/store"
	"github.com/coddyum/kanso/internal/vendor"
)

// Book is the database, and the vendors its invoices are read with.
type Book struct {
	store   *store.Store[DB]
	vendors *vendor.Catalog
}

// Open reads the database at path, or starts an empty one, written at once
// with the first-run checklist open. Every day's first change saves the file
// as it was in backups/ beside it (the last 30 days).
func Open(path string, vendors *vendor.Catalog) (*Book, error) {
	_, missing := os.Stat(path)
	s, err := store.Open(path, Fill)
	if err != nil {
		return nil, err
	}
	s.KeepBackups(filepath.Join(filepath.Dir(path), "backups"), 30)
	if errors.Is(missing, fs.ErrNotExist) {
		if err := s.Update(func(db *DB) error { db.onboarding(); return nil }); err != nil {
			return nil, err
		}
	}
	return &Book{store: s, vendors: vendors}, nil
}

// Backups lists the saved copies of the database, the newest first.
func (b *Book) Backups() ([]store.Backup, error) { return b.store.Backups() }

// Restore puts a saved copy back; the database as it was is saved first.
func (b *Book) Restore(name string) error {
	if err := b.store.Restore(name); errors.Is(err, store.ErrNoBackup) {
		return Invalid("error.noBackup")
	} else if err != nil {
		return err
	}
	return nil
}

// Snapshot is a copy of the whole database: what an export writes.
func (b *Book) Snapshot() DB { return b.store.Snapshot() }

// Version changes with every write to the database.
func (b *Book) Version() int { return b.store.Version() }

// Model is the folder and the database read together: what every screen is computed from.
type Model struct {
	DB          DB
	Docs        []folder.Doc
	Invoices    []Invoice
	Suggestions []Suggestion
	Added       []string // names of the subscriptions this very build added
	Today       time.Time

	vendors *vendor.Catalog
	byID    map[string]*Invoice
	dues    map[string]dueState
	history map[string][]Invoice // payment id → the invoices paired with its due dates that carry an amount, oldest first
}

// Model reads the folder's documents with the database's corrections, groups
// them into invoices, adds the running subscriptions it finds as payments
// (a write), and pairs every past due date with its invoice.
func (b *Book) Model(docs []folder.Doc, today time.Time) (*Model, error) {
	db := b.store.Snapshot()
	edited := map[string]bool{}
	dropped := map[string]bool{}
	for _, hash := range db.Links {
		dropped[hash] = true
	}
	for i := range docs {
		// A file dropped for a due date is its invoice, whatever its title says.
		if dropped[docs[i].Hash] && docs[i].Info.Kind == "document" {
			docs[i].Info.Kind = "invoice"
		}
		if o, ok := db.Overrides[docs[i].Hash]; ok {
			docs[i].Info = o.apply(docs[i].Info, b.vendors)
			edited[docs[i].Hash] = true
		}
	}
	invoices := groupInvoices(docs, edited)
	m := &Model{Docs: docs, Invoices: invoices, Today: today, vendors: b.vendors, byID: map[string]*Invoice{}}
	for i := range invoices {
		m.byID[invoices[i].ID] = &invoices[i]
	}
	m.Suggestions = suggest(invoices, db.Payments, db.Dismissed, today)
	var err error
	if m.Added, err = b.autoAdd(m.Suggestions); err != nil {
		return nil, err
	}
	if len(m.Added) > 0 {
		db = b.store.Snapshot()
		m.Suggestions = suggest(invoices, db.Payments, db.Dismissed, today)
	}
	for i := range invoices {
		if invoices[i].Domain != "" {
			continue
		}
		for _, p := range db.Payments {
			if p.Domain != "" && vendor.Matches(p.InvoiceName(), invoices[i].Vendor) {
				invoices[i].Domain = p.Domain
				break
			}
		}
	}
	sort.SliceStable(db.Payments, func(i, j int) bool { return db.Payments[i].Start < db.Payments[j].Start })
	m.DB = db
	m.dues = matchDues(db.Payments, invoices, db.Links, today)
	m.history = paidHistory(m.dues, m.byID)
	return m, nil
}

// Again is the same model served to a later request: nothing was added by it.
func (m *Model) Again() *Model {
	c := *m
	c.Added = nil
	return &c
}

// autoAdd turns every running subscription found in the invoices into a
// payment, unless one already stands for it or it was dismissed.
func (b *Book) autoAdd(sugs []Suggestion) ([]string, error) {
	var todo []Suggestion
	for _, s := range sugs {
		if s.Active {
			todo = append(todo, s)
		}
	}
	if len(todo) == 0 {
		return nil, nil
	}
	var added []string
	now := time.Now().Format(time.RFC3339)
	err := b.store.Update(func(db *DB) error {
		added = nil
		for _, s := range todo {
			if db.Dismissed[s.Key] || covered(db.Payments, s.Key, s.Vendor, s.Match) {
				continue
			}
			db.Payments = append(db.Payments, Payment{
				ID: NewID(), Name: s.Name, Vendor: s.Vendor, Domain: s.Domain, Match: s.Match,
				Amount: s.Amount, Currency: s.Currency, Cycle: s.Cycle, Start: s.Start,
				ExpectInvoice: true, Auto: true, AutoAt: now, SuggestKey: s.Key,
			})
			added = append(added, s.Name)
		}
		return nil
	})
	return added, err
}

// paidHistory lists, per payment, the invoices paired with its due dates that
// carry an amount, oldest first.
func paidHistory(dues map[string]dueState, byID map[string]*Invoice) map[string][]Invoice {
	out := map[string][]Invoice{}
	for key, st := range dues {
		if inv := byID[st.InvoiceID]; inv != nil && inv.Amount != nil {
			id, _, _ := strings.Cut(key, "@")
			out[id] = append(out[id], *inv)
		}
	}
	for id := range out {
		sort.Slice(out[id], func(i, j int) bool { return out[id][i].Date < out[id][j].Date })
	}
	return out
}

// Deadlines are the administrative due dates between from and to.
func (m *Model) Deadlines(from, to time.Time, tr i18n.Lang) []deadlines.Deadline {
	return deadlines.List(m.DB.Settings(), m.DB.Done, from, to, m.Today, tr)
}

// Describe fills, for the files of a folder view, what the app read in them
// and the invoice they belong to.
func (m *Model) Describe(entries []folder.Entry) {
	docOf := map[string]folder.Doc{}
	for _, d := range m.Docs {
		docOf[d.Path] = d
	}
	invoiceOf := map[string]string{}
	for _, inv := range m.Invoices {
		for _, f := range inv.Files {
			invoiceOf[f.Path] = inv.ID
		}
	}
	for i := range entries {
		if e := &entries[i]; !e.Dir {
			d := docOf[e.Path]
			e.Kind, e.Hash, e.InvoiceID = d.Info.Kind, d.Hash, invoiceOf[e.Path]
		}
	}
}
