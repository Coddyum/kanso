package ledger

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/coddyum/kanso/internal/day"
	"github.com/coddyum/kanso/internal/rates"
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

// Reminders lists the notifications of the next two months, all at 9:00:
//   - the day before a payment (zero amounts left out; several on one day: one notification);
//   - 7 days and 1 day before an administrative deadline still to do;
//   - every Monday while invoices are missing;
//   - 30, 14, 7 and 1 days before the reserve runs out.
//
// Only reminders still to come after now are listed.
func (m *Model) Reminders(r rates.Rates, now time.Time) []Reminder {
	out := []Reminder{}
	add := func(id string, d time.Time, title, body, view string) {
		at := time.Date(d.Year(), d.Month(), d.Day(), reminderHour, 0, 0, 0, time.Local)
		if at.After(now) {
			out = append(out, Reminder{ID: id, At: at.Format("2006-01-02T15:04"), Title: title, Body: body, View: view})
		}
	}
	horizon := m.Today.AddDate(0, 2, 0)
	m.paymentReminders(horizon, add)
	m.deadlineReminders(horizon, add)
	m.missingReminders(add)
	if f := m.Forecast(r); f.RunsOut != "" {
		out0, _ := day.Parse(f.RunsOut)
		for _, before := range []int{30, 14, 7, 1} {
			body := fmt.Sprintf("Les paiements prévus vident le compte pro dans %d jours : pense à l'alimenter.", before)
			add(fmt.Sprintf("reserve@%s@%d", f.RunsOut, before), out0.AddDate(0, 0, -before), "Réserve à 0 le "+shortDate(out0), body, "calendar")
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}

type addReminder func(id string, d time.Time, title, body, view string)

// paymentReminders: the day before each upcoming payment that costs something, one notification per day.
func (m *Model) paymentReminders(horizon time.Time, add addReminder) {
	byDay := map[string][]DueItem{}
	for _, p := range m.DB.Payments {
		for _, d := range p.Dues(m.Today.AddDate(0, 0, 1), horizon) {
			if it := m.dueItem(p, d); it.Status == "upcoming" && it.Amount != nil && *it.Amount > 0 {
				byDay[it.Date] = append(byDay[it.Date], it)
			}
		}
	}
	price := func(it DueItem) string {
		s := moneyFR(*it.Amount, it.Currency)
		if it.Estimate {
			s = "≈ " + s
		}
		return s
	}
	for date, items := range byDay {
		d, _ := day.Parse(date)
		if len(items) == 1 {
			add("pay@"+date, d.AddDate(0, 0, -1), "Demain : "+items[0].Name, price(items[0]), "calendar")
			continue
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
		var parts []string
		for _, it := range items {
			parts = append(parts, it.Name+" "+price(it))
		}
		add("pay@"+date, d.AddDate(0, 0, -1), fmt.Sprintf("Demain : %d paiements", len(items)), strings.Join(parts, " · "), "calendar")
	}
}

// deadlineReminders: 7 days and 1 day before each administrative deadline still to do.
func (m *Model) deadlineReminders(horizon time.Time, add addReminder) {
	for _, dl := range m.Deadlines(m.Today, horizon.AddDate(0, 0, 7)) {
		if dl.Status != "todo" {
			continue
		}
		d, _ := day.Parse(dl.Date)
		body := dl.Detail
		if dl.Approx {
			body += " (date prévisionnelle)"
		}
		add(dl.Key+"@j7", d.AddDate(0, 0, -7), fmt.Sprintf("%s dans 7 jours, le %s", dl.Title, shortDate(d)), body, "calendar")
		add(dl.Key+"@j1", d.AddDate(0, 0, -1), dl.Title+" demain", body, "calendar")
	}
}

// missingReminders: every Monday of the next 8 weeks while invoices are
// missing, naming the first four services.
func (m *Model) missingReminders(add addReminder) {
	var missing []string
	seen := map[string]bool{}
	for _, p := range m.DB.Payments {
		for _, d := range p.Dues(m.Today.AddDate(-3, 0, 0), m.Today) {
			if m.dueStatus(p, d).Status == "missing" {
				missing = append(missing, p.Name)
				seen[p.Name] = true
			}
		}
	}
	n := len(missing)
	if n == 0 {
		return
	}
	var names []string
	for _, p := range m.DB.Payments {
		if seen[p.Name] && len(names) < 4 {
			names = append(names, p.Name)
			delete(seen, p.Name)
		}
	}
	body := strings.Join(names, ", ")
	if len(seen) > 0 {
		body += "…"
	}
	monday := m.Today.AddDate(0, 0, (8-int(m.Today.Weekday()))%7)
	title := fmt.Sprintf("%d facture%s à récupérer", n, map[bool]string{true: "s"}[n > 1])
	for w := 0; w < 8; w++ {
		d := monday.AddDate(0, 0, 7*w)
		add("missing@"+d.Format(day.Format), d, title, body, "missing")
	}
}
