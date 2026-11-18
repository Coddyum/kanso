package ledger

import (
	"sort"
	"strings"
	"time"

	"github.com/coddyum/kanso/internal/day"
	"github.com/coddyum/kanso/internal/vendor"
)

func nthDue(start time.Time, cycle string, i int) time.Time {
	switch cycle {
	case "weekly":
		return start.AddDate(0, 0, 7*i)
	case "monthly":
		return day.AddMonths(start, i)
	case "quarterly":
		return day.AddMonths(start, 3*i)
	case "yearly":
		return day.AddMonths(start, 12*i)
	}
	return start
}

// Dues lists the due dates of p between from and to, both included, up to its end.
func (p Payment) Dues(from, to time.Time) []time.Time {
	start, ok := day.Parse(p.Start)
	if !ok {
		return nil
	}
	end, hasEnd := day.Parse(p.End)
	var out []time.Time
	for i := 0; i < 5000; i++ {
		d := nthDue(start, p.Cycle, i)
		if d.After(to) || (hasEnd && d.After(end)) {
			break
		}
		if !d.Before(from) {
			out = append(out, d)
		}
		if p.Cycle == "once" || p.Cycle == "" {
			break
		}
	}
	return out
}

// invoiceText is what a payment's Match is looked for in: label, number, file names.
func invoiceText(inv Invoice) string {
	parts := []string{inv.Label, inv.Number}
	for _, f := range inv.Files {
		parts = append(parts, f.Name)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

// pays says whether inv can be an invoice of p: same vendor, and its Match
// when it has one. An invoice carrying the Match of another payment of the same
// vendor belongs to that one (a domain renewal is not the monthly bill).
func pays(p Payment, inv Invoice, payments []Payment) bool {
	if !vendor.Matches(p.InvoiceName(), inv.Vendor) {
		return false
	}
	text := invoiceText(inv)
	if p.Match != "" {
		return strings.Contains(text, strings.ToLower(p.Match))
	}
	for _, q := range payments {
		if q.Match != "" && vendor.Matches(q.InvoiceName(), inv.Vendor) && strings.Contains(text, strings.ToLower(q.Match)) {
			return false
		}
	}
	return true
}

// DueKey names one due date of a payment: « id@YYYY-MM-DD ».
func DueKey(id string, d time.Time) string { return id + "@" + d.Format(day.Format) }

type dueState struct {
	Status    string // upcoming | today | matched | missing | skipped | paid
	InvoiceID string
}

// matchDues pairs each due date of the last three years with an invoice:
// first the file dropped for it in the app, then an invoice it pays dated 10
// days before to 25 days after. Specific payments go first (see rank); an
// invoice pays one due date only, and a free invoice never pays a due date
// that costs something.
func matchDues(payments []Payment, invoices []Invoice, links map[string]string, today time.Time) map[string]dueState {
	res := map[string]dueState{}
	used := map[string]bool{}
	byHash := map[string]string{}
	for _, inv := range invoices {
		for _, f := range inv.Files {
			byHash[f.Hash] = inv.ID
		}
	}
	from := today.AddDate(-3, 0, 0)
	for _, p := range payments {
		for _, d := range p.Dues(from, today) {
			key := DueKey(p.ID, d)
			if id, ok := byHash[links[key]]; ok {
				res[key] = dueState{Status: "matched", InvoiceID: id}
				used[id] = true
			}
		}
	}
	// The most specific payments pick their invoice first: one with a Match,
	// then a one-off payment (its date is exact), then the recurring ones.
	rank := func(p Payment) int {
		switch {
		case p.Match != "":
			return 0
		case p.Cycle == "once" || p.Cycle == "":
			return 1
		}
		return 2
	}
	ordered := append([]Payment(nil), payments...)
	sort.SliceStable(ordered, func(i, j int) bool { return rank(ordered[i]) < rank(ordered[j]) })
	for _, p := range ordered {
		var cands []Invoice
		for _, inv := range invoices {
			if pays(p, inv, payments) {
				cands = append(cands, inv)
			}
		}
		for _, d := range p.Dues(from, today) {
			key := DueKey(p.ID, d)
			if _, done := res[key]; done {
				continue
			}
			best, bestGap := "", 1<<30
			for _, inv := range cands {
				t, ok := day.Parse(inv.Date)
				if used[inv.ID] || !ok {
					continue
				}
				// A free invoice (a month of storage under its free tier) never pays a due date that costs something.
				if inv.Amount != nil && *inv.Amount == 0 && p.Amount > 0 {
					continue
				}
				gap := day.Between(d, t)
				if gap < -10 || gap > 25 {
					continue
				}
				if abs := max(gap, -gap); abs < bestGap {
					best, bestGap = inv.ID, abs
				}
			}
			if best != "" {
				used[best] = true
				res[key] = dueState{Status: "matched", InvoiceID: best}
			}
		}
	}
	return res
}
