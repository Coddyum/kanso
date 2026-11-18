package ledger

import (
	"sort"
	"strings"

	"github.com/coddyum/kanso/internal/day"
	"github.com/coddyum/kanso/internal/i18n"
	"github.com/coddyum/kanso/internal/norm"
	"github.com/coddyum/kanso/internal/rates"
)

// Project files expenses together: an app, a client, the company itself.
type Project struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Color    string   `json:"color"`              // a palette name the front knows: violet, sky, emerald…
	Keywords []string `json:"keywords,omitempty"` // besides its name, words that file an expense here on their own
}

var projectColors = []string{"violet", "sky", "emerald", "amber", "rose", "teal", "orange", "indigo", "pink", "lime", "fuchsia", "blue"}

// Unfiled is the assignment that keeps an expense out of every project, even when a keyword would file it.
const Unfiled = "-"

// Spend is one amount of the analytics: an invoice, a past due date whose
// invoice is not in the folder, or a due date still to come.
type Spend struct {
	Date      string  `json:"date"`
	Service   string  `json:"service"` // the row it adds up in: the payment's name, or the vendor of an invoice no payment claims
	Domain    string  `json:"domain,omitempty"`
	Label     string  `json:"label,omitempty"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
	EUR       float64 `json:"eur"`
	Kind      string  `json:"kind"`            // invoice | estimate | planned
	Cycle     string  `json:"cycle,omitempty"` // the payment's cycle; empty for an invoice no payment claims
	PaymentID string  `json:"paymentId,omitempty"`
	InvoiceID string  `json:"invoiceId,omitempty"`
	Project   string  `json:"project"`   // project id; empty when not filed
	ProjectBy string  `json:"projectBy"` // invoice | service | keyword, or empty
	match     string
}

// Analytics is every amount of the analytics screen.
type Analytics struct {
	Today    string             `json:"today"`
	Projects []Project          `json:"projects"`
	Spend    []Spend            `json:"spend"`
	NoAmount int                `json:"noAmount"` // invoices whose amount could not be read: left out
	NoRate   int                `json:"noRate"`   // amounts in a currency with no known rate: left out
	PerEUR   map[string]float64 `json:"perEUR,omitempty"`
	RateDate string             `json:"rateDate,omitempty"`
}

// ServiceKey is how a service is named in the assignments: « Cloudflare · example.com » → « cloudflare · example.com ».
func ServiceKey(name string) string { return strings.Join(strings.Fields(strings.ToLower(name)), " ") }

// ProjectOf files an expense: the invoice's own choice, then its service's,
// then the first project whose name or keyword appears in its service, its
// match or its label. An assignment to a deleted project is ignored.
func ProjectOf(db DB, s Spend) (id, by string) {
	known := func(p string) bool { return p == Unfiled || hasProject(db.Projects, p) }
	choices := []struct{ key, by string }{{"svc:" + ServiceKey(s.Service), "service"}}
	if s.InvoiceID != "" {
		choices = append([]struct{ key, by string }{{"inv:" + s.InvoiceID, "invoice"}}, choices...)
	}
	for _, k := range choices {
		if p := db.Assign[k.key]; p != "" && known(p) {
			if p == Unfiled {
				return "", k.by
			}
			return p, k.by
		}
	}
	hay := norm.Words(s.Service + " " + s.match + " " + s.Label)
	for _, pr := range db.Projects {
		for _, kw := range append([]string{pr.Name}, pr.Keywords...) {
			if w := norm.Words(kw); strings.TrimSpace(w) != "" && strings.Contains(hay, w) {
				return pr.ID, "keyword"
			}
		}
	}
	return "", ""
}

func hasProject(list []Project, id string) bool {
	for _, p := range list {
		if p.ID == id {
			return true
		}
	}
	return false
}

// Analytics lists every amount from three years back to one year ahead, in
// euros: the invoices, the past due dates with no invoice at their estimate,
// and the due dates to come. Skipped due dates and zero amounts are left out.
func (m *Model) Analytics(r rates.Rates, tr i18n.Lang) Analytics {
	res := Analytics{Today: m.Today.Format(day.Format), Projects: m.DB.Projects, Spend: []Spend{}, RateDate: r.Date, PerEUR: r.PerEUR}
	add := func(s Spend) {
		if s.Amount == 0 {
			return
		}
		eur, ok := r.ToEUR(s.Amount, s.Currency)
		if !ok {
			res.NoRate++
			return
		}
		s.EUR = round2(eur)
		s.Project, s.ProjectBy = ProjectOf(m.DB, s)
		res.Spend = append(res.Spend, s)
	}
	for _, p := range m.DB.Payments {
		est := m.estimate(p)
		for _, d := range p.Dues(m.Today.AddDate(-3, 0, 0), m.Today.AddDate(1, 0, 0)) {
			s := Spend{
				Date: d.Format(day.Format), Service: p.Name, Domain: p.Domain, Cycle: p.Cycle, PaymentID: p.ID,
				Amount: est.Amount, Currency: est.Currency, Kind: "estimate", match: p.Match,
			}
			switch st := m.dueStatus(p, d); st.Status {
			case "skipped":
				continue
			case "upcoming", "today":
				s.Kind = "planned"
			case "matched":
				if inv := m.byID[st.InvoiceID]; inv != nil && inv.Amount != nil {
					s.Date, s.Label, s.InvoiceID = inv.Date, inv.Label, inv.ID
					s.Amount, s.Currency, s.Kind = *inv.Amount, inv.Currency, "invoice"
				}
			}
			add(s)
		}
	}
	claimed := m.claimed()
	for _, inv := range m.Invoices {
		if claimed[inv.ID] {
			continue
		}
		if inv.Amount == nil {
			res.NoAmount++
			continue
		}
		name := inv.Vendor
		if name == "" {
			name = tr.T("common.unknownVendor")
		}
		add(Spend{
			Date: inv.Date, Service: name, Domain: inv.Domain, Label: inv.Label,
			Amount: *inv.Amount, Currency: inv.Currency, Kind: "invoice", InvoiceID: inv.ID,
		})
	}
	sort.SliceStable(res.Spend, func(i, j int) bool { return res.Spend[i].Date < res.Spend[j].Date })
	return res
}
