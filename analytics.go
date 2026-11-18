package main

import (
	"errors"
	"math"
	"net/http"
	"sort"
	"strings"
)

// Project files expenses together: an app, a client, the company itself.
type Project struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Color    string   `json:"color"`              // a palette name the front knows: violet, sky, emerald…
	Keywords []string `json:"keywords,omitempty"` // besides its name, words that file an expense here on their own
}

var projectColors = []string{"violet", "sky", "emerald", "amber", "rose", "teal", "orange", "indigo", "pink", "lime", "fuchsia", "blue"}

// unfiled is the assignment that keeps an expense out of every project, even when a keyword would file it.
const unfiled = "-"

// Spend is one amount of the analytics: an invoice, a past due date whose
// invoice is not in the folder, or a due date still to come.
type Spend struct {
	Date      string  `json:"date"`
	Service   string  `json:"service"` // the row it adds up in: the payment's name, or the vendor of an invoice no payment claims
	Domain    string  `json:"domain,omitempty"`
	Label     string  `json:"label,omitempty"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
	EUR       float64 `json:"eur"`
	Kind      string  `json:"kind"`            // invoice | estimate | planned
	Cycle     string  `json:"cycle,omitempty"` // the payment's cycle; empty for an invoice no payment claims
	PaymentID string  `json:"paymentId,omitempty"`
	InvoiceID string  `json:"invoiceId,omitempty"`
	Project   string  `json:"project"`   // project id; empty when not filed
	ProjectBy string  `json:"projectBy"` // invoice | service | keyword, or empty
	match     string
}

type analyticsResponse struct {
	Today    string    `json:"today"`
	Projects []Project `json:"projects"`
	Spend    []Spend   `json:"spend"`
	NoAmount int       `json:"noAmount"` // invoices whose amount could not be read: left out
	NoRate   int       `json:"noRate"`   // amounts in a currency with no known rate: left out
	USDToEUR float64   `json:"usdToEur,omitempty"`
	RateDate string    `json:"rateDate,omitempty"`
}

// serviceKey is how a service is named in the assignments: « Cloudflare · example.com » → « cloudflare · example.com ».
func serviceKey(name string) string { return strings.Join(strings.Fields(strings.ToLower(name)), " ") }

// projectOf files an expense: the invoice's own choice, then its service's,
// then the first project whose name or keyword appears in its service, its
// match or its label.
func projectOf(db DB, s Spend) (id, by string) {
	known := func(p string) bool {
		for _, pr := range db.Projects {
			if pr.ID == p {
				return true
			}
		}
		return p == unfiled
	}
	choices := []struct{ key, by string }{{"svc:" + serviceKey(s.Service), "service"}}
	if s.InvoiceID != "" {
		choices = append([]struct{ key, by string }{{"inv:" + s.InvoiceID, "invoice"}}, choices...)
	}
	for _, k := range choices {
		if p := db.Assign[k.key]; p != "" && known(p) {
			if p == unfiled {
				return "", k.by
			}
			return p, k.by
		}
	}
	hay := words(s.Service + " " + s.match + " " + s.Label)
	for _, pr := range db.Projects {
		for _, kw := range append([]string{pr.Name}, pr.Keywords...) {
			if w := words(kw); strings.TrimSpace(w) != "" && strings.Contains(hay, w) {
				return pr.ID, "keyword"
			}
		}
	}
	return "", ""
}

// spending lists every amount from three years back to one year ahead, in
// euros: the invoices, the past due dates with no invoice at their estimate,
// and the due dates to come. Skipped due dates and zero amounts are left out.
func (m *model) spending(rates Rates) analyticsResponse {
	res := analyticsResponse{Today: m.today.Format(dayFmt), Projects: m.db.Projects, Spend: []Spend{}, RateDate: rates.Date}
	if usd := rates.PerEUR["USD"]; usd > 0 {
		res.USDToEUR = math.Round(10000/usd) / 10000
	}
	add := func(s Spend) {
		if s.Amount == 0 {
			return
		}
		eur, ok := rates.toEUR(s.Amount, s.Currency)
		if !ok {
			res.NoRate++
			return
		}
		s.EUR = round2(eur)
		s.Project, s.ProjectBy = projectOf(m.db, s)
		res.Spend = append(res.Spend, s)
	}
	claimed := map[string]bool{}
	for _, st := range m.dues {
		claimed[st.InvoiceID] = true
	}
	for _, p := range m.db.Payments {
		est := m.estimate(p)
		for _, d := range p.dues(m.today.AddDate(-3, 0, 0), m.today.AddDate(1, 0, 0)) {
			s := Spend{
				Date: d.Format(dayFmt), Service: p.Name, Domain: p.Domain, Cycle: p.Cycle, PaymentID: p.ID,
				Amount: est.Amount, Currency: est.Currency, Kind: "estimate", match: p.Match,
			}
			switch st := m.dueStatus(p, d); st.Status {
			case "skipped":
				continue
			case "upcoming", "today":
				s.Kind = "planned"
			case "matched":
				if inv := m.byID[st.InvoiceID]; inv != nil && inv.Amount != nil {
					s.Date, s.Label, s.InvoiceID = inv.Date, inv.Label, inv.ID
					s.Amount, s.Currency, s.Kind = *inv.Amount, inv.Currency, "invoice"
				}
			}
			add(s)
		}
	}
	for _, inv := range m.invoices {
		if claimed[inv.ID] {
			continue
		}
		if inv.Amount == nil {
			res.NoAmount++
			continue
		}
		name := inv.Vendor
		if name == "" {
			name = "Fournisseur inconnu"
		}
		add(Spend{
			Date: inv.Date, Service: name, Domain: inv.Domain, Label: inv.Label,
			Amount: *inv.Amount, Currency: inv.Currency, Kind: "invoice", InvoiceID: inv.ID,
		})
	}
	sort.SliceStable(res.Spend, func(i, j int) bool { return res.Spend[i].Date < res.Spend[j].Date })
	return res
}

func (a *App) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	m, err := a.buildModel()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, m.spending(a.rates()))
}

// handleSaveProject creates a project, or renames and recolours one.
func (a *App) handleSaveProject(w http.ResponseWriter, r *http.Request) {
	var p Project
	if !readJSON(w, r, &p) {
		return
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || p.Name == unfiled {
		fail(w, http.StatusBadRequest, errors.New("le projet a besoin d'un nom"))
		return
	}
	var kws []string
	for _, k := range p.Keywords {
		if k = strings.TrimSpace(k); k != "" {
			kws = append(kws, k)
		}
	}
	p.Keywords = kws
	err := a.store.update(func(db *DB) error {
		used := map[string]bool{}
		for _, other := range db.Projects {
			if other.ID != p.ID && strings.EqualFold(other.Name, p.Name) {
				return errors.New("un projet porte déjà ce nom")
			}
			used[other.Color] = true
		}
		if !validColor(p.Color) {
			p.Color = projectColors[len(db.Projects)%len(projectColors)]
			for _, c := range projectColors {
				if !used[c] {
					p.Color = c
					break
				}
			}
		}
		if p.ID == "" {
			p.ID = newID()
			db.Projects = append(db.Projects, p)
			return nil
		}
		for i := range db.Projects {
			if db.Projects[i].ID == p.ID {
				db.Projects[i] = p
				return nil
			}
		}
		return errors.New("projet introuvable")
	})
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, p)
}

func validColor(c string) bool {
	for _, x := range projectColors {
		if x == c {
			return true
		}
	}
	return false
}

// handleDeleteProject removes a project and every choice that filed something in it.
func (a *App) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := a.store.update(func(db *DB) error {
		kept := db.Projects[:0]
		for _, p := range db.Projects {
			if p.ID != id {
				kept = append(kept, p)
			}
		}
		db.Projects = kept
		for k, v := range db.Assign {
			if v == id {
				delete(db.Assign, k)
			}
		}
		return nil
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// handleAssign files a service or one invoice in a project; « - » keeps it out
// of every project, an empty project goes back to the automatic choice.
func (a *App) handleAssign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind    string `json:"kind"` // service | invoice
		Ref     string `json:"ref"`
		Project string `json:"project"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	key := ""
	switch req.Kind {
	case "service":
		key = "svc:" + serviceKey(req.Ref)
	case "invoice":
		key = "inv:" + strings.TrimSpace(req.Ref)
	}
	if strings.HasSuffix(key, ":") || key == "" {
		fail(w, http.StatusBadRequest, errors.New("service ou facture manquant"))
		return
	}
	err := a.store.update(func(db *DB) error {
		if req.Project == "" {
			delete(db.Assign, key)
			return nil
		}
		if req.Project != unfiled && !hasProject(db.Projects, req.Project) {
			return errors.New("projet introuvable")
		}
		db.Assign[key] = req.Project
		return nil
	})
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func hasProject(list []Project, id string) bool {
	for _, p := range list {
		if p.ID == id {
			return true
		}
	}
	return false
}
