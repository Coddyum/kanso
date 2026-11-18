package ledger

import (
	"sort"
	"strings"

	"github.com/coddyum/kanso/internal/day"
	"github.com/coddyum/kanso/internal/deadlines"
	"github.com/coddyum/kanso/internal/rates"
)

// PaymentView is a payment with what it costs now.
type PaymentView struct {
	Payment
	Current         float64   `json:"current"`
	CurrentCurrency string    `json:"currentCurrency"`
	Variable        bool      `json:"variable"`
	Recent          []float64 `json:"recent,omitempty"`
	Manual          bool      `json:"manual"`
	LastInvoice     string    `json:"lastInvoice,omitempty"` // date of the latest invoice paired with one of its due dates
	BillingURL      string    `json:"billingUrl,omitempty"`  // where to fetch its invoices
	Project         string    `json:"project"`               // where its due dates are filed
	ProjectBy       string    `json:"projectBy"`             // service (chosen) | keyword (found in its name) | empty
}

// State is everything the main screens show at once.
type State struct {
	Today       string               `json:"today"`
	Payments    []PaymentView        `json:"payments"`
	Invoices    []Invoice            `json:"invoices"`
	Missing     []DueItem            `json:"missing"`  // past due dates with no invoice, newest first
	Upcoming    []DueItem            `json:"upcoming"` // the next 45 days
	Todos       []Todo               `json:"todos"`
	Suggestions []Suggestion         `json:"suggestions"`
	AutoAdded   []string             `json:"autoAdded"`
	Forecast    Forecast             `json:"forecast"`
	Projects    []Project            `json:"projects"`
	Deadlines   []deadlines.Deadline `json:"deadlines"` // from 100 days back to a year ahead
	Admin       deadlines.Settings   `json:"admin"`
	Billing     []BillingLink        `json:"billing"` // where to fetch invoices, once per address
}

// State computes the main screens' data.
func (m *Model) State(r rates.Rates) State {
	res := State{
		Today: m.Today.Format(day.Format), Payments: []PaymentView{}, Invoices: m.Invoices, Todos: m.DB.Todos,
		Missing: []DueItem{}, Upcoming: []DueItem{}, Suggestions: m.Suggestions, AutoAdded: m.Added,
		Forecast: m.Forecast(r), Projects: m.DB.Projects,
		Deadlines: m.Deadlines(m.Today.AddDate(0, 0, -100), m.Today.AddDate(1, 0, 0)), Admin: m.DB.Settings(),
		Billing: m.billingLinks(),
	}
	if res.AutoAdded == nil {
		res.AutoAdded = []string{}
	}
	lastInvoice := m.lastInvoices()
	for _, p := range m.DB.Payments {
		est := m.estimate(p)
		project, by := ProjectOf(m.DB, Spend{Service: p.Name, match: p.Match})
		res.Payments = append(res.Payments, PaymentView{
			Payment: p, Current: est.Amount, CurrentCurrency: est.Currency,
			Variable: est.Variable, Recent: est.Recent, Manual: est.Manual, LastInvoice: lastInvoice[p.ID],
			Project: project, ProjectBy: by, BillingURL: p.billingURL(m.vendors),
		})
		for _, d := range p.Dues(m.Today.AddDate(-3, 0, 0), m.Today) {
			if it := m.dueItem(p, d); it.Status == "missing" {
				res.Missing = append(res.Missing, it)
			}
		}
		for _, d := range p.Dues(m.Today, m.Today.AddDate(0, 0, 45)) {
			res.Upcoming = append(res.Upcoming, m.dueItem(p, d))
		}
	}
	sort.SliceStable(res.Missing, func(i, j int) bool { return res.Missing[i].Date > res.Missing[j].Date })
	sort.SliceStable(res.Upcoming, func(i, j int) bool { return res.Upcoming[i].Date < res.Upcoming[j].Date })
	return res
}

// BillingLink is one place to fetch invoices from.
type BillingLink struct {
	Name   string `json:"name"`
	Domain string `json:"domain,omitempty"`
	URL    string `json:"url"`
}

// billingLinks lists, once per address, where to fetch the invoices of the
// running payments and of the vendors found in the folder.
func (m *Model) billingLinks() []BillingLink {
	type entry struct {
		BillingLink
		names  map[string]bool
		vendor string // the vendor named by the first service of this page
	}
	byURL := map[string]*entry{}
	var order []string
	add := func(name, vendorName, domain, u string) {
		if u == "" {
			return
		}
		e := byURL[u]
		if e == nil {
			e = &entry{BillingLink: BillingLink{Name: name, Domain: domain, URL: u}, names: map[string]bool{}, vendor: vendorName}
			byURL[u] = e
			order = append(order, u)
		}
		e.names[name] = true
		if len(e.names) > 1 && e.vendor != "" {
			e.Name = e.vendor // several services share the page: their vendor's name
		}
	}
	today := m.Today.Format(day.Format)
	for _, p := range m.DB.Payments {
		if p.End != "" && p.End < today {
			continue
		}
		add(p.Name, p.InvoiceName(), p.Domain, p.billingURL(m.vendors))
	}
	for _, inv := range m.Invoices {
		add(inv.Vendor, inv.Vendor, inv.Domain, m.vendors.BillingPage(inv.Domain, inv.Vendor))
	}
	out := make([]BillingLink, 0, len(order))
	for _, u := range order {
		out = append(out, byURL[u].BillingLink)
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}
