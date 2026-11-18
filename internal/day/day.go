// Package day handles calendar days: local midnights, written YYYY-MM-DD.
package day

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/coddyum/kanso/internal/norm"
)

// Format is how a day is written everywhere: in the database, the API and the file names.
const Format = "2006-01-02"

// Parse reads a YYYY-MM-DD day, at local midnight.
func Parse(s string) (time.Time, bool) {
	t, err := time.ParseInLocation(Format, s, time.Local)
	return t, err == nil
}

// Of is the local midnight of t.
func Of(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

// DaysIn is the number of days of month m (1-12) of year y.
func DaysIn(y, m int) int {
	return time.Date(y, time.Month(m)+1, 0, 0, 0, 0, 0, time.Local).Day()
}

// AddMonths keeps the day of the month, clamped: Jan 31 + 1 month = Feb 28.
func AddMonths(t time.Time, n int) time.Time {
	total := t.Year()*12 + int(t.Month()) - 1 + n
	y, m := total/12, total%12+1
	return time.Date(y, time.Month(m), min(t.Day(), DaysIn(y, m)), 0, 0, 0, 0, time.Local)
}

// Between is the number of days from from to to, negative when to comes first.
func Between(from, to time.Time) int {
	return int(math.Round(to.Sub(from).Hours() / 24))
}

// Gap is the absolute number of days between two YYYY-MM-DD days; huge when
// one of them cannot be read.
func Gap(a, b string) int {
	ta, ok1 := Parse(a)
	tb, ok2 := Parse(b)
	if !ok1 || !ok2 {
		return math.MaxInt32
	}
	d := Between(ta, tb)
	return max(d, -d)
}

// YMD writes a day, or "" when it does not exist or falls outside 2000-2100.
func YMD(y, m, d int) string {
	if y < 2000 || y > 2100 || m < 1 || m > 12 || d < 1 || d > DaysIn(y, m) {
		return ""
	}
	return strconv.Itoa(y) + "-" + pad2(m) + "-" + pad2(d)
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

var monthWords = map[string]int{
	"janvier": 1, "january": 1, "jan": 1, "janv": 1,
	"fevrier": 2, "february": 2, "feb": 2, "fev": 2, "fevr": 2,
	"mars": 3, "march": 3, "mar": 3,
	"avril": 4, "april": 4, "apr": 4, "avr": 4,
	"mai": 5, "may": 5,
	"juin": 6, "june": 6, "jun": 6,
	"juillet": 7, "july": 7, "jul": 7, "juil": 7,
	"aout": 8, "august": 8, "aug": 8,
	"septembre": 9, "september": 9, "sep": 9, "sept": 9,
	"octobre": 10, "october": 10, "oct": 10,
	"novembre": 11, "november": 11, "nov": 11,
	"decembre": 12, "december": 12, "dec": 12,
}

// MonthFolders names the month folders the app creates.
var MonthFolders = []string{"Janvier", "Fevrier", "Mars", "Avril", "Mai", "Juin", "Juillet", "Aout", "Septembre", "Octobre", "Novembre", "Decembre"}

// MonthOf reads a month name, French or English, full or short: 0 when it is not one.
func MonthOf(word string) int { return monthWords[strings.TrimSpace(norm.Words(word))] }

// IsTimeName says whether a folder name only holds a year, a month or a day.
func IsTimeName(s string) bool {
	ws := strings.Fields(norm.Words(s))
	if len(ws) == 0 {
		return false
	}
	for _, w := range ws {
		if _, err := strconv.Atoi(w); err == nil {
			continue
		}
		if monthWords[w] == 0 {
			return false
		}
	}
	return true
}
