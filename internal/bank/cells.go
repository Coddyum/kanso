package bank

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coddyum/kanso/internal/day"
)

// A date cell: 2026-11-18, 18/11/2026, 11/18/26, 18.11.2026, 20261118, 18 nov. 2026, Nov 18, 2026;
// a time after it (« 2026-11-18 10:22:33 », « …T10:22:33Z ») is ignored.
var (
	reISO      = regexp.MustCompile(`^(\d{4})[-/.](\d{1,2})[-/.](\d{1,2})(?:$|[T\s])`)
	reNumeric  = regexp.MustCompile(`^(\d{1,2})[-/.](\d{1,2})[-/.](\d{4}|\d{2})(?:$|[T\s])`)
	reCompact  = regexp.MustCompile(`^(\d{4})(\d{2})(\d{2})(?:$|\d{6}|[T\s.\[])`)
	reDayMonth = regexp.MustCompile(`^(\d{1,2})[\s-]+(\p{L}+)\.?[\s-]+(\d{4}|\d{2})(?:$|\s)`)
	reMonthDay = regexp.MustCompile(`^(\p{L}+)\.?\s+(\d{1,2}),?\s+(\d{4})(?:$|\s)`)
)

// dateParts is a date cell cut in three numbers; numeric says the first two may be day/month or month/day.
type dateParts struct {
	a, b, y int
	numeric bool
}

func splitDate(s string) (dateParts, bool) {
	s = strings.TrimSpace(s)
	if m := reISO.FindStringSubmatch(s); m != nil {
		return dateParts{atoi(m[3]), atoi(m[2]), atoi(m[1]), false}, valid(atoi(m[1]), atoi(m[2]), atoi(m[3]))
	}
	if m := reNumeric.FindStringSubmatch(s); m != nil {
		y := year(m[3])
		a, b := atoi(m[1]), atoi(m[2])
		return dateParts{a, b, y, true}, valid(y, b, a) || valid(y, a, b)
	}
	if m := reCompact.FindStringSubmatch(s); m != nil {
		return dateParts{atoi(m[3]), atoi(m[2]), atoi(m[1]), false}, valid(atoi(m[1]), atoi(m[2]), atoi(m[3]))
	}
	if m := reDayMonth.FindStringSubmatch(s); m != nil {
		mo, y := day.MonthOf(m[2]), year(m[3])
		return dateParts{atoi(m[1]), mo, y, false}, valid(y, mo, atoi(m[1]))
	}
	if m := reMonthDay.FindStringSubmatch(s); m != nil {
		mo, y := day.MonthOf(m[1]), atoi(m[3])
		return dateParts{atoi(m[2]), mo, y, false}, valid(y, mo, atoi(m[2]))
	}
	return dateParts{}, false
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

// year reads a 2-digit year in this century.
func year(s string) int {
	y := atoi(s)
	if len(s) == 2 {
		y += 2000
	}
	return y
}

func valid(y, m, d int) bool { return day.YMD(y, m, d) != "" }

// dateOrder says how the column writes its numeric dates: day first
// (« dmy »), or month first (« mdy »). A part over 12 settles it; otherwise
// the order that keeps the dates in sequence wins, day first on a tie.
func dateOrder(cells []string) string {
	var parts []dateParts
	for _, c := range cells {
		if p, ok := splitDate(c); ok && p.numeric {
			if p.a > 12 {
				return "dmy"
			}
			if p.b > 12 {
				return "mdy"
			}
			parts = append(parts, p)
		}
	}
	inSequence := func(dayFirst bool) int {
		n, prev := 0, ""
		for _, p := range parts {
			d, m := p.a, p.b
			if !dayFirst {
				d, m = m, d
			}
			cur := day.YMD(p.y, m, d)
			if prev != "" && cur != "" && cur >= prev {
				n++
			}
			prev = cur
		}
		// A file written newest first is in sequence backwards: count both ways.
		return max(n, len(parts)-1-n)
	}
	if inSequence(false) > inSequence(true) {
		return "mdy"
	}
	return "dmy"
}

// parseDate reads a date cell at local midnight; order settles 01/02/2026.
func parseDate(s, order string) (time.Time, bool) {
	p, ok := splitDate(s)
	if !ok {
		return time.Time{}, false
	}
	d, m := p.a, p.b
	if p.numeric && order == "mdy" {
		d, m = m, d
	}
	return day.Parse(day.YMD(p.y, m, d))
}

// An amount cell: a sign, a currency (a symbol or an ISO code in capitals), digits with their separators, a sign after.
var reAmount = regexp.MustCompile(`^\(?[+\-\x{2212}]?\s*(?:[A-Z]{3}\s*|[\x{20ac}$\x{a3}\x{a5}]\s*)?[+\-\x{2212}]?\d[\d\s.,'\x{a0}\x{202f}]*(?:\s*(?:[A-Z]{3}|[\x{20ac}$\x{a3}\x{a5}]))?\s*[-\x{2212}]?\)?$`)

func looksAmount(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && reAmount.MatchString(s)
}

// decimalOf is the decimal separator the cells of these columns use: the
// one followed by one or two digits at the end of most of them; « . » when
// no cell has decimals.
func decimalOf(columns ...[]string) byte {
	comma, dot := 0, 0
	for _, col := range columns {
		for _, c := range col {
			digits := strings.TrimRightFunc(c, func(r rune) bool { return r < '0' || r > '9' })
			i := strings.LastIndexAny(digits, ".,")
			if i < 0 || len(digits)-i-1 > 2 || len(digits)-i-1 == 0 {
				continue
			}
			if digits[i] == ',' {
				comma++
			} else {
				dot++
			}
		}
	}
	if comma > dot {
		return ','
	}
	return '.'
}

// parseAmount reads an amount cell with dec as its decimal separator:
// « -1 234,56 », « 1.234,56 », « 1,234.56 », « 12,99- », « (12.99) », « €12 ».
func parseAmount(s string, dec byte) (float64, bool) {
	s = strings.TrimSpace(s)
	if !looksAmount(s) {
		return 0, false
	}
	neg := strings.HasPrefix(s, "(") || strings.ContainsAny(s, "-−")
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' || r == ',' {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	// A cell with both separators writes its decimals after the last one, whatever the column does.
	if strings.Contains(digits, ".") && strings.Contains(digits, ",") {
		dec = digits[strings.LastIndexAny(digits, ".,")]
	}
	thousands := ","
	if dec == ',' {
		thousands = "."
	}
	digits = strings.ReplaceAll(digits, thousands, "")
	digits = strings.Replace(digits, string(dec), ".", 1)
	v, err := strconv.ParseFloat(digits, 64)
	if err != nil {
		return 0, false
	}
	if neg {
		v = -v
	}
	return v, true
}
