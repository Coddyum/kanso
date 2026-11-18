// Package deadlines computes the administrative due dates of a French
// micro-entrepreneur: the Urssaf declarations, the CFE and the income tax return.
package deadlines

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/coddyum/kanso/internal/day"
)

// Settings says which administrative due dates apply to the business.
type Settings struct {
	Urssaf string `json:"urssaf"` // quarterly | monthly | off
	CFE    bool   `json:"cfe"`
	IR     bool   `json:"ir"`
}

// Default is what a new business follows: quarterly Urssaf declarations, CFE and income tax.
func Default() Settings { return Settings{Urssaf: "quarterly", CFE: true, IR: true} }

// Valid says whether the Urssaf rhythm is one the app knows.
func (s Settings) Valid() bool {
	return s.Urssaf == "quarterly" || s.Urssaf == "monthly" || s.Urssaf == "off"
}

// Deadline is an administrative due date: a declaration to file or a tax to pay.
type Deadline struct {
	Key    string `json:"key"`  // kind@YYYY-MM-DD: what « done » is remembered by
	Kind   string `json:"kind"` // urssaf | cfe | ir
	Date   string `json:"date"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	URL    string `json:"url"`
	Approx bool   `json:"approx,omitempty"` // a forecast: the official date is not published yet
	Status string `json:"status"`           // todo | done | past
}

var monthNames = []string{"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"}

// fromMonth writes « de mars », « d'avril » (m from 0).
func fromMonth(m int) string {
	if strings.ContainsRune("aeiouéè", []rune(monthNames[m])[0]) {
		return "d'" + monthNames[m]
	}
	return "de " + monthNames[m]
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
// A key in done is ticked; an unticked one before today is past.
func List(s Settings, done map[string]bool, from, to, today time.Time) []Deadline {
	out := []Deadline{}
	add := func(kind string, d time.Time, title, detail, url string, approx bool) {
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
		out = append(out, Deadline{Key: key, Kind: kind, Date: d.Format(day.Format), Title: title, Detail: detail, URL: url, Approx: approx, Status: status})
	}
	const urssafURL = "https://www.autoentrepreneur.urssaf.fr"
	for y := from.Year() - 1; y <= to.Year(); y++ {
		switch s.Urssaf {
		case "quarterly":
			for q := 0; q < 4; q++ {
				first := time.Month(3*q + 1)
				due := workingDay(lastDayOf(y, first+3))
				detail := fmt.Sprintf("chiffre d'affaires %s à %s %d, à déclarer même s'il est à 0\u00a0€", fromMonth(3*q), monthNames[3*q+2], y)
				add("urssaf", due, "Déclaration URSSAF", detail, urssafURL, false)
			}
		case "monthly":
			for m := time.January; m <= time.December; m++ {
				due := workingDay(lastDayOf(y, m+1))
				detail := fmt.Sprintf("chiffre d'affaires %s %d, à déclarer même s'il est à 0\u00a0€", fromMonth(int(m)-1), y)
				add("urssaf", due, "Déclaration URSSAF", detail, urssafURL, false)
			}
		}
		if s.CFE {
			detail := fmt.Sprintf("payer l'avis dans ton espace professionnel impots.gouv (en ligne début novembre) · rien à payer si ton chiffre d'affaires de %d ne dépasse pas 5\u00a0000\u00a0€", y-2)
			add("cfe", time.Date(y, time.December, 15, 0, 0, 0, 0, time.Local), "CFE à payer", detail, "https://www.impots.gouv.fr/professionnel", false)
		}
		if s.IR {
			detail := fmt.Sprintf("déclarer le chiffre d'affaires de %d sur la 2042-C-PRO · date limite entre fin mai et début juin selon ton département", y-1)
			add("ir", time.Date(y, time.May, 20, 0, 0, 0, 0, time.Local), "Déclaration de revenus", detail, "https://www.impots.gouv.fr", true)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}
