package extract

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/coddyum/kanso/internal/currency"
)

const numRe = `\d+(?:[ \x{00a0}\x{202f}.,]\d{3})*(?:[.,]\d{1,2})?`

// signs are the currency signs an invoice writes, lowercased, with their ISO
// code. A bare « $ » is the US dollar; the other dollars carry their country.
var signs = map[string]string{
	"€": "EUR", "$": "USD", "us$": "USD", "£": "GBP", "¥": "JPY", "円": "JPY", "cn¥": "CNY", "元": "CNY", "rmb": "CNY",
	"ca$": "CAD", "c$": "CAD", "a$": "AUD", "au$": "AUD", "nz$": "NZD", "hk$": "HKD", "s$": "SGD", "sg$": "SGD",
	"r$": "BRL", "mx$": "MXN", "₹": "INR", "₩": "KRW", "₺": "TRY", "₪": "ILS", "฿": "THB", "₱": "PHP", "zł": "PLN", "kč": "CZK",
}

// moneyRe is the alternation of every sign and code, longest first; a sign
// or code that starts with a letter must start a word (« ils 12 » is not « ILS »).
var moneyRe = func() string {
	var alts []string
	for s := range signs {
		alts = append(alts, s)
	}
	for _, c := range currency.Codes {
		alts = append(alts, strings.ToLower(c))
	}
	sort.Slice(alts, func(i, j int) bool {
		return len(alts[i]) > len(alts[j]) || len(alts[i]) == len(alts[j]) && alts[i] < alts[j]
	})
	for i, a := range alts {
		alts[i] = regexp.QuoteMeta(a)
		if r := a[0]; r >= 'a' && r <= 'z' {
			alts[i] = `\b` + alts[i]
			if _, ok := signs[a]; !ok {
				alts[i] += `\b`
			}
		}
	}
	return "(?:" + strings.Join(alts, "|") + ")"
}()

// codeRe is the alternation of the codes alone: after a sign, a code may touch
// the number (« $USD8.89 »), so it needs no word boundary there.
var codeRe = "(?:" + strings.ToLower(strings.Join(currency.Codes, "|")) + ")"

var (
	reMoney      = regexp.MustCompile(`(?i)(?:(` + moneyRe + `)\s?(` + codeRe + `)?\s?(` + numRe + `)|(` + numRe + `)\s?(` + moneyRe + `))`)
	reHeadline   = regexp.MustCompile(`(?i)(` + moneyRe + `)\s?(` + numRe + `)(?:\s?` + moneyRe + `)?\s+(?:paid on|due)`)
	reHeadlineFR = regexp.MustCompile(`(?i)(` + numRe + `)\s?€\s+(?:dus? le|payés? le|prélevés? le)`)
	reBareNum    = regexp.MustCompile(`\d+[.,]\d{2}\b`)
	reMoneyStart = regexp.MustCompile(`(?i)^(?:\d|` + moneyRe + `)`)
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

// startsWithMoney says whether s begins with a digit, a currency sign or a currency code.
func startsWithMoney(s string) bool { return reMoneyStart.MatchString(s) }

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
	sym, num := m[1], m[3]
	if m[2] != "" {
		sym = m[2] // « $ CAD 12 »: the code says which dollar
	}
	if num == "" {
		num, sym = m[4], m[5]
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

// currencyOf names the currency of a sign or a code; the euro when it is neither.
func currencyOf(sym string) string {
	sym = strings.ToLower(sym)
	if code := signs[sym]; code != "" {
		return code
	}
	if code := strings.ToUpper(sym); currency.Known(code) {
		return code
	}
	return "EUR"
}
