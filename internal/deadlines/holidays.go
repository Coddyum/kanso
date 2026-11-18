package deadlines

import (
	"time"

	"github.com/coddyum/kanso/internal/day"
)

// easterSunday is the Gregorian Easter (anonymous algorithm).
func easterSunday(y int) time.Time {
	a, b, c := y%19, y/100, y%100
	d, e := b/4, b%4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i, k := c/4, c%4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	dom := (h+l-7*m+114)%31 + 1
	return time.Date(y, time.Month(month), dom, 0, 0, 0, 0, time.Local)
}

// isHoliday says whether t is a public holiday in mainland France.
func isHoliday(t time.Time) bool {
	switch t.Format("01-02") {
	case "01-01", "05-01", "05-08", "07-14", "08-15", "11-01", "11-11", "12-25":
		return true
	}
	e := easterSunday(t.Year())
	for _, off := range []int{1, 39, 50} { // Easter Monday, Ascension, Whit Monday
		if day.Of(t).Equal(e.AddDate(0, 0, off)) {
			return true
		}
	}
	return false
}

// workingDay moves a due date off a weekend or a public holiday, as the
// Urssaf does: « reportée au premier jour ouvré suivant ».
func workingDay(t time.Time) time.Time {
	for t.Weekday() == time.Saturday || t.Weekday() == time.Sunday || isHoliday(t) {
		t = t.AddDate(0, 0, 1)
	}
	return t
}
