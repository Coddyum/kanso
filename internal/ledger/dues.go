package ledger

import (
	"sort"
	"strings"
	"time"

	"github.com/coddyum/kanso/internal/day"
	"github.com/coddyum/kanso/internal/i18n"
)

// Estimate is what the next due dates of a payment should cost.
type Estimate struct {
	Amount   float64   // the reference: the latest invoice, or the amount typed after it
	Currency string    //
	Prudent  float64   // what the reserve counts: the highest of the last three invoices when it varies
	Variable bool      // its last three invoices differ by more than 10 %
	Recent   []float64 // those invoices' amounts, oldest first
	Manual   bool      // the typed amount is newer than the latest invoice
}

func (m *Model) estimate(p Payment) Estimate {
	e := Estimate{Amount: p.Amount, Currency: p.Currency, Prudent: p.Amount}
	hist := m.history[p.ID]
	if len(hist) == 0 {
		return e
	}
	last := hist[len(hist)-1]
	if typed, err := time.Parse(time.RFC3339, p.AmountSet); err == nil && typed.After(last.Arrived) {
		e.Manual = true
		return e
	}
	e.Amount, e.Currency, e.Prudent = *last.Amount, last.Currency, *last.Amount
	lo, hi := *last.Amount, *last.Amount
	for _, inv := range hist[max(0, len(hist)-3):] {
		if inv.Currency != last.Currency {
			continue
		}
		e.Recent = append(e.Recent, *inv.Amount)
		lo, hi = min(lo, *inv.Amount), max(hi, *inv.Amount)
	}
	if len(e.Recent) >= 2 && hi > 0 && (hi-lo)/hi > 0.10 {
		e.Variable, e.Prudent = true, hi
	}
	return e
}

// dueStatus: matched to an invoice, or else upcoming, today, skipped (no
// invoice expected for it), paid (the payment expects no invoice) or missing.
func (m *Model) dueStatus(p Payment, d time.Time) dueState {
	key := DueKey(p.ID, d)
	if st, ok := m.dues[key]; ok {
		return st
	}
	switch {
	case d.After(m.Today):
		return dueState{Status: "upcoming"}
	case d.Equal(m.Today):
		return dueState{Status: "today"}
	case m.DB.Dismissed[key]:
		return dueState{Status: "skipped"}
	case !p.ExpectInvoice:
		return dueState{Status: "paid"}
	}
	return dueState{Status: "missing"}
}

// DueItem is one due date, as the calendar and the lists show it: a
// payment's, an invoice no payment claims, or an administrative deadline.
type DueItem struct {
	Key       string   `json:"key"`
	Type      string   `json:"type"` // payment | invoice | deadline
	Date      string   `json:"date"`
	PaymentID string   `json:"paymentId,omitempty"`
	InvoiceID string   `json:"invoiceId,omitempty"`
	Name      string   `json:"name"`
	Vendor    string   `json:"vendor,omitempty"`
	Domain    string   `json:"domain,omitempty"`
	Amount    *float64 `json:"amount"`
	Currency  string   `json:"currency"`
	Status    string   `json:"status"`
	Estimate  bool     `json:"estimate,omitempty"` // the amount is a guess: a variable service with no invoice yet
	Detail    string   `json:"detail,omitempty"`   // a deadline: what to declare or pay
}

// dueItem shows a due date; once its invoice is found, at the invoice's amount.
func (m *Model) dueItem(p Payment, d time.Time) DueItem {
	st := m.dueStatus(p, d)
	est := m.estimate(p)
	amount, currency, guess := est.Amount, est.Currency, est.Variable
	if inv := m.byID[st.InvoiceID]; inv != nil && inv.Amount != nil {
		amount, currency, guess = *inv.Amount, inv.Currency, false
	}
	return DueItem{
		Key: DueKey(p.ID, d), Type: "payment", Date: d.Format(day.Format), PaymentID: p.ID, InvoiceID: st.InvoiceID,
		Name: p.Name, Vendor: p.InvoiceName(), Domain: p.Domain, Amount: &amount, Currency: currency, Status: st.Status,
		Estimate: guess,
	}
}

// Calendar lists, between from and to, the due dates of the payments, the
// administrative deadlines, and the invoices no due date claims (spending
// that was not planned), by date then by name.
func (m *Model) Calendar(from, to time.Time, tr i18n.Lang) []DueItem {
	events := []DueItem{}
	for _, p := range m.DB.Payments {
		for _, d := range p.Dues(from, to) {
			events = append(events, m.dueItem(p, d))
		}
	}
	for _, d := range m.Deadlines(from, to, tr) {
		events = append(events, DueItem{Key: d.Key, Type: "deadline", Date: d.Date, Name: d.Title, Detail: d.Detail, Status: d.Status})
	}
	claimed := m.claimed()
	lo, hi := from.Format(day.Format), to.Format(day.Format)
	for _, inv := range m.Invoices {
		if claimed[inv.ID] || inv.Date < lo || inv.Date > hi {
			continue
		}
		name := inv.Vendor
		if name == "" {
			name = inv.Files[0].Name
		}
		events = append(events, DueItem{
			Key: "inv:" + inv.ID, Type: "invoice", Date: inv.Date, InvoiceID: inv.ID, Name: name,
			Vendor: inv.Vendor, Domain: inv.Domain, Amount: inv.Amount, Currency: inv.Currency, Status: "invoice",
		})
	}
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Date != events[j].Date {
			return events[i].Date < events[j].Date
		}
		return events[i].Name < events[j].Name
	})
	return events
}

// claimed are the invoices paired with a due date.
func (m *Model) claimed() map[string]bool {
	out := map[string]bool{}
	for _, st := range m.dues {
		out[st.InvoiceID] = true
	}
	return out
}

// lastInvoices is, per payment, the date of the latest invoice paired with one of its due dates.
func (m *Model) lastInvoices() map[string]string {
	out := map[string]string{}
	for key, st := range m.dues {
		if inv := m.byID[st.InvoiceID]; inv != nil {
			if id, _, _ := strings.Cut(key, "@"); inv.Date > out[id] {
				out[id] = inv.Date
			}
		}
	}
	return out
}
