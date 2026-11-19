// Package extract reads what a document says about itself — its kind, vendor,
// date, amount, number and billed period — from its path and its text.
package extract

import (
	"path"
	"regexp"
	"strings"

	"github.com/coddyum/kanso/internal/norm"
	"github.com/coddyum/kanso/internal/vendor"
)

// Extracted is what a document says about itself.
type Extracted struct {
	Kind     string   `json:"kind"` // invoice | receipt | order | statement | document
	Vendor   string   `json:"vendor,omitempty"`
	Domain   string   `json:"domain,omitempty"`
	Number   string   `json:"number,omitempty"`
	Ref      string   `json:"ref,omitempty"` // a receipt: the invoice number it pays
	Date     string   `json:"date,omitempty"`
	Amount   *float64 `json:"amount,omitempty"`
	Currency string   `json:"currency,omitempty"`
	Label    string   `json:"label,omitempty"`
	// PeriodDays is the length of the billed period when the document writes
	// one (« Oct 21–Nov 21, 2026 » → 31): a single invoice then tells its cycle.
	PeriodDays int `json:"periodDays,omitempty"`
}

// Extract reads what it can from a file's path (relative, slash-separated)
// and its text (a PDF's, or what Vision read in a scan or a photo; empty otherwise).
func Extract(vendors *vendor.Catalog, rel, text string) Extracted {
	lines := splitLines(text)
	e := Extracted{Kind: detectKind(lines, path.Base(rel), text)}
	e.Vendor, e.Domain = detectVendor(vendors, rel, text)
	if text == "" {
		return e
	}
	if m := reInvoiceNo.FindStringSubmatch(text); m != nil {
		e.Number = m[1]
	}
	if e.Kind == "receipt" {
		e.Ref, e.Number = e.Number, ""
		if m := reReceiptNo.FindStringSubmatch(text); m != nil {
			e.Number = m[1]
		}
	}
	e.Date = detectDate(text)
	if v, cur, ok := detectAmount(lines, text); ok {
		e.Amount, e.Currency = &v, cur
	}
	e.Label = detectLabel(lines)
	e.PeriodDays = detectPeriod(text)
	return e
}

var (
	reInvoiceNo = regexp.MustCompile(`(?i)(?:invoice number|invoice no\.?|numéro de facture|n° de facture|bill reference:?)\s*:?\s*([a-z0-9][a-z0-9_-]{3,})`)
	reReceiptNo = regexp.MustCompile(`(?i)receipt number\s*:?\s*([a-z0-9][a-z0-9-]{3,})`)
)

func splitLines(text string) []string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func hasAny(s string, subs ...string) bool {
	for _, x := range subs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}

func startsAny(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// detectKind reads the title of the document: one of its first 3 lines, or a
// short line among the first 15 (a title under the address block, as on the
// receipts of the INPI), then its file name, then a few phrases of its text.
// A word in a sentence is not enough: « plateforme de réception des factures »
// is not an invoice.
func detectKind(lines []string, name, text string) string {
	for i, l := range lines[:min(15, len(lines))] {
		if i >= 3 && len([]rune(l)) > 48 {
			continue
		}
		ll := strings.ToLower(l)
		switch {
		case startsAny(ll, "receipt", "reçu", "recu de paiement"):
			return "receipt"
		case startsAny(ll, "relevé", "transaction statement", "account statement"):
			return "statement"
		case startsAny(ll, "purchase order", "bon de commande"):
			return "order"
		case startsAny(ll, "invoice", "facture", "tax invoice", "bill reference"):
			return "invoice"
		}
	}
	n := norm.Words(name)
	switch {
	case hasAny(n, " receipt ", " recu "):
		return "receipt"
	case hasAny(n, " invoice ", " facture ", " bill "):
		return "invoice"
	case hasAny(n, " order ", " commande "):
		return "order"
	case hasAny(n, " transaction statement ", " releve "):
		return "statement"
	}
	if hasAny(strings.ToLower(text), "invoice number", "numéro de facture", "n° de facture", "date de facture", "invoice date", "reçu de paiement", "recu de paiement", "vaudra facture") {
		return "invoice"
	}
	return "document"
}

func detectLabel(lines []string) string {
	header := func(s string) bool {
		return hasAny(strings.ToLower(s), "qty", "quantit", "qté", "unit", "amount", "montant", "prix", "rate")
	}
	for i, l := range lines {
		ll := strings.ToLower(l)
		if !strings.HasPrefix(ll, "description") {
			continue
		}
		if rest := strings.TrimSpace(l[len("description"):]); rest != "" && !header(rest) {
			return norm.Clip(rest, 48)
		}
		// The line under « Description »; a header's second line read by Vision (« (Incl. VAT) ») is skipped.
		for _, n := range lines[i+1 : min(i+3, len(lines))] {
			nl := strings.ToLower(n)
			if strings.HasPrefix(n, "(") {
				continue
			}
			if !startsWithMoney(n) && !hasAny(nl, "subtotal", "sous-total", "total") && !header(n) {
				// Vision writes a table's cells on one line, two spaces apart: the label is the first.
				name, _, _ := strings.Cut(n, "  ")
				return norm.Clip(name, 48)
			}
			break
		}
	}
	return ""
}
