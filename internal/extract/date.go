package extract

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/coddyum/kanso/internal/day"
)

const monthRe = `(jan[a-z]*|f[eé]v[a-zé]*|feb[a-z]*|mar[a-z]*|apr[a-z]*|avr[a-z]*|ma[iy]|juin|june?|juil[a-z]*|july?|ao[uû]t|aug[a-z]*|sept?[a-z]*|oct[a-z]*|nov[a-z]*|d[eé]c[a-zé]*)\.?`

var (
	reMDY = regexp.MustCompile(`(?i)\b` + monthRe + `\s+(\d{1,2}),?\s+(\d{4})\b`)
	reDMY = regexp.MustCompile(`(?i)\b(\d{1,2})(?:er)?\s+` + monthRe + `\s+(\d{4})\b`)
	reNum = regexp.MustCompile(`\b(\d{1,2})/(\d{1,2})/(\d{4})\b`)
	reISO = regexp.MustCompile(`\b(\d{4})-(\d{2})-(\d{2})\b`)

	rePeriodEN = regexp.MustCompile(`(?i)\b` + monthRe + `\s+(\d{1,2}),?\s*(?:(\d{4})\s*)?[–-]\s*(?:` + monthRe + `\s+)?(\d{1,2}),?\s+(\d{4})\b`)
	rePeriodFR = regexp.MustCompile(`(?i)\b(\d{1,2})(?:er)?\s+` + monthRe + `\s+(?:(\d{4})\s*)?[–-]\s*(\d{1,2})(?:er)?\s+` + monthRe + `\s+(\d{4})\b`)
)

var dateLabels = []string{"date paid", "date de paiement", "payé le", "date of issue", "invoice date", "émission", "emission", "date de facture", "issue date", "date:", "généré le", "date"}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

// detectDate takes the date after the first label that has one (« date paid »,
// « date de facture »…), or else the first date of the text.
func detectDate(text string) string {
	lt := strings.ToLower(text)
	for _, l := range dateLabels {
		if i := strings.Index(lt, l); i >= 0 {
			if d := firstDate(lt[i:min(len(lt), i+len(l)+48)]); d != "" {
				return d
			}
		}
	}
	return firstDate(lt)
}

// firstDate returns the earliest valid date written in s, whatever its format.
func firstDate(s string) string {
	best, bestAt := "", -1
	try := func(re *regexp.Regexp, conv func(m []string) string) {
		for _, loc := range re.FindAllStringSubmatchIndex(s, -1) {
			if bestAt >= 0 && loc[0] >= bestAt {
				return
			}
			m := make([]string, len(loc)/2)
			for i := range m {
				if loc[2*i] >= 0 {
					m[i] = s[loc[2*i]:loc[2*i+1]]
				}
			}
			if d := conv(m); d != "" {
				best, bestAt = d, loc[0]
				return
			}
		}
	}
	try(reMDY, func(m []string) string { return day.YMD(atoi(m[3]), day.MonthOf(m[1]), atoi(m[2])) })
	try(reDMY, func(m []string) string { return day.YMD(atoi(m[3]), day.MonthOf(m[2]), atoi(m[1])) })
	try(reNum, func(m []string) string { return day.YMD(atoi(m[3]), atoi(m[2]), atoi(m[1])) })
	try(reISO, func(m []string) string { return day.YMD(atoi(m[1]), atoi(m[2]), atoi(m[3])) })
	return best
}

// detectPeriod returns the length in days of the first billed period written
// as « Mon D–Mon D, YYYY », « Mon D – D, YYYY » or « D mois YYYY–D mois YYYY ».
func detectPeriod(text string) int {
	span := func(y1, m1, d1, y2, m2, d2 int) int {
		if m1 == 0 || m2 == 0 {
			return 0
		}
		if y1 == 0 {
			y1 = y2
			if m1 > m2 {
				y1--
			}
		}
		from, ok1 := day.Parse(day.YMD(y1, m1, d1))
		to, ok2 := day.Parse(day.YMD(y2, m2, d2))
		if days := day.Between(from, to); ok1 && ok2 && days > 0 && days <= 400 {
			return days
		}
		return 0
	}
	for _, m := range rePeriodEN.FindAllStringSubmatch(text, 5) {
		m2 := day.MonthOf(m[1])
		if m[4] != "" {
			m2 = day.MonthOf(m[4])
		}
		if d := span(atoi(m[3]), day.MonthOf(m[1]), atoi(m[2]), atoi(m[6]), m2, atoi(m[5])); d > 0 {
			return d
		}
	}
	for _, m := range rePeriodFR.FindAllStringSubmatch(text, 5) {
		if d := span(atoi(m[3]), day.MonthOf(m[2]), atoi(m[1]), atoi(m[6]), day.MonthOf(m[5]), atoi(m[4])); d > 0 {
			return d
		}
	}
	return 0
}
