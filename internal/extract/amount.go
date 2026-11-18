package extract

import (
	"regexp"
	"strconv"
	"strings"
)

const numRe = `\d+(?:[ \x{00a0}\x{202f}.,]\d{3})*(?:[.,]\d{1,2})?`

var (
	reMoney      = regexp.MustCompile(`(?i)(?:([€$£])\s?(?:usd|eur)?\s?(` + numRe + `)|(` + numRe + `)\s?(€|eur\b|usd\b))`)
	reHeadline   = regexp.MustCompile(`(?i)([€$£])\s?(` + numRe + `)(?:\s?(?:usd|eur))?\s+(?:paid on|due)`)
	reHeadlineFR = regexp.MustCompile(`(?i)(` + numRe + `)\s?€\s+(?:dus? le|payés? le|prélevés? le)`)
	reBareNum    = regexp.MustCompile(`\d+[.,]\d{2}\b`)
)

// amountLabels start the lines that carry the total, most precise first.
var amountLabels = []struct {
	prefix   string
	lastBare bool // the amount carries no currency: take the last number of the line
}{
	{"amount paid", false}, {"montant payé", false}, {"bill total", false},
	{"total. including taxes", false}, {"total including taxes", false}, {"total ttc", false},
	{"net à payer", false}, {"montant ttc", false}, {"total à payer", false},
	{"montant prélevé", true}, {"montant total", false}, {"total", false},
	{"amount due", false}, {"montant dû", false},
}

// detectAmount looks for the total paid: labelled lines first, then the
// « €21.60 paid on … » headline of Stripe-made invoices.
func detectAmount(lines []string, text string) (float64, string, bool) {
	lowered := make([]string, len(lines))
	for i, l := range lines {
		lowered[i] = strings.ToLower(l)
	}
	for _, lab := range amountLabels {
		for _, l := range lowered {
			if !strings.HasPrefix(l, lab.prefix) {
				continue
			}
			rest := strings.TrimLeft(l[len(lab.prefix):], " :")
			if lab.prefix == "total" && !startsWithMoney(rest) {
				continue
			}
			if v, cur, ok := moneyIn(rest, lab.lastBare); ok {
				return v, cur, true
			}
		}
	}
	lt := strings.ToLower(text)
	if m := reHeadline.FindStringSubmatch(lt); m != nil {
		if v, ok := parseNumber(m[2]); ok {
			return v, currencyOf(m[1]), true
		}
	}
	if m := reHeadlineFR.FindStringSubmatch(lt); m != nil {
		if v, ok := parseNumber(m[1]); ok {
			return v, "EUR", true
		}
	}
	return 0, "", false
}

// startsWithMoney says whether s begins with a digit or a currency sign.
func startsWithMoney(s string) bool {
	for _, r := range s {
		return strings.ContainsRune("€$£0123456789", r) // privacy:ok (the digits, not a phone number)
	}
	return false
}

func moneyIn(s string, lastBare bool) (float64, string, bool) {
	if lastBare {
		all := reBareNum.FindAllString(s, -1)
		if len(all) == 0 {
			return 0, "", false
		}
		v, ok := parseNumber(all[len(all)-1])
		return v, "EUR", ok
	}
	m := reMoney.FindStringSubmatch(s)
	if m == nil {
		return 0, "", false
	}
	sym, num := m[1], m[2]
	if num == "" {
		num, sym = m[3], m[4]
	}
	v, ok := parseNumber(num)
	return v, currencyOf(sym), ok
}

// parseNumber reads « 1 234,50 », « 1,234.50 » or « 21.6 »: the last separator
// followed by one or two digits is the decimal one.
func parseNumber(s string) (float64, bool) {
	s = strings.Map(func(r rune) rune {
		if r == ' ' || r == ' ' || r == ' ' {
			return -1
		}
		return r
	}, s)
	strip := strings.NewReplacer(".", "", ",", "")
	if i := strings.LastIndexAny(s, ".,"); i >= 0 && len(s)-i-1 >= 1 && len(s)-i-1 <= 2 {
		s = strip.Replace(s[:i]) + "." + s[i+1:]
	} else {
		s = strip.Replace(s)
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

func currencyOf(sym string) string {
	switch strings.ToLower(sym) {
	case "$", "usd":
		return "USD"
	case "£":
		return "GBP"
	}
	return "EUR"
}
