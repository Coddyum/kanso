package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// DB is everything the folder itself cannot tell: planned payments, reminders
// and the corrections made in the app. Saved as one JSON file.
type DB struct {
	Payments  []Payment           `json:"payments"`
	Todos     []Todo              `json:"todos"`
	Dismissed map[string]bool     `json:"dismissed"` // due keys with no invoice, ignored suggestions
	Links     map[string]string   `json:"links"`     // due key → hash of the file dropped for it
	Overrides map[string]Override `json:"overrides"` // file hash → corrected fields
	Reserve   *Reserve            `json:"reserve,omitempty"`
	Projects  []Project           `json:"projects"`
	Assign    map[string]string   `json:"assign"` // « svc:<service> » or « inv:<invoice id> » → project id, « - » for none
	Admin     *Admin              `json:"admin,omitempty"`
	Done      map[string]bool     `json:"done"` // administrative due dates ticked as done
}

// Reserve is the balance of the business account on a given day, in euros.
type Reserve struct {
	Amount float64 `json:"amount"`
	Date   string  `json:"date"`
}

// Payment is a planned expense: once, or repeated on a cycle.
type Payment struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Vendor        string  `json:"vendor,omitempty"` // name looked for on invoices; Name when empty
	Domain        string  `json:"domain,omitempty"` // for the logo
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Cycle         string  `json:"cycle"` // once | weekly | monthly | quarterly | yearly
	Start         string  `json:"start"` // first due date, YYYY-MM-DD
	End           string  `json:"end,omitempty"`
	ExpectInvoice bool    `json:"expectInvoice"`
	Note          string  `json:"note,omitempty"`
	Match         string  `json:"match,omitempty"`      // text its invoices carry (a domain name…); empty: any invoice of the vendor
	AmountSet     string  `json:"amountSet,omitempty"`  // when Amount was typed (RFC 3339): it wins until a newer invoice arrives
	Auto          bool    `json:"auto,omitempty"`       // added by the app from the invoices of the folder
	SuggestKey    string  `json:"suggestKey,omitempty"` // the detection it comes from, dismissed when it is deleted
	Billing       string  `json:"billing,omitempty"`    // page of its invoices, typed by the owner; the vendor's known page otherwise
}

// Todo is a free reminder of something to fetch.
type Todo struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Done    bool   `json:"done"`
	Created string `json:"created"`
}

// Override corrects what was read in a file. Empty fields keep the extracted value.
type Override struct {
	Kind     string   `json:"kind,omitempty"` // "document" removes it from invoices, "invoice" adds it
	Vendor   string   `json:"vendor,omitempty"`
	Label    string   `json:"label,omitempty"`
	Date     string   `json:"date,omitempty"`
	Amount   *float64 `json:"amount,omitempty"`
	Currency string   `json:"currency,omitempty"`
}

func (o Override) apply(e Extracted) Extracted {
	if o.Kind != "" {
		e.Kind = o.Kind
	}
	if o.Vendor != "" {
		e.Vendor = o.Vendor
		e.Domain = vendorDomain(o.Vendor)
	}
	if o.Label != "" {
		e.Label = o.Label
	}
	if o.Date != "" {
		e.Date = o.Date
	}
	if o.Amount != nil {
		e.Amount = o.Amount
	}
	if o.Currency != "" {
		e.Currency = o.Currency
	}
	return e
}

type Store struct {
	path    string
	mu      sync.Mutex
	db      DB
	version int // incremented by every write
}

// Version changes with every write: what was computed from an older version is stale.
func (s *Store) Version() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.version
}

func openStore(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, fmt.Errorf("store: read %s: %w", path, err)
	default:
		if err := json.Unmarshal(data, &s.db); err != nil {
			return nil, fmt.Errorf("store: parse %s: %w", path, err)
		}
	}
	s.db.ensureMaps()
	return s, nil
}

// ensureMaps gives every list and map a value: the front reads [] and {}, never null.
func (d *DB) ensureMaps() {
	if d.Payments == nil {
		d.Payments = []Payment{}
	}
	if d.Todos == nil {
		d.Todos = []Todo{}
	}
	if d.Dismissed == nil {
		d.Dismissed = map[string]bool{}
	}
	if d.Links == nil {
		d.Links = map[string]string{}
	}
	if d.Overrides == nil {
		d.Overrides = map[string]Override{}
	}
	if d.Projects == nil {
		d.Projects = []Project{}
	}
	if d.Assign == nil {
		d.Assign = map[string]string{}
	}
	if d.Done == nil {
		d.Done = map[string]bool{}
	}
}

// snapshot returns a deep copy, safe to read without the lock.
func (s *Store) snapshot() DB {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, _ := json.Marshal(s.db)
	var c DB
	_ = json.Unmarshal(data, &c)
	c.ensureMaps()
	return c
}

// update runs fn under the lock and saves the result atomically.
func (s *Store) update(fn func(*DB) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fn(&s.db); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.db, "", "  ")
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
	s.version++
	return nil
}
