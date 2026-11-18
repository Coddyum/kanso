package main

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Admin says which administrative due dates apply to the business.
type Admin struct {
	Urssaf string `json:"urssaf"` // quarterly | monthly | off
	CFE    bool   `json:"cfe"`
	IR     bool   `json:"ir"`
}

// Deadline is an administrative due date: a declaration to file or a tax to pay.
type Deadline struct {
	Key    string `json:"key"`
	Kind   string `json:"kind"` // urssaf | cfe | ir
	Date   string `json:"date"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	URL    string `json:"url"`
	Approx bool   `json:"approx,omitempty"` // a forecast: the official date is not published yet
	Status string `json:"status"`           // todo | done | past
}

var monthNames = []string{"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"}

// fromMonth writes « de mars », « d'avril ».
func fromMonth(m int) string {
	if strings.ContainsRune("aeiouéè", []rune(monthNames[m])[0]) {
		return "d'" + monthNames[m]
	}
	return "de " + monthNames[m]
}

func (d DB) admin() Admin {
	if d.Admin != nil {
		return *d.Admin
	}
	return Admin{Urssaf: "quarterly", CFE: true, IR: true}
}

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
	day := (h+l-7*m+114)%31 + 1
	return time.Date(y, time.Month(month), day, 0, 0, 0, 0, time.Local)
}

// isHoliday says whether t is a public holiday in mainland France.
func isHoliday(t time.Time) bool {
	switch t.Format("01-02") {
	case "01-01", "05-01", "05-08", "07-14", "08-15", "11-01", "11-11", "12-25":
		return true
	}
	e := easterSunday(t.Year())
	for _, off := range []int{1, 39, 50} { // Easter Monday, Ascension, Whit Monday
		if dayOf(t).Equal(e.AddDate(0, 0, off)) {
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

func lastDayOf(y int, m time.Month) time.Time {
	return time.Date(y, m+1, 1, 0, 0, 0, 0, time.Local).AddDate(0, 0, -1)
}

// deadlines lists the administrative due dates between from and to:
//   - the Urssaf turnover declaration, due the last day of the month after the
//     quarter (or the month), moved to the next working day;
//   - the CFE, to pay by 15 December;
//   - the yearly income tax return (2042-C-PRO), around the end of May: a forecast.
func deadlines(a Admin, done map[string]bool, from, to, today time.Time) []Deadline {
	out := []Deadline{}
	add := func(kind string, d time.Time, title, detail, url string, approx bool) {
		if d.Before(from) || d.After(to) {
			return
		}
		key := kind + "@" + d.Format(dayFmt)
		status := "todo"
		switch {
		case done[key]:
			status = "done"
		case d.Before(today):
			status = "past"
		}
		out = append(out, Deadline{Key: key, Kind: kind, Date: d.Format(dayFmt), Title: title, Detail: detail, URL: url, Approx: approx, Status: status})
	}
	const urssafURL = "https://www.autoentrepreneur.urssaf.fr"
	for y := from.Year() - 1; y <= to.Year(); y++ {
		switch a.Urssaf {
		case "quarterly":
			for q := 0; q < 4; q++ {
				first := time.Month(3*q + 1)
				due := workingDay(lastDayOf(y, first+3))
				detail := fmt.Sprintf("chiffre d'affaires %s à %s %d, à déclarer même s'il est à 0 €", fromMonth(3*q), monthNames[3*q+2], y)
				add("urssaf", due, "Déclaration URSSAF", detail, urssafURL, false)
			}
		case "monthly":
			for m := time.January; m <= time.December; m++ {
				due := workingDay(lastDayOf(y, m+1))
				detail := fmt.Sprintf("chiffre d'affaires %s %d, à déclarer même s'il est à 0 €", fromMonth(int(m)-1), y)
				add("urssaf", due, "Déclaration URSSAF", detail, urssafURL, false)
			}
		}
		if a.CFE {
			detail := fmt.Sprintf("payer l'avis dans ton espace professionnel impots.gouv (en ligne début novembre) · rien à payer si ton chiffre d'affaires de %d ne dépasse pas 5 000 €", y-2)
			add("cfe", time.Date(y, time.December, 15, 0, 0, 0, 0, time.Local), "CFE à payer", detail, "https://www.impots.gouv.fr/professionnel", false)
		}
		if a.IR {
			detail := fmt.Sprintf("déclarer le chiffre d'affaires de %d sur la 2042-C-PRO · date limite entre fin mai et début juin selon ton département", y-1)
			add("ir", time.Date(y, time.May, 20, 0, 0, 0, 0, time.Local), "Déclaration de revenus", detail, "https://www.impots.gouv.fr", true)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

func (m *model) deadlines(from, to time.Time) []Deadline {
	return deadlines(m.db.admin(), m.db.Done, from, to, m.today)
}

// handleDeadline ticks a due date as done, or unticks it.
func (a *App) handleDeadline(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key  string `json:"key"`
		Done bool   `json:"done"`
	}
	if !readJSON(w, r, &req) || req.Key == "" {
		return
	}
	err := a.store.update(func(db *DB) error {
		if req.Done {
			db.Done[req.Key] = true
		} else {
			delete(db.Done, req.Key)
		}
		return nil
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// handleAdmin saves which administrative due dates apply.
func (a *App) handleAdmin(w http.ResponseWriter, r *http.Request) {
	var req Admin
	if !readJSON(w, r, &req) {
		return
	}
	if req.Urssaf != "quarterly" && req.Urssaf != "monthly" && req.Urssaf != "off" {
		fail(w, http.StatusBadRequest, errors.New("rythme URSSAF inconnu"))
		return
	}
	err := a.store.update(func(db *DB) error {
		db.Admin = &req
		return nil
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, req)
}
