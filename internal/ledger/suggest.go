package ledger

import (
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/coddyum/kanso/internal/day"
	"github.com/coddyum/kanso/internal/norm"
	"github.com/coddyum/kanso/internal/vendor"
)

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

// suggestKey names a detection: « suggest:vendor » or « suggest:vendor:match ».
func suggestKey(vendorName, match string) string {
	k := "suggest:" + norm.Key(vendorName)
	if match != "" {
		k += ":" + norm.Key(match)
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

// cycleOf reads the cycle of a vendor's invoices, oldest first: the usual
// number of months between them, or the period a single invoice writes.
func cycleOf(list []Invoice) string {
	var months []int
	for _, inv := range list {
		if t, ok := day.Parse(inv.Date); ok {
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
		if norm.Key(inv.Vendor) == "" {
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
		first, _ := day.Parse(g.list[0].Date)
		last := g.list[len(g.list)-1]
		lastDay, _ := day.Parse(last.Date)
		// The month of the first invoice, on the day of the latest one: the past
		// due dates meet the invoices already filed, the next ones fall on the
		// current billing day.
		start := time.Date(first.Year(), first.Month(), min(lastDay.Day(), day.DaysIn(first.Year(), int(first.Month()))), 0, 0, 0, 0, time.Local)
		name := g.vendor
		if g.match != "" {
			name += " · " + g.match
		}
		s := Suggestion{
			Key: k, Name: name, Vendor: g.vendor, Domain: last.Domain, Match: g.match, Currency: last.Currency,
			Cycle: cycle, Start: start.Format(day.Format), Count: len(g.list), Last: last.Date,
			Active: day.Between(lastDay, today) <= recentDays[cycle],
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
		if vendor.Matches(p.InvoiceName(), vendorName) && strings.EqualFold(p.Match, match) {
			return true
		}
	}
	return false
}
