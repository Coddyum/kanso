// Package demo builds a folder of made-up but credible data: a year of
// invoices of a small business's subscriptions, and a database that follows
// them. Nothing in it comes from the user. It lives beside the config, never
// in the user's folder; the Settings screen opens the app on it, and
// `kanso demo` serves it for the README's screenshots.
package demo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/coddyum/kanso/internal/brand"
	"github.com/coddyum/kanso/internal/config"
	"github.com/coddyum/kanso/internal/day"
	"github.com/coddyum/kanso/internal/deadlines"
	"github.com/coddyum/kanso/internal/i18n"
	"github.com/coddyum/kanso/internal/ledger"
)

// marker, in the folder's .kanso, says the folder is a demo: only such a folder is ever replaced.
const marker = "demo"

// Dir is where the demo folder lives: in the app's own folder of Application Support.
func Dir(home string) string { return filepath.Join(config.Dir(home), "demo", "Demo") }

// Is says whether dir is a demo folder.
func Is(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, brand.DataDir, marker))
	return err == nil
}

// service is a made-up subscription and how its invoices read.
type service struct {
	name, folder, legal string
	domain              string // its site: the app shows its logo
	amount              float64
	currency            string
	cycle               string // monthly | yearly
	day, month          int    // the billing day; the month of a yearly one
	english             bool
	label               string
	project             string
	missingLast         bool // its latest invoice is not in the folder: the « missing » screen shows it
}

var services = []service{
	{"Notion", "Notion", "Notion Labs, Inc.", "notion.com", 10, "USD", "monthly", 4, 0, true, "Plus plan (monthly)", "product", false},
	{"Figma", "Figma", "Figma, Inc.", "figma.com", 15, "EUR", "monthly", 12, 0, true, "Professional plan, 1 editor", "product", true},
	{"GitHub", "GitHub", "GitHub, Inc.", "github.com", 4, "USD", "monthly", 21, 0, true, "GitHub Pro", "product", false},
	{"Render", "Render", "Render Services, Inc.", "render.com", 7, "USD", "monthly", 1, 0, true, "Starter web service", "product", false},
	{"Free Mobile", "Free Mobile", "Free Mobile", "free.fr", 19.99, "EUR", "monthly", 5, 0, false, "Forfait Free 5G", "office", false},
	{"Proton", "Proton", "Proton AG", "proton.me", 9.99, "EUR", "monthly", 10, 0, true, "Proton Unlimited (monthly)", "office", false},
	{"OVHcloud", "OVHcloud", "OVHcloud", "ovhcloud.com", 14.99, "EUR", "yearly", 8, 3, false, "Hébergement Perso", "product", false},
	{"Apple", "Apple", "Apple Distribution International", "apple.com", 99, "EUR", "yearly", 15, 6, true, "Apple Developer Program", "product", false},
}

// Build writes the demo folder at dir, from scratch, dated from today; its
// projects and notes are in tr's language, its invoices in French or English
// like real ones. A folder already there is replaced only when it is a demo.
func Build(dir string, today time.Time, tr i18n.Lang) error {
	today = day.Of(today)
	if _, err := os.Stat(dir); err == nil {
		if !Is(dir) {
			return fmt.Errorf("demo: %s is not a demo folder: left alone", dir)
		}
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("demo: %w", err)
		}
	}
	db := ledger.DB{Assign: map[string]string{}}
	ledger.Fill(&db)
	projects := map[string]string{"product": ledger.NewID(), "office": ledger.NewID()}
	db.Projects = []ledger.Project{
		{ID: projects["product"], Name: tr.T("demo.product"), Color: "violet"},
		{ID: projects["office"], Name: tr.T("demo.office"), Color: "amber"},
	}
	for _, s := range services {
		dates := s.dates(today)
		for i, d := range dates {
			if s.missingLast && i == len(dates)-1 {
				continue
			}
			if err := s.write(dir, d, i+1, tr.List("folder.months")); err != nil {
				return err
			}
		}
		db.Payments = append(db.Payments, ledger.Payment{
			ID: ledger.NewID(), Name: s.name, Domain: s.domain, Amount: s.amount, Currency: s.currency, Cycle: s.cycle,
			Start: dates[0].Format(day.Format), ExpectInvoice: true,
		})
		db.Assign["svc:"+ledger.ServiceKey(s.name)] = projects[s.project]
	}
	if err := laptop(dir, today, tr.List("folder.months")); err != nil {
		return err
	}
	// A yearly payment with no invoice yet: the professional insurance, due next month.
	insurance := time.Date(today.Year(), today.Month()+1, 15, 0, 0, 0, 0, time.Local)
	db.Payments = append(db.Payments, ledger.Payment{
		ID: ledger.NewID(), Name: "Hiscox", Domain: "hiscox.fr", Amount: 180, Currency: "EUR", Cycle: "yearly",
		Start: insurance.Format(day.Format), Note: tr.T("demo.insurance"),
	})
	db.Reserve = &ledger.Reserve{Amount: 6840, Date: today.Format(day.Format)}
	db.Admin = &deadlines.Settings{Urssaf: "quarterly", CFE: true, IR: true, Created: today.AddDate(-2, 0, 0).Format(day.Format)}
	db.Todos = []ledger.Todo{{ID: ledger.NewID(), Text: tr.T("demo.todo"), Created: today.Format(day.Format)}}
	steps := map[string]string{}
	for _, id := range ledger.OnboardingSteps {
		steps[id] = "done"
	}
	db.Onboarding = &ledger.Onboarding{Steps: steps, Closed: true}
	return writeDB(dir, db)
}

// dates are the billing days of a service over the past year, oldest first, today included.
func (s service) dates(today time.Time) []time.Time {
	var out []time.Time
	if s.cycle == "yearly" {
		for y := today.Year() - 1; y <= today.Year(); y++ {
			if d := time.Date(y, time.Month(s.month), s.day, 0, 0, 0, 0, time.Local); !d.After(today) {
				out = append(out, d)
			}
		}
		return out
	}
	first := time.Date(today.Year(), today.Month(), s.day, 0, 0, 0, 0, time.Local)
	for k := 11; k >= 0; k-- {
		if d := day.AddMonths(first, -k); !d.After(today) {
			out = append(out, d)
		}
	}
	return out
}

// write files one invoice of a service in YEAR/Vendor/Month, dated d.
func (s service) write(dir string, d time.Time, n int, months []string) error {
	name, lines := s.invoice(d, n)
	return file(dir, s.folder, d, name, lines, months)
}

// invoice is the file name and the lines of a service's n-th invoice, dated d.
func (s service) invoice(d time.Time, n int) (name string, lines []string) {
	number := fmt.Sprintf("%s-%d%02d-%03d", strings.ToUpper(s.folder[:2]), d.Year(), int(d.Month()), n)
	if s.english {
		symbol := map[string]string{"USD": "$", "EUR": "€"}[s.currency]
		lines = []string{s.legal, "Invoice", "Invoice number: " + number, "Date of issue: " + d.Format("January 2, 2006"),
			"Description", s.label, fmt.Sprintf("Total %s%.2f", symbol, s.amount)}
		name = s.folder + "-Invoice-" + number + ".pdf"
	} else {
		lines = []string{s.legal, "Facture", "Numéro de facture : " + number, "Date : " + d.Format("02/01/2006"),
			s.label, "Total TTC : " + euros(s.amount)}
		name = "Facture-" + number + ".pdf"
	}
	return name, lines
}

// laptop is a one-off purchase two months ago: the analytics show more than subscriptions.
func laptop(dir string, today time.Time, months []string) error {
	d := time.Date(today.Year(), today.Month()-2, 23, 0, 0, 0, 0, time.Local)
	lines := []string{"LDLC.com", "Facture", "Numéro de facture : LD-" + d.Format("20060102") + "-7781", "Date : " + d.Format("02/01/2006"),
		"Ordinateur portable 14 pouces", "Total TTC : " + euros(1249)}
	return file(dir, "LDLC", d, "Facture-LDLC-"+d.Format("2006-01")+".pdf", lines, months)
}

// file writes an invoice in YEAR/Vendor/Month, the month named by months (day.MonthFolders when not 12).
func file(dir, vendor string, d time.Time, name string, lines []string, months []string) error {
	if len(months) != 12 {
		months = day.MonthFolders
	}
	folder := filepath.Join(dir, strconv.Itoa(d.Year()), vendor, months[d.Month()-1])
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return fmt.Errorf("demo: %w", err)
	}
	p := filepath.Join(folder, name)
	if err := os.WriteFile(p, pdf(lines), 0o644); err != nil {
		return fmt.Errorf("demo: %w", err)
	}
	stamp := d.Add(9 * time.Hour)
	return os.Chtimes(p, stamp, stamp)
}

// euros writes an amount the French way: « 1 249,00 € ».
func euros(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	whole, cents, _ := strings.Cut(s, ".")
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + " " + whole[i:]
	}
	return whole + "," + cents + " €"
}

func writeDB(dir string, db ledger.DB) error {
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	kanso := filepath.Join(dir, brand.DataDir)
	if err := os.MkdirAll(kanso, 0o755); err != nil {
		return fmt.Errorf("demo: %w", err)
	}
	if err := os.WriteFile(filepath.Join(kanso, marker), []byte("Made-up data: this folder can be replaced at any time.\n"), 0o644); err != nil {
		return fmt.Errorf("demo: %w", err)
	}
	return os.WriteFile(filepath.Join(kanso, "db.json"), data, 0o644)
}
