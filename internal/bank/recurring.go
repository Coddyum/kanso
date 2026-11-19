package bank

import (
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/coddyum/kanso/internal/norm"
	"github.com/coddyum/kanso/internal/vendor"
)

// Subscription is a debit that comes back: the same label, close amounts, a regular rhythm.
type Subscription struct {
	Key      string  `json:"key"`
	Name     string  `json:"name"`
	Domain   string  `json:"domain,omitempty"`
	Amount   float64 `json:"amount"` // the latest debit, positive
	Currency string  `json:"currency"`
	Cycle    string  `json:"cycle"` // weekly | monthly | quarterly | yearly
	Start    string  `json:"start"` // its latest debit: the next due dates follow it
	Count    int     `json:"count"`
	Known    bool    `json:"known"` // a payment already stands for it
}

// cycles: the usual gap between two debits, in days, and how many debits make a rhythm.
var cycles = []struct {
	name     string
	min, max int
	need     int
}{
	{"weekly", 6, 8, 4},
	{"monthly", 26, 35, 3},
	{"quarterly", 84, 98, 2},
	{"yearly", 350, 380, 2},
}

// noise are the words of a bank label that say how it was paid, not to whom.
var noise = map[string]bool{
	"cb": true, "carte": true, "card": true, "prlv": true, "prelevement": true, "prelevmt": true, "sepa": true,
	"paiement": true, "payment": true, "achat": true, "purchase": true, "vir": true, "virement": true, "transfer": true,
	"facture": true, "fact": true, "ech": true, "echeance": true, "id": true, "emetteur": true, "mdt": true, "ref": true,
	"rum": true, "de": true, "du": true, "des": true, "le": true, "la": true, "les": true, "pos": true, "dd": true,
	"direct": true, "debit": true, "com": true, "fr": true, "net": true, "io": true, "www": true, "sas": true, "sarl": true,
	"sa": true, "sasu": true, "eurl": true, "ltd": true, "inc": true, "gmbh": true, "llc": true, "bv": true, "srl": true,
}

// deadlineWords are the payments the administrative deadlines already follow (URSSAF, taxes).
var deadlineWords = []string{"urssaf", "dgfip", "impot", "finances publiques"}

var reDomain = regexp.MustCompile(`(?i)\b([a-z0-9][a-z0-9-]*\.(?:com|fr|io|net|org|app|dev|co|me|so|ai|eu|tv))\b`)

// labelWords keeps the words that name who is paid: the noise of the bank
// goes, the label stops at its first reference or date, three words at most.
func labelWords(label string) []string {
	var kept []string
	for _, w := range strings.Fields(norm.Words(label)) {
		if strings.IndexFunc(w, unicode.IsDigit) >= 0 {
			if len(kept) > 0 {
				break
			}
			continue
		}
		if noise[w] || len(w) < 2 {
			continue
		}
		kept = append(kept, w)
		if len(kept) == 3 {
			break
		}
	}
	return kept
}

// Recurring reads the subscriptions out of the debits: grouped by label, the
// debits close to their group's usual amount must come back on a regular
// rhythm, and the latest must be recent for the file (a stopped subscription
// is left out). The payments the administrative deadlines follow are left out too.
func Recurring(txs []Transaction, currency string, vendors *vendor.Catalog) []Subscription {
	groups := map[string][]Transaction{}
	labels := map[string]string{}
	var end time.Time
	for _, tx := range txs {
		if tx.Date.After(end) {
			end = tx.Date
		}
		words := labelWords(tx.Label)
		if tx.Amount >= 0 || len(words) == 0 || isDeadline(tx.Label) {
			continue
		}
		k := strings.Join(words, " ")
		groups[k] = append(groups[k], tx)
		labels[k] = tx.Label
	}
	var out []Subscription
	for k, list := range groups {
		members := closeToUsual(list)
		cycle, ok := rhythm(members)
		if !ok {
			continue
		}
		last := members[len(members)-1]
		if end.Sub(last.Date).Hours()/24 > float64(graceDays(cycle)) {
			continue
		}
		s := Subscription{
			Key: "bank:" + norm.Key(k), Amount: -last.Amount, Currency: last.Currency, Cycle: cycle,
			Start: last.Date.Format("2006-01-02"), Count: len(members),
		}
		if s.Currency == "" {
			s.Currency = currency
		}
		if s.Currency == "" {
			s.Currency = "EUR"
		}
		s.Name, s.Domain = nameOf(k, labels[k], vendors)
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

func isDeadline(label string) bool {
	w := norm.Words(label)
	for _, d := range deadlineWords {
		if strings.Contains(w, " "+d) {
			return true
		}
	}
	return false
}

// closeToUsual keeps the debits within 15 % (or 1 unit) of the group's median amount, by date.
func closeToUsual(list []Transaction) []Transaction {
	amounts := make([]float64, len(list))
	for i, tx := range list {
		amounts[i] = -tx.Amount
	}
	sort.Float64s(amounts)
	median := amounts[len(amounts)/2]
	tolerance := max(median*0.15, 1)
	var kept []Transaction
	for _, tx := range list {
		if d := -tx.Amount - median; d <= tolerance && d >= -tolerance {
			kept = append(kept, tx)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].Date.Before(kept[j].Date) })
	return kept
}

// rhythm is the cycle of debits by date: the usual gap between two of them
// must fall in a cycle, and seven gaps in ten at least. Two monthly debits of
// the very same amount are enough; otherwise each cycle needs its count.
func rhythm(list []Transaction) (string, bool) {
	var gaps []int
	for i := 1; i < len(list); i++ {
		if g := int(list[i].Date.Sub(list[i-1].Date).Hours()/24 + 0.5); g > 0 {
			gaps = append(gaps, g)
		}
	}
	if len(gaps) == 0 {
		return "", false
	}
	sorted := append([]int(nil), gaps...)
	sort.Ints(sorted)
	usual := sorted[len(sorted)/2]
	for _, c := range cycles {
		if usual < c.min || usual > c.max {
			continue
		}
		regular := 0
		for _, g := range gaps {
			if g >= c.min && g <= c.max {
				regular++
			}
		}
		count := len(gaps) + 1
		sameAmount := len(list) == 2 && list[0].Amount == list[1].Amount
		if regular*10 >= len(gaps)*7 && (count >= c.need || (c.name == "monthly" && sameAmount)) {
			return c.name, true
		}
	}
	return "", false
}

// graceDays: how long after its latest debit a subscription still counts as running.
func graceDays(cycle string) int {
	return map[string]int{"weekly": 11, "monthly": 45, "quarterly": 135, "yearly": 400}[cycle]
}

// nameOf names a subscription: the known vendor its label names, or its
// words with capitals; its domain is the vendor's, or the one the label writes.
func nameOf(key, label string, vendors *vendor.Catalog) (string, string) {
	domain := ""
	if m := reDomain.FindStringSubmatch(label); m != nil {
		domain = strings.ToLower(m[1])
	}
	if v, ok := vendors.Find(label); ok {
		if v.Domain != "" {
			domain = v.Domain
		}
		return v.Name, domain
	}
	words := strings.Fields(key)
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " "), domain
}
