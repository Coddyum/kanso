package ledger

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coddyum/kanso/internal/day"
	"github.com/coddyum/kanso/internal/extract"
	"github.com/coddyum/kanso/internal/folder"
	"github.com/coddyum/kanso/internal/i18n"
	"github.com/coddyum/kanso/internal/rates"
	"github.com/coddyum/kanso/internal/vendor"
)

// rateless knows no rate: every amount of these tests is in euros.
var rateless = rates.Rates{}

// Every vendor, amount and file below is made up.

func d(s string) time.Time {
	t, ok := day.Parse(s)
	if !ok {
		panic(s)
	}
	return t
}

func amount(v float64) *float64 { return &v }

func doc(path, kind, vendorName, date string, amt float64, extra func(*extract.Extracted)) folder.Doc {
	info := extract.Extracted{Kind: kind, Vendor: vendorName, Date: date, Amount: amount(amt), Currency: "EUR"}
	if extra != nil {
		extra(&info)
	}
	return folder.Doc{Path: path, Name: filepath.Base(path), Dir: filepath.Dir(path), Hash: "h-" + path, ModTime: d(date), Info: info}
}

func TestDues(t *testing.T) {
	cases := []struct {
		p        Payment
		from, to string
		want     []string
	}{
		{Payment{Start: "2026-01-31", Cycle: "monthly"}, "2026-01-01", "2026-04-30", []string{"2026-01-31", "2026-02-28", "2026-03-31", "2026-04-30"}},
		{Payment{Start: "2026-03-10", Cycle: "once"}, "2026-01-01", "2026-12-31", []string{"2026-03-10"}},
		{Payment{Start: "2026-01-05", Cycle: "quarterly", End: "2026-08-01"}, "2026-01-01", "2026-12-31", []string{"2026-01-05", "2026-04-05", "2026-07-05"}},
		{Payment{Start: "2024-02-29", Cycle: "yearly"}, "2025-01-01", "2026-12-31", []string{"2025-02-28", "2026-02-28"}},
		{Payment{Start: "2026-06-01", Cycle: "weekly"}, "2026-06-10", "2026-06-23", []string{"2026-06-15", "2026-06-22"}},
		{Payment{Start: "pas une date", Cycle: "monthly"}, "2026-01-01", "2026-12-31", nil},
	}
	for _, c := range cases {
		var got []string
		for _, t := range c.p.Dues(d(c.from), d(c.to)) {
			got = append(got, t.Format(day.Format))
		}
		if len(got) != len(c.want) {
			t.Errorf("%+v: got %v, want %v", c.p, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%+v: got %v, want %v", c.p, got, c.want)
				break
			}
		}
	}
}

func TestMatchDues(t *testing.T) {
	inv := func(id, vendorName, date string, amt float64, label string) Invoice {
		return Invoice{ID: id, Vendor: vendorName, Date: date, Amount: amount(amt), Currency: "EUR", Label: label, Files: []FileRef{{Hash: "f-" + id, Name: id + ".pdf"}}}
	}
	payments := []Payment{
		{ID: "hosting", Name: "Nuage", Amount: 10, Cycle: "monthly", Start: "2026-08-24", ExpectInvoice: true},
		{ID: "domain", Name: "Nuage · exemple.fr", Vendor: "Nuage", Match: "exemple.fr", Amount: 12, Cycle: "yearly", Start: "2026-09-02", ExpectInvoice: true},
		{ID: "phone", Name: "Téléphone", Amount: 20, Cycle: "monthly", Start: "2026-09-05", ExpectInvoice: true},
	}
	invoices := []Invoice{
		inv("free", "Nuage", "2026-09-24", 0, "storage, free tier"),
		inv("sep", "Nuage", "2026-09-25", 10, "hosting"),
		inv("oct", "Nuage", "2026-10-30", 10, "hosting"), // 6 days after the due date: still its invoice
		inv("dom", "Nuage", "2026-09-02", 12, "Registration exemple.fr (1 yr)"),
		inv("late", "Téléphone", "2026-10-05", 20, ""), // dropped for september in the app
	}
	links := map[string]string{"phone@2026-09-05": "f-late"}
	got := matchDues(payments, invoices, links, d("2026-11-01"))
	want := map[string]string{
		"hosting@2026-08-24": "",    // nothing within 10 days before or 25 days after
		"hosting@2026-09-24": "sep", // the free invoice of the same day never pays 10 €
		"hosting@2026-10-24": "oct",
		"domain@2026-09-02":  "dom", // the invoice carrying its Match is the domain's, not the hosting's
		"phone@2026-09-05":   "late",
		"phone@2026-10-05":   "", // the dropped file pays one due date only
	}
	for key, id := range want {
		if got[key].InvoiceID != id {
			t.Errorf("%s: got %q, want %q", key, got[key].InvoiceID, id)
		}
	}
}

func TestGroupInvoices(t *testing.T) {
	withNumber := func(n string) func(*extract.Extracted) { return func(e *extract.Extracted) { e.Number = n } }
	withRef := func(n string) func(*extract.Extracted) { return func(e *extract.Extracted) { e.Ref = n } }
	docs := []folder.Doc{
		doc("2026/Nuage/facture.pdf", "invoice", "Nuage", "2026-10-03", 21.6, withNumber("N-1")),
		doc("2026/Nuage/recu.pdf", "receipt", "Nuage", "2026-10-03", 21.6, withRef("N-1")),
		doc("2026/Nuage/facture copie.pdf", "invoice", "Nuage", "2026-10-03", 21.6, withNumber("N-1")),
		doc("2026/Boutique/commande.pdf", "order", "Boutique", "2026-09-10", 30, nil),
		doc("2026/Boutique/releve.pdf", "statement", "Boutique", "2026-09-11", 30, nil),
		doc("2026/Divers/notice.pdf", "document", "", "2026-09-01", 0, nil),
	}
	got := groupInvoices(docs, nil)
	if len(got) != 2 {
		t.Fatalf("got %d invoices, want 2: %+v", len(got), got)
	}
	if got[0].Vendor != "Nuage" || len(got[0].Files) != 3 || got[0].Files[0].Kind != "invoice" {
		t.Errorf("invoice, its copy and its receipt make one expense: %+v", got[0])
	}
	if got[1].Vendor != "Boutique" || len(got[1].Files) != 2 {
		t.Errorf("an order and the statement next to it make one expense: %+v", got[1])
	}
}

func TestSuggestions(t *testing.T) {
	var invoices []Invoice
	for _, date := range []string{"2026-08-12", "2026-09-12", "2026-10-12"} {
		invoices = append(invoices, Invoice{ID: date, Vendor: "Nuage", Date: date, Amount: amount(5), Currency: "EUR"})
	}
	invoices = append(invoices, Invoice{ID: "dom", Vendor: "Nuage", Date: "2026-03-02", Amount: amount(12), Currency: "EUR", Label: "Renewal exemple.fr (1 yr)"})
	got := suggest(invoices, nil, map[string]bool{}, d("2026-11-01"))
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	if got[0].Name != "Nuage" || got[0].Cycle != "monthly" || got[0].Start != "2026-08-12" || !got[0].Active {
		t.Errorf("monthly subscription: %+v", got[0])
	}
	if got[1].Name != "Nuage · exemple.fr" || got[1].Cycle != "yearly" || got[1].Match != "exemple.fr" {
		t.Errorf("yearly line: %+v", got[1])
	}
	covered := []Payment{{Name: "Nuage"}}
	if again := suggest(invoices, covered, map[string]bool{"suggest:nuage:exemplefr": true}, d("2026-11-01")); len(again) != 0 {
		t.Errorf("a covered vendor and a dismissed line are not suggested: %+v", again)
	}
}

func TestProjectOf(t *testing.T) {
	db := DB{
		Projects: []Project{{ID: "app", Name: "Mon app", Keywords: []string{"monapp.fr"}}, {ID: "client", Name: "Client A"}},
		Assign:   map[string]string{"svc:nuage": "client", "inv:42": Unfiled, "svc:old": "deleted"},
	}
	cases := []struct {
		s      Spend
		id, by string
	}{
		{Spend{Service: "Nuage", InvoiceID: "42"}, "", "invoice"},
		{Spend{Service: "Nuage"}, "client", "service"},
		{Spend{Service: "Hébergeur", match: "monapp.fr"}, "app", "keyword"},
		{Spend{Service: "Old"}, "", ""},
	}
	for _, c := range cases {
		if id, by := ProjectOf(db, c.s); id != c.id || by != c.by {
			t.Errorf("ProjectOf(%+v) = %q %q, want %q %q", c.s, id, by, c.id, c.by)
		}
	}
}

// book builds a model on made-up documents, in a database of the test's own.
func book(t *testing.T, db DB, docs []folder.Doc, today string) *Model {
	t.Helper()
	b, err := Open(filepath.Join(t.TempDir(), "db.json"), vendor.NewCatalog())
	if err != nil {
		t.Fatal(err)
	}
	if err := b.store.Update(func(d *DB) error { *d = db; Fill(d); return nil }); err != nil {
		t.Fatal(err)
	}
	m, err := b.Model(docs, d(today))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestEstimateAndForecast(t *testing.T) {
	var docs []folder.Doc
	for i, amt := range []float64{10, 14, 12} {
		date := d("2026-08-01").AddDate(0, i, 0).Format(day.Format)
		docs = append(docs, doc("2026/Nuage/"+date+".pdf", "invoice", "Nuage", date, amt, nil))
	}
	db := DB{
		Payments: []Payment{{ID: "p", Name: "Nuage", Amount: 9, Currency: "EUR", Cycle: "monthly", Start: "2026-08-01", ExpectInvoice: true}},
		Reserve:  &Reserve{Amount: 40, Date: "2026-10-15"},
	}
	m := book(t, db, docs, "2026-10-20")
	est := m.estimate(m.DB.Payments[0])
	if est.Amount != 12 || !est.Variable || est.Prudent != 14 {
		t.Errorf("estimate = %+v: the latest invoice, variable, prudent at the highest", est)
	}
	f := m.Forecast(rateless)
	if !f.Set || f.RunsOut != "2027-01-01" || f.PerMonth != 12 {
		t.Errorf("forecast = runsOut %q perMonth %v; want 2027-01-01 (40 - 3×14 < 0) and 12", f.RunsOut, f.PerMonth)
	}
}

func TestMissingAndReminders(t *testing.T) {
	db := DB{Payments: []Payment{
		{ID: "a", Name: "Nuage", Amount: 5, Currency: "EUR", Cycle: "monthly", Start: "2026-10-01", ExpectInvoice: true},
		{ID: "b", Name: "Banque", Amount: 3, Currency: "EUR", Cycle: "monthly", Start: "2026-10-01", ExpectInvoice: false},
	}}
	m := book(t, db, nil, "2026-11-18")
	st := m.State(rateless, french)
	if len(st.Missing) != 2 || st.Missing[0].Date != "2026-11-01" {
		t.Errorf("missing = %+v: two due dates of Nuage, newest first; Banque expects no invoice", st.Missing)
	}
	rs := m.Reminders(rateless, time.Date(2026, 11, 18, 12, 0, 0, 0, time.Local), french)
	var titles []string
	for _, r := range rs {
		titles = append(titles, r.At+" "+r.Title)
	}
	wantFirst := []string{
		"2026-11-23T09:00 2 factures à récupérer",
		"2026-11-30T09:00 Demain : 2 paiements",
		"2026-11-30T09:00 2 factures à récupérer",
	}
	for i, w := range wantFirst {
		if i >= len(titles) || titles[i] != w {
			t.Fatalf("reminders = %q, want them to start with %q", titles, wantFirst)
		}
	}
}

func TestEditsRefuse(t *testing.T) {
	b, err := Open(filepath.Join(t.TempDir(), "db.json"), vendor.NewCatalog())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.SavePayment(Payment{Name: "x"}, false); !isInvalid(err) {
		t.Errorf("a payment with no cycle or date: %v", err)
	}
	if _, err := b.SaveProject(Project{Name: "A"}); err != nil {
		t.Fatal(err)
	}
	saved := b.Version()
	if _, err := b.SaveProject(Project{Name: "a"}); !isInvalid(err) {
		t.Errorf("two projects with the same name: %v", err)
	}
	if err := b.Assign("service", "Nuage", "inconnu"); !isInvalid(err) {
		t.Errorf("assign to an unknown project: %v", err)
	}
	if v := b.Version(); v != saved {
		t.Errorf("refused edits must write nothing: version %d, want %d", v, saved)
	}
	p, err := b.SavePayment(Payment{Name: "Nuage", Cycle: "monthly", Start: "2026-01-01", Amount: 5}, false)
	if err != nil || p.ID == "" || p.Currency != "EUR" || p.Domain != "" {
		t.Errorf("saved payment = %+v, %v", p, err)
	}
	if err := b.DeletePayment(p.ID); err != nil {
		t.Fatal(err)
	}
	if db := b.store.Snapshot(); len(db.Payments) != 0 || !db.Dismissed["suggest:nuage"] {
		t.Errorf("a deleted payment is gone and its detection dismissed: %+v", db)
	}
}

func isInvalid(err error) bool {
	_, ok := err.(Invalid)
	return ok
}

// french is the French of the real language files.
var french = func() i18n.Lang {
	c, err := i18n.Load(os.DirFS("../../web"))
	if err != nil {
		panic(err)
	}
	return c.Lang("fr")
}()
