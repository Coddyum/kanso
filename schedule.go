package main

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

const dayFmt = "2006-01-02"

func parseDay(s string) (time.Time, bool) {
	t, err := time.ParseInLocation(dayFmt, s, time.Local)
	return t, err == nil
}

func dayOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

func daysIn(y, m int) int {
	return time.Date(y, time.Month(m)+1, 0, 0, 0, 0, 0, time.Local).Day()
}

// addMonths keeps the day of the month, clamped: Jan 31 + 1 month = Feb 28.
func addMonths(t time.Time, n int) time.Time {
	total := t.Year()*12 + int(t.Month()) - 1 + n
	y, m := total/12, total%12+1
	return time.Date(y, time.Month(m), min(t.Day(), daysIn(y, m)), 0, 0, 0, 0, time.Local)
}

func nthDue(start time.Time, cycle string, i int) time.Time {
	switch cycle {
	case "weekly":
		return start.AddDate(0, 0, 7*i)
	case "monthly":
		return addMonths(start, i)
	case "quarterly":
		return addMonths(start, 3*i)
	case "yearly":
		return addMonths(start, 12*i)
	}
	return start
}

// dues lists the due dates of p between from and to, both included.
func (p Payment) dues(from, to time.Time) []time.Time {
	start, ok := parseDay(p.Start)
	if !ok {
		return nil
	}
	end, hasEnd := parseDay(p.End)
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

func (p Payment) invoiceName() string {
	if p.Vendor != "" {
		return p.Vendor
	}
	return p.Name
}

func vendorMatches(name, vendorName string) bool {
	a, b := norm(name), norm(vendorName)
	if a == "" || b == "" {
		return false
	}
	return strings.Contains(a, b) || strings.Contains(b, a)
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
	if !vendorMatches(p.invoiceName(), inv.Vendor) {
		return false
	}
	text := invoiceText(inv)
	if p.Match != "" {
		return strings.Contains(text, strings.ToLower(p.Match))
	}
	for _, q := range payments {
		if q.Match != "" && vendorMatches(q.invoiceName(), inv.Vendor) && strings.Contains(text, strings.ToLower(q.Match)) {
			return false
		}
	}
	return true
}

func dueKey(id string, d time.Time) string { return id + "@" + d.Format(dayFmt) }

type dueState struct {
	Status    string `json:"status"` // upcoming | today | matched | missing | skipped | paid
	InvoiceID string `json:"invoiceId,omitempty"`
}

// matchDues pairs each past due date with an invoice: first the file dropped
// for it in the app, then an invoice it pays dated 10 days before to 25 days
// after. Specific payments go first (see rank); an invoice pays one due date only.
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
		for _, d := range p.dues(from, today) {
			key := dueKey(p.ID, d)
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
		for _, d := range p.dues(from, today) {
			key := dueKey(p.ID, d)
			if _, done := res[key]; done {
				continue
			}
			best, bestGap := "", 1<<30
			for _, inv := range cands {
				t, ok := parseDay(inv.Date)
				if used[inv.ID] || !ok {
					continue
				}
				// A free invoice (a month of R2 under its free tier) never pays a due date that costs something.
				if inv.Amount != nil && *inv.Amount == 0 && p.Amount > 0 {
					continue
				}
				gap := daysBetween(d, t)
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

func (m *model) dueStatus(p Payment, d time.Time) dueState {
	key := dueKey(p.ID, d)
	if st, ok := m.dues[key]; ok {
		return st
	}
	switch {
	case d.After(m.today):
		return dueState{Status: "upcoming"}
	case d.Equal(m.today):
		return dueState{Status: "today"}
	case m.db.Dismissed[key]:
		return dueState{Status: "skipped"}
	case !p.ExpectInvoice:
		return dueState{Status: "paid"}
	}
	return dueState{Status: "missing"}
}

// Suggestion is a subscription read from the invoices of the folder.
type Suggestion struct {
	Key      string  `json:"key"`
	Name     string  `json:"name"`
	Vendor   string  `json:"vendor"`
	Domain   string  `json:"domain,omitempty"`
	Match    string  `json:"match,omitempty"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Cycle    string  `json:"cycle"`
	Start    string  `json:"start"`
	Count    int     `json:"count"`
	Last     string  `json:"last"`
	Active   bool    `json:"active"` // its latest invoice is recent: the app adds it by itself
}

func suggestKey(vendorName, match string) string {
	k := "suggest:" + norm(vendorName)
	if match != "" {
		k += ":" + norm(match)
	}
	return k
}

var (
	reYearly     = regexp.MustCompile(`(?i)\b(1 yr|1 year|1 an|annual|yearly|annuel|registrar|renewal|renouvellement)\b`)
	reDomainName = regexp.MustCompile(`(?i)\b[a-z0-9][a-z0-9-]*\.(?:[a-z]{2,6})\b`)
)

// yearlySubject names what a yearly line bills on its own (« example.com » for
// « Registrar Registration Fee - example.com (1 yr) »), or "" for a regular bill.
func yearlySubject(label string) string {
	if !reYearly.MatchString(label) {
		return ""
	}
	if d := reDomainName.FindString(label); d != "" {
		return strings.ToLower(d)
	}
	return strings.TrimSpace(reYearly.ReplaceAllString(label, ""))
}

// recentDays: how old the latest invoice may be for the subscription to count
// as running — three monthly bills, two quarterly ones, one yearly one.
var recentDays = map[string]int{"monthly": 3*31 + 15, "quarterly": 2*92 + 15, "yearly": 366 + 60}

func cycleOf(list []Invoice) string {
	var months []int
	for _, inv := range list {
		if t, ok := parseDay(inv.Date); ok {
			if mi := t.Year()*12 + int(t.Month()); len(months) == 0 || months[len(months)-1] != mi {
				months = append(months, mi)
			}
		}
	}
	if len(months) >= 2 {
		gaps := make([]int, 0, len(months)-1)
		for i := 1; i < len(months); i++ {
			gaps = append(gaps, months[i]-months[i-1])
		}
		sort.Ints(gaps)
		switch g := gaps[len(gaps)/2]; {
		case g == 1:
			return "monthly"
		case g == 3:
			return "quarterly"
		case g >= 11 && g <= 13:
			return "yearly"
		}
		return ""
	}
	switch d := list[len(list)-1].PeriodDays; {
	case d >= 25 && d <= 35:
		return "monthly"
	case d >= 85 && d <= 95:
		return "quarterly"
	case d >= 355 && d <= 370:
		return "yearly"
	}
	return ""
}

// suggest reads the subscriptions out of the invoices: a vendor billed every
// month, quarter or year, a single invoice whose written period says it, and
// each yearly line (a domain) on its own. Covered or dismissed ones are left out.
func suggest(invoices []Invoice, payments []Payment, dismissed map[string]bool, today time.Time) []Suggestion {
	type group struct {
		vendor, match string
		list          []Invoice
	}
	groups := map[string]*group{}
	var keys []string
	for _, inv := range invoices {
		if norm(inv.Vendor) == "" {
			continue
		}
		match := yearlySubject(inv.Label)
		k := suggestKey(inv.Vendor, match)
		if groups[k] == nil {
			groups[k] = &group{vendor: inv.Vendor, match: match}
			keys = append(keys, k)
		}
		groups[k].list = append(groups[k].list, inv)
	}
	out := []Suggestion{}
	for _, k := range keys {
		g := groups[k]
		if dismissed[k] || covered(payments, k, g.vendor, g.match) {
			continue
		}
		sort.Slice(g.list, func(i, j int) bool { return g.list[i].Date < g.list[j].Date })
		cycle := cycleOf(g.list)
		if cycle == "" && g.match != "" {
			cycle = "yearly"
		}
		if cycle == "" {
			continue
		}
		first, _ := parseDay(g.list[0].Date)
		last := g.list[len(g.list)-1]
		lastDay, _ := parseDay(last.Date)
		// The month of the first invoice, on the day of the latest one: the past
		// due dates meet the invoices already filed, the next ones fall on the
		// current billing day.
		start := time.Date(first.Year(), first.Month(), min(lastDay.Day(), daysIn(first.Year(), int(first.Month()))), 0, 0, 0, 0, time.Local)
		name := g.vendor
		if g.match != "" {
			name += " · " + g.match
		}
		s := Suggestion{
			Key: k, Name: name, Vendor: g.vendor, Domain: last.Domain, Match: g.match, Currency: last.Currency,
			Cycle: cycle, Start: start.Format(dayFmt), Count: len(g.list), Last: last.Date,
			Active: daysBetween(lastDay, today) <= recentDays[cycle],
		}
		if last.Amount != nil {
			s.Amount = *last.Amount
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// covered: a payment already stands for this detection — made from it, or the
// same vendor (and the same Match).
func covered(payments []Payment, key, vendorName, match string) bool {
	for _, p := range payments {
		if p.SuggestKey == key {
			return true
		}
		if vendorMatches(p.invoiceName(), vendorName) && strings.EqualFold(p.Match, match) {
			return true
		}
	}
	return false
}
