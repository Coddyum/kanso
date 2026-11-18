package main

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Reminder is a notification the Mac app schedules: when, what, and the screen a click opens.
type Reminder struct {
	ID    string `json:"id"`
	At    string `json:"at"` // local time, 2006-01-02T15:04
	Title string `json:"title"`
	Body  string `json:"body"`
	View  string `json:"view"`
}

var monthShort = []string{"janv.", "févr.", "mars", "avr.", "mai", "juin", "juil.", "août", "sept.", "oct.", "nov.", "déc."}

const reminderHour = 9

// moneyFR writes « 1 234,50 € », « 19,08 $ ».
func moneyFR(v float64, cur string) string {
	s := strconv.FormatFloat(math.Abs(v), 'f', 2, 64)
	whole, dec, _ := strings.Cut(s, ".")
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + " " + whole[i:]
	}
	sym := map[string]string{"EUR": "€", "USD": "$", "GBP": "£"}[cur]
	if sym == "" {
		sym = cur
	}
	return whole + "," + dec + " " + sym
}

func shortDate(t time.Time) string { return fmt.Sprintf("%d %s", t.Day(), monthShort[t.Month()-1]) }

// reminders lists the notifications of the next two months, all at 9:00:
//   - the day before a payment (zero amounts left out; several on one day: one notification);
//   - 7 days and 1 day before an administrative deadline still to do;
//   - every Monday while invoices are missing;
//   - 30, 14, 7 and 1 days before the reserve runs out.
//
// Only reminders still to come are listed.
func (m *model) reminders(rates Rates, now time.Time) []Reminder {
	out := []Reminder{}
	add := func(id string, day time.Time, title, body, view string) {
		at := time.Date(day.Year(), day.Month(), day.Day(), reminderHour, 0, 0, 0, time.Local)
		if at.After(now) {
			out = append(out, Reminder{ID: id, At: at.Format("2006-01-02T15:04"), Title: title, Body: body, View: view})
		}
	}
	horizon := m.today.AddDate(0, 2, 0)

	byDay := map[string][]DueItem{}
	for _, p := range m.db.Payments {
		for _, d := range p.dues(m.today.AddDate(0, 0, 1), horizon) {
			if it := m.dueItem(p, d); it.Status == "upcoming" && it.Amount != nil && *it.Amount > 0 {
				byDay[it.Date] = append(byDay[it.Date], it)
			}
		}
	}
	for date, items := range byDay {
		day, _ := parseDay(date)
		price := func(it DueItem) string {
			s := moneyFR(*it.Amount, it.Currency)
			if it.Estimate {
				s = "≈ " + s
			}
			return s
		}
		if len(items) == 1 {
			add("pay@"+date, day.AddDate(0, 0, -1), "Demain : "+items[0].Name, price(items[0]), "calendar")
			continue
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
		var parts []string
		for _, it := range items {
			parts = append(parts, it.Name+" "+price(it))
		}
		add("pay@"+date, day.AddDate(0, 0, -1), fmt.Sprintf("Demain : %d paiements", len(items)), strings.Join(parts, " · "), "calendar")
	}

	for _, d := range m.deadlines(m.today, horizon.AddDate(0, 0, 7)) {
		if d.Status != "todo" {
			continue
		}
		day, _ := parseDay(d.Date)
		body := d.Detail
		if d.Approx {
			body += " (date prévisionnelle)"
		}
		add(d.Key+"@j7", day.AddDate(0, 0, -7), fmt.Sprintf("%s dans 7 jours, le %s", d.Title, shortDate(day)), body, "calendar")
		add(d.Key+"@j1", day.AddDate(0, 0, -1), d.Title+" demain", body, "calendar")
	}

	var missing []string
	seen := map[string]bool{}
	for _, p := range m.db.Payments {
		for _, d := range p.dues(m.today.AddDate(-3, 0, 0), m.today) {
			if m.dueStatus(p, d).Status == "missing" {
				missing = append(missing, p.Name)
				seen[p.Name] = true
			}
		}
	}
	if n := len(missing); n > 0 {
		var names []string
		for _, p := range m.db.Payments {
			if seen[p.Name] && len(names) < 4 {
				names = append(names, p.Name)
				delete(seen, p.Name)
			}
		}
		body := strings.Join(names, ", ")
		if len(seen) > 0 {
			body += "…"
		}
		monday := m.today.AddDate(0, 0, (8-int(m.today.Weekday()))%7)
		for w := 0; w < 8; w++ {
			day := monday.AddDate(0, 0, 7*w)
			title := fmt.Sprintf("%d facture%s à récupérer", n, map[bool]string{true: "s"}[n > 1])
			add("missing@"+day.Format(dayFmt), day, title, body, "missing")
		}
	}

	if f := m.forecast(rates); f.RunsOut != "" {
		out0, _ := parseDay(f.RunsOut)
		for _, before := range []int{30, 14, 7, 1} {
			day := out0.AddDate(0, 0, -before)
			body := fmt.Sprintf("Les paiements prévus vident le compte pro dans %d jours : pense à l'alimenter.", before)
			add(fmt.Sprintf("reserve@%s@%d", f.RunsOut, before), day, "Réserve à 0 le "+shortDate(out0), body, "calendar")
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}

func (a *App) handleReminders(w http.ResponseWriter, r *http.Request) {
	m, err := a.buildModel()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, m.reminders(a.rates(), time.Now()))
}
