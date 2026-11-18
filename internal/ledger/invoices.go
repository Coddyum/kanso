package ledger

import (
	"math"
	"sort"
	"time"

	"github.com/coddyum/kanso/internal/day"
	"github.com/coddyum/kanso/internal/extract"
	"github.com/coddyum/kanso/internal/folder"
	"github.com/coddyum/kanso/internal/norm"
)

// FileRef is one file of an invoice: the invoice itself, its receipt…
type FileRef struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	Hash string `json:"hash"`
}

// Invoice is one expense, whatever the number of files that prove it.
type Invoice struct {
	ID       string    `json:"id"` // hash of the main file
	Vendor   string    `json:"vendor"`
	Domain   string    `json:"domain,omitempty"`
	Label    string    `json:"label,omitempty"`
	Number   string    `json:"number,omitempty"`
	Date     string    `json:"date"`
	Amount   *float64  `json:"amount"`
	Currency string    `json:"currency"`
	Folder   string    `json:"folder"`
	Files    []FileRef `json:"files"`
	Edited   bool      `json:"edited,omitempty"`
	// PeriodDays: length of the billed period, when one of its files writes it.
	PeriodDays int `json:"periodDays,omitempty"`
	// Arrived: when its newest file landed in the folder (its modification time).
	Arrived time.Time `json:"arrived"`
}

var billingRank = map[string]int{"invoice": 0, "order": 1, "receipt": 2, "statement": 3}

type bundle struct{ docs []folder.Doc }

func docDay(d folder.Doc) string {
	if d.Info.Date != "" {
		return d.Info.Date
	}
	return d.ModTime.Format(day.Format)
}

// groupInvoices merges the files that prove the same expense: a receipt with
// the invoice it names, the same invoice saved twice (same content or same
// number), an order or a bank statement next to it in the same folder.
// Newest first; documents that bill nothing are left out.
func groupInvoices(docs []folder.Doc, edited map[string]bool) []Invoice {
	var billing []folder.Doc
	for _, d := range docs {
		if _, ok := billingRank[d.Info.Kind]; ok {
			billing = append(billing, d)
		}
	}
	sort.SliceStable(billing, func(i, j int) bool {
		ri, rj := billingRank[billing[i].Info.Kind], billingRank[billing[j].Info.Kind]
		if ri != rj {
			return ri < rj
		}
		return billing[i].Path < billing[j].Path
	})
	var groups []*bundle
	byNumber := map[string]*bundle{}
	byHash := map[string]*bundle{}
	for _, d := range billing {
		key := func(n string) string { return norm.Key(d.Info.Vendor) + "#" + n }
		b := byHash[d.Hash]
		if b == nil && d.Info.Kind == "invoice" && d.Info.Number != "" {
			b = byNumber[key(d.Info.Number)]
		}
		if b == nil && d.Info.Kind == "receipt" && d.Info.Ref != "" {
			b = byNumber[key(d.Info.Ref)]
		}
		if b == nil && d.Info.Kind != "invoice" {
			b = sibling(groups, d)
		}
		if b == nil {
			b = &bundle{}
			groups = append(groups, b)
		}
		b.docs = append(b.docs, d)
		byHash[d.Hash] = b
		if d.Info.Kind == "invoice" && d.Info.Number != "" && byNumber[key(d.Info.Number)] == nil {
			byNumber[key(d.Info.Number)] = b
		}
	}
	out := make([]Invoice, 0, len(groups))
	for _, b := range groups {
		out = append(out, b.invoice(edited))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date > out[j].Date
		}
		return out[i].Vendor < out[j].Vendor
	})
	return out
}

// sibling is the group d completes: same folder and vendor, a kind it lacks,
// within 3 days and at the same amount.
func sibling(groups []*bundle, d folder.Doc) *bundle {
	for _, b := range groups {
		p := b.docs[0]
		if p.Dir != d.Dir || norm.Key(p.Info.Vendor) != norm.Key(d.Info.Vendor) || b.has(d.Info.Kind) {
			continue
		}
		if day.Gap(docDay(p), docDay(d)) > 3 || !amountsAgree(p.Info, d.Info) {
			continue
		}
		return b
	}
	return nil
}

func (b *bundle) has(kind string) bool {
	for _, d := range b.docs {
		if d.Info.Kind == kind {
			return true
		}
	}
	return false
}

func amountsAgree(a, b extract.Extracted) bool {
	if a.Amount == nil || b.Amount == nil || a.Currency != b.Currency {
		return true
	}
	return math.Abs(*a.Amount-*b.Amount) < 0.01
}

// invoice reads the group's main file, completed by the others where it is silent.
func (b *bundle) invoice(edited map[string]bool) Invoice {
	p := b.docs[0]
	inv := Invoice{
		ID: p.Hash, Vendor: p.Info.Vendor, Domain: p.Info.Domain, Label: p.Info.Label,
		Number: p.Info.Number, Date: p.Info.Date, Amount: p.Info.Amount, Currency: p.Info.Currency,
		Folder: p.Dir, Edited: edited[p.Hash], PeriodDays: p.Info.PeriodDays,
	}
	for _, d := range b.docs[1:] {
		if inv.Amount == nil && d.Info.Amount != nil {
			inv.Amount, inv.Currency = d.Info.Amount, d.Info.Currency
		}
		if inv.Date == "" {
			inv.Date = d.Info.Date
		}
		if inv.Label == "" {
			inv.Label = d.Info.Label
		}
		if inv.Number == "" {
			inv.Number = d.Info.Ref
		}
		if inv.PeriodDays == 0 {
			inv.PeriodDays = d.Info.PeriodDays
		}
	}
	if inv.Date == "" {
		inv.Date = p.ModTime.Format(day.Format)
	}
	if inv.Currency == "" {
		inv.Currency = "EUR"
	}
	for _, d := range b.docs {
		inv.Files = append(inv.Files, FileRef{Path: d.Path, Name: d.Name, Kind: d.Info.Kind, Hash: d.Hash})
		if d.ModTime.After(inv.Arrived) {
			inv.Arrived = d.ModTime
		}
	}
	return inv
}
