// Package deadlines computes the administrative due dates of a French
// micro-entrepreneur: the Urssaf declarations, the CFE and the income tax return.
package deadlines

import (
	"sort"
	"time"

	"github.com/coddyum/kanso/internal/day"
	"github.com/coddyum/kanso/internal/i18n"
)

// Settings says which administrative due dates apply to the business.
type Settings struct {
	Urssaf  string `json:"urssaf"` // quarterly | monthly | off
	CFE     bool   `json:"cfe"`
	IR      bool   `json:"ir"`
	Created string `json:"created,omitempty"` // the business's creation day: nothing is due for before it
}

// Default is what a new business follows: quarterly Urssaf declarations, CFE and income tax.
func Default() Settings { return Settings{Urssaf: "quarterly", CFE: true, IR: true} }

// Valid says whether the Urssaf rhythm is one the app knows and the creation day, when given, is a day.
func (s Settings) Valid() bool {
	if _, ok := day.Parse(s.Created); s.Created != "" && !ok {
		return false
	}
	return s.Urssaf == "quarterly" || s.Urssaf == "monthly" || s.Urssaf == "off"
}

// Deadline is an administrative due date: a declaration to file or a tax to pay.
type Deadline struct {
	Key    string `json:"key"`  // kind@YYYY-MM-DD: what « done » is remembered by
	Kind   string `json:"kind"` // urssaf | cfe | ir
	Date   string `json:"date"`
	Title  string `json:"title"`
	Short  string `json:"short"` // the title in a word or two, for a small space
	Detail string `json:"detail"`
	URL    string `json:"url"`
	Approx bool   `json:"approx,omitempty"` // a forecast: the official date is not published yet
	Status string `json:"status"`           // todo | done | past
}

func lastDayOf(y int, m time.Month) time.Time {
	return time.Date(y, m+1, 1, 0, 0, 0, 0, time.Local).AddDate(0, 0, -1)
}

// List gives the administrative due dates between from and to, oldest first:
//   - the Urssaf turnover declaration, due the last day of the month after the
//     quarter (or the month), moved to the next working day;
//   - the CFE, to pay by 15 December;
//   - the yearly income tax return (2042-C-PRO), around the end of May: a forecast.
//
// A key in done is ticked; an unticked one before today is past. With a
// creation day, a declaration whose period ended before it is left out, and so
// is the CFE of the creation year (a new business does not pay it). The texts
// are in tr's language; the rules stay French whatever the language.
func List(s Settings, done map[string]bool, from, to, today time.Time, tr i18n.Lang) []Deadline {
	out := []Deadline{}
	months, monthsFrom := tr.List("date.months"), tr.List("date.monthsFrom")
	created, hasCreated := day.Parse(s.Created)
	// before says whether a period ending on periodEnd is over before the business existed.
	before := func(periodEnd time.Time) bool { return hasCreated && periodEnd.Before(created) }
	add := func(kind string, d time.Time, detail, url string, approx bool) {
		if d.Before(from) || d.After(to) {
			return
		}
		key := kind + "@" + d.Format(day.Format)
		status := "todo"
		switch {
		case done[key]:
			status = "done"
		case d.Before(today):
			status = "past"
		}
		out = append(out, Deadline{
			Key: key, Kind: kind, Date: d.Format(day.Format), Title: tr.T("deadline." + kind + ".title"), Short: tr.T("deadline." + kind + ".short"),
			Detail: detail, URL: url, Approx: approx, Status: status,
		})
	}
	const urssafURL = "https://www.autoentrepreneur.urssaf.fr"
	for y := from.Year() - 1; y <= to.Year(); y++ {
		switch s.Urssaf {
		case "quarterly":
			for q := 0; q < 4; q++ {
				first := time.Month(3*q + 1)
				if before(lastDayOf(y, first+2)) {
					continue
				}
				due := workingDay(lastDayOf(y, first+3))
				detail := tr.T("deadline.urssaf.quarter", "from", monthsFrom[3*q], "to", months[3*q+2], "year", y)
				add("urssaf", due, detail, urssafURL, false)
			}
		case "monthly":
			for m := time.January; m <= time.December; m++ {
				if before(lastDayOf(y, m)) {
					continue
				}
				due := workingDay(lastDayOf(y, m+1))
				detail := tr.T("deadline.urssaf.month", "from", monthsFrom[m-1], "year", y)
				add("urssaf", due, detail, urssafURL, false)
			}
		}
		if s.CFE && (!hasCreated || y > created.Year()) {
			detail := tr.T("deadline.cfe.detail", "year", y-2)
			add("cfe", time.Date(y, time.December, 15, 0, 0, 0, 0, time.Local), detail, "https://www.impots.gouv.fr/professionnel", false)
		}
		if s.IR && !before(time.Date(y-1, time.December, 31, 0, 0, 0, 0, time.Local)) {
			detail := tr.T("deadline.ir.detail", "year", y-1)
			add("ir", time.Date(y, time.May, 20, 0, 0, 0, 0, time.Local), detail, "https://www.impots.gouv.fr", true)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}
