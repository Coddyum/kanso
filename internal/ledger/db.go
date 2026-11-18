// Package ledger is the business of the app: the planned payments, the
// invoices of the folder, how they pair up, and what follows from it — the
// calendar, the missing invoices, the analytics, the reserve and the reminders.
package ledger

import (
	"github.com/coddyum/kanso/internal/deadlines"
	"github.com/coddyum/kanso/internal/extract"
	"github.com/coddyum/kanso/internal/vendor"
)

// DB is everything the folder itself cannot tell: planned payments, reminders
// and the corrections made in the app. Saved as one JSON file.
type DB struct {
	Payments   []Payment           `json:"payments"`
	Todos      []Todo              `json:"todos"`
	Dismissed  map[string]bool     `json:"dismissed"` // due keys with no invoice, ignored suggestions
	Links      map[string]string   `json:"links"`     // due key → hash of the file dropped for it
	Overrides  map[string]Override `json:"overrides"` // file hash → corrected fields
	Reserve    *Reserve            `json:"reserve,omitempty"`
	Projects   []Project           `json:"projects"`
	Assign     map[string]string   `json:"assign"` // « svc:<service> » or « inv:<invoice id> » → project id, « - » for none
	Admin      *deadlines.Settings `json:"admin,omitempty"`
	Done       map[string]bool     `json:"done"`           // administrative due dates ticked as done
	Lang       string              `json:"lang,omitempty"` // the language chosen in the app; empty: the Mac's
	Notify     *Notify             `json:"notify,omitempty"`
	Privacy    *Privacy            `json:"privacy,omitempty"`
	Onboarding *Onboarding         `json:"onboarding,omitempty"`
}

// Notify says which reminders the Mac app shows, and at what hour.
type Notify struct {
	Payments  bool `json:"payments"`  // the day before a payment
	Deadlines bool `json:"deadlines"` // 7 days and 1 day before an administrative deadline
	Missing   bool `json:"missing"`   // every Monday while invoices are missing
	Reserve   bool `json:"reserve"`   // 30, 14, 7 and 1 days before the reserve runs out
	Hour      int  `json:"hour"`      // 0 to 23
}

// Privacy says what the app may download: the ECB's rates, the brands' icons.
type Privacy struct {
	Rates bool `json:"rates"`
	Logos bool `json:"logos"`
}

// NotifySettings are the reminders chosen; all of them at 9:00 until the user chooses.
func (d DB) NotifySettings() Notify {
	if d.Notify != nil {
		return *d.Notify
	}
	return Notify{Payments: true, Deadlines: true, Missing: true, Reserve: true, Hour: 9}
}

// PrivacySettings are the downloads allowed; both until the user turns one off.
func (d DB) PrivacySettings() Privacy {
	if d.Privacy != nil {
		return *d.Privacy
	}
	return Privacy{Rates: true, Logos: true}
}

// Fill gives every list and map a value: the front reads [] and {}, never null.
func Fill(d *DB) {
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

// Settings are the administrative due dates that apply; the defaults until the user chooses.
func (d DB) Settings() deadlines.Settings {
	if d.Admin != nil {
		return *d.Admin
	}
	return deadlines.Default()
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

// InvoiceName is the vendor its invoices come from.
func (p Payment) InvoiceName() string {
	if p.Vendor != "" {
		return p.Vendor
	}
	return p.Name
}

// billingURL is the link typed on the payment, or the known page of its vendor.
func (p Payment) billingURL(vendors *vendor.Catalog) string {
	if p.Billing != "" {
		return p.Billing
	}
	return vendors.BillingPage(p.Domain, p.InvoiceName())
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

// apply returns e with the corrected fields; a corrected vendor brings its known domain.
func (o Override) apply(e extract.Extracted, vendors *vendor.Catalog) extract.Extracted {
	if o.Kind != "" {
		e.Kind = o.Kind
	}
	if o.Vendor != "" {
		e.Vendor = o.Vendor
		e.Domain = vendors.Domain(o.Vendor)
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
