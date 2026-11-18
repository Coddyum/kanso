package extract

import (
	"testing"

	"github.com/coddyum/kanso/internal/vendor"
)

// Every text below is made up: no real invoice, no real customer.

const stripeLike = `Invoice
Invoice number ABCD-0042
Date of issue November 3, 2026
Exemple Cloud SAS
Description Qty Unit price Amount
Pro plan
€21.60 paid on November 3, 2026
Oct 3–Nov 3, 2026
Total €21.60
Amount paid €21.60`

const frenchReceipt = `Reçu de paiement
N° de facture : FA-2026-118
Date de paiement : 12/10/2026
Montant payé : 1 234,50 €`

func TestExtract(t *testing.T) {
	cat := vendor.NewCatalog()
	e := Extract(cat, "2026/Cloudflare/Novembre/facture.pdf", stripeLike)
	if e.Kind != "invoice" || e.Vendor != "Cloudflare" || e.Domain != "cloudflare.com" {
		t.Errorf("kind/vendor = %q %q %q", e.Kind, e.Vendor, e.Domain)
	}
	if e.Number != "ABCD-0042" || e.Date != "2026-11-03" || e.PeriodDays != 31 || e.Label != "Pro plan" {
		t.Errorf("number/date/period/label = %q %q %d %q", e.Number, e.Date, e.PeriodDays, e.Label)
	}
	if e.Amount == nil || *e.Amount != 21.60 || e.Currency != "EUR" {
		t.Errorf("amount = %v %s", e.Amount, e.Currency)
	}

	r := Extract(cat, "Divers/recu.pdf", frenchReceipt)
	if r.Kind != "receipt" || r.Ref != "FA-2026-118" || r.Number != "" {
		t.Errorf("receipt kind/ref/number = %q %q %q", r.Kind, r.Ref, r.Number)
	}
	if r.Date != "2026-10-12" || r.Amount == nil || *r.Amount != 1234.50 {
		t.Errorf("receipt date/amount = %q %v", r.Date, r.Amount)
	}
}

func TestDetectKind(t *testing.T) {
	cases := []struct {
		lines []string
		name  string
		text  string
		want  string
	}{
		{[]string{"FACTURE"}, "x.pdf", "", "invoice"},
		{[]string{"Receipt"}, "x.pdf", "", "receipt"},
		{[]string{"Relevé de compte"}, "x.pdf", "", "statement"},
		{[]string{"Bon de commande"}, "x.pdf", "", "order"},
		{[]string{"Bonjour"}, "Facture octobre.pdf", "", "invoice"},
		{[]string{"Bonjour"}, "contrat.pdf", "la plateforme de réception des factures", "document"},
		{[]string{"Bonjour"}, "x.pdf", "Date de facture : 01/02/2026", "invoice"},
		// A title under an address block: short line among the first 15.
		{[]string{"Société", "1 rue", "75000 Ville", "Reçu de paiement"}, "x.pdf", "", "receipt"},
	}
	for _, c := range cases {
		if got := detectKind(c.lines, c.name, c.text); got != c.want {
			t.Errorf("detectKind(%q, %q) = %q, want %q", c.lines, c.name, got, c.want)
		}
	}
}

func TestFirstDate(t *testing.T) {
	cases := map[string]string{
		"payé le 1er octobre 2026":             "2026-10-01",
		"Paid Nov 21, 2026":                    "2026-11-21",
		"le 07/03/2026":                        "2026-03-07",
		"2026-02-14":                           "2026-02-14",
		"le 31/02/2026 puis le 01/03/2026":     "2026-03-01",
		"rien ici":                             "",
		"12 août 2026, payé depuis 2026-08-01": "2026-08-12",
	}
	for in, want := range cases {
		if got := firstDate(in); got != want {
			t.Errorf("firstDate(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseNumber(t *testing.T) {
	cases := map[string]float64{
		"1 234,50": 1234.50,
		"1,234.50": 1234.50,
		"21.6":     21.6,
		"7,40":     7.40,
		"1.000":    1000,
		"1 500,00": 1500,
	}
	for in, want := range cases {
		if got, ok := parseNumber(in); !ok || got != want {
			t.Errorf("parseNumber(%q) = %v, %v; want %v", in, got, ok, want)
		}
	}
}

func TestDetectAmount(t *testing.T) {
	cases := []struct {
		text string
		want float64
		cur  string
	}{
		{"Total TTC : 120,00 €", 120, "EUR"},
		{"Amount due $8.20 USD", 8.20, "USD"},
		{"Montant prélevé le 01/11 7,40", 7.40, "EUR"},
		{"Total articles 3\nTotal £12.00", 12, "GBP"},
		{"€19.08 due December 1, 2026", 19.08, "EUR"},
		{"49,00 € prélevés le 5 novembre", 49, "EUR"},
	}
	for _, c := range cases {
		lines := splitLines(c.text)
		got, cur, ok := detectAmount(lines, c.text)
		if !ok || got != c.want || cur != c.cur {
			t.Errorf("detectAmount(%q) = %v %s %v; want %v %s", c.text, got, cur, ok, c.want, c.cur)
		}
	}
}

func TestDetectPeriod(t *testing.T) {
	cases := map[string]int{
		"Oct 21–Nov 21, 2026":             31,
		"Dec 15, 2026 - Jan 15, 2027":     31,
		"Nov 1 – 30, 2026":                29,
		"1 janvier 2026 - 1 janvier 2027": 365,
		"du 3 mars 2026 – 3 avril 2026":   31,
		"rien":                            0,
	}
	for in, want := range cases {
		if got := detectPeriod(in); got != want {
			t.Errorf("detectPeriod(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestDetectVendor(t *testing.T) {
	cat := vendor.NewCatalog()
	cases := []struct {
		rel, text, want string
	}{
		{"2026/OVH/Mars/x.pdf", "", "OVHcloud"},
		{"Clients/Atelier Dupuis/2026/x.pdf", "", "Atelier Dupuis"},
		{"2026/Octobre/x.pdf", "Thanks for using Render. Render Services Inc.", "Render"},
		{"x.pdf", "", ""},
	}
	for _, c := range cases {
		if got, _ := detectVendor(cat, c.rel, c.text); got != c.want {
			t.Errorf("detectVendor(%q) = %q, want %q", c.rel, got, c.want)
		}
	}
}
