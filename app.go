package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// App holds the synced folder, its index and the small JSON database.
type App struct {
	root     string
	cacheDir string
	index    *Index
	store    *Store
	boot     string // when this process started: the page reloads itself when it changes (a new version)

	mu       sync.Mutex
	cached   *model
	cacheKey string // folder signature | database version | day
}

func newApp(root, cacheDir, pdftext string) (*App, error) {
	data := filepath.Join(root, ".coddyum-inc")
	store, err := openStore(filepath.Join(data, "db.json"))
	if err != nil {
		return nil, err
	}
	if err := loadLocalVendors(filepath.Join(data, "vendors.json")); err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(root))
	cacheFile := filepath.Join(cacheDir, "index-"+hex.EncodeToString(sum[:4])+".json")
	ix := &Index{root: root, cacheFile: cacheFile, pdftext: pdftext}
	return &App{root: root, cacheDir: cacheDir, index: ix, store: store, boot: time.Now().Format(time.RFC3339Nano)}, nil
}

func (a *App) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/state", a.handleState)
	mux.HandleFunc("GET /api/calendar", a.handleCalendar)
	mux.HandleFunc("GET /api/sig", a.handleSig)
	mux.HandleFunc("GET /api/ls", a.handleList)
	mux.HandleFunc("GET /api/logo", a.handleLogo)
	mux.HandleFunc("GET /files/{path...}", a.handleFile)
	mux.HandleFunc("POST /api/mkdir", a.handleMkdir)
	mux.HandleFunc("POST /api/upload", a.handleUpload)
	mux.HandleFunc("POST /api/rename", a.handleRename)
	mux.HandleFunc("POST /api/move", a.handleMove)
	mux.HandleFunc("POST /api/reveal", a.handleReveal)
	mux.HandleFunc("POST /api/open", a.handleOpen)
	mux.HandleFunc("POST /api/payments", a.handleSavePayment)
	mux.HandleFunc("DELETE /api/payments/{id}", a.handleDeletePayment)
	mux.HandleFunc("POST /api/todos", a.handleSaveTodo)
	mux.HandleFunc("DELETE /api/todos/{id}", a.handleDeleteTodo)
	mux.HandleFunc("POST /api/dismiss", a.handleDismiss)
	mux.HandleFunc("POST /api/override", a.handleOverride)
	mux.HandleFunc("POST /api/reserve", a.handleReserve)
	mux.HandleFunc("GET /api/analytics", a.handleAnalytics)
	mux.HandleFunc("POST /api/projects", a.handleSaveProject)
	mux.HandleFunc("DELETE /api/projects/{id}", a.handleDeleteProject)
	mux.HandleFunc("POST /api/assign", a.handleAssign)
	mux.HandleFunc("POST /api/deadline", a.handleDeadline)
	mux.HandleFunc("POST /api/admin", a.handleAdmin)
	mux.HandleFunc("GET /api/reminders", a.handleReminders)
}

// model is the folder and the database, read together for one request.
type model struct {
	db          DB
	docs        []Doc
	invoices    []Invoice
	byID        map[string]*Invoice
	dues        map[string]dueState
	history     map[string][]Invoice // payment id → the invoices paired with its due dates that carry an amount, oldest first
	suggestions []Suggestion
	added       []string // names of the subscriptions added by this very request
	today       time.Time
}

// buildModel returns the model of the folder and the database, rebuilt only
// when one of them changed or the day turned: switching screens costs nothing.
func (a *App) buildModel() (*model, error) {
	sig, err := a.index.signature()
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	key := func() string {
		return fmt.Sprintf("%s|%d|%s", sig, a.store.Version(), dayOf(time.Now()).Format(dayFmt))
	}
	if a.cached != nil && a.cacheKey == key() {
		m := *a.cached
		m.added = nil
		return &m, nil
	}
	m, err := a.freshModel()
	if err != nil {
		return nil, err
	}
	a.cached, a.cacheKey = m, key()
	return m, nil
}

func (a *App) freshModel() (*model, error) {
	docs, err := a.index.scan()
	if err != nil {
		return nil, err
	}
	db := a.store.snapshot()
	edited := map[string]bool{}
	dropped := map[string]bool{}
	for _, hash := range db.Links {
		dropped[hash] = true
	}
	for i := range docs {
		// A file dropped for a due date is its invoice, whatever its title says.
		if dropped[docs[i].Hash] && docs[i].Info.Kind == "document" {
			docs[i].Info.Kind = "invoice"
		}
		if o, ok := db.Overrides[docs[i].Hash]; ok {
			docs[i].Info = o.apply(docs[i].Info)
			edited[docs[i].Hash] = true
		}
	}
	invoices := groupInvoices(docs, edited)
	today := dayOf(time.Now())
	m := &model{docs: docs, invoices: invoices, byID: map[string]*Invoice{}, today: today}
	for i := range invoices {
		m.byID[invoices[i].ID] = &invoices[i]
	}
	m.suggestions = suggest(invoices, db.Payments, db.Dismissed, today)
	if m.added, err = a.autoAdd(m.suggestions); err != nil {
		return nil, err
	}
	if len(m.added) > 0 {
		db = a.store.snapshot()
		m.suggestions = suggest(invoices, db.Payments, db.Dismissed, today)
	}
	for i := range invoices {
		if invoices[i].Domain != "" {
			continue
		}
		for _, p := range db.Payments {
			if p.Domain != "" && vendorMatches(p.invoiceName(), invoices[i].Vendor) {
				invoices[i].Domain = p.Domain
				break
			}
		}
	}
	sort.SliceStable(db.Payments, func(i, j int) bool { return db.Payments[i].Start < db.Payments[j].Start })
	m.db = db
	m.dues = matchDues(db.Payments, invoices, db.Links, today)
	m.history = paidHistory(m.dues, m.byID)
	return m, nil
}

// autoAdd turns every running subscription found in the invoices into a
// payment, unless one already stands for it or it was dismissed.
func (a *App) autoAdd(sugs []Suggestion) ([]string, error) {
	var todo []Suggestion
	for _, s := range sugs {
		if s.Active {
			todo = append(todo, s)
		}
	}
	if len(todo) == 0 {
		return nil, nil
	}
	var added []string
	err := a.store.update(func(db *DB) error {
		for _, s := range todo {
			if db.Dismissed[s.Key] || covered(db.Payments, s.Key, s.Vendor, s.Match) {
				continue
			}
			db.Payments = append(db.Payments, Payment{
				ID: newID(), Name: s.Name, Vendor: s.Vendor, Domain: s.Domain, Match: s.Match,
				Amount: s.Amount, Currency: s.Currency, Cycle: s.Cycle, Start: s.Start,
				ExpectInvoice: true, Auto: true, SuggestKey: s.Key,
			})
			added = append(added, s.Name)
		}
		return nil
	})
	return added, err
}

// paidHistory lists, per payment, the invoices paired with its due dates that
// carry an amount, oldest first.
func paidHistory(dues map[string]dueState, byID map[string]*Invoice) map[string][]Invoice {
	out := map[string][]Invoice{}
	for key, st := range dues {
		if inv := byID[st.InvoiceID]; inv != nil && inv.Amount != nil {
			id, _, _ := strings.Cut(key, "@")
			out[id] = append(out[id], *inv)
		}
	}
	for id := range out {
		sort.Slice(out[id], func(i, j int) bool { return out[id][i].Date < out[id][j].Date })
	}
	return out
}

// Estimate is what the next due dates of a payment should cost.
type Estimate struct {
	Amount   float64   // the reference: the latest invoice, or the amount typed after it
	Currency string    //
	Prudent  float64   // what the reserve counts: the highest of the last three invoices when it varies
	Variable bool      // its last three invoices differ by more than 10 %
	Recent   []float64 // those invoices' amounts, oldest first
	Manual   bool      // the typed amount is newer than the latest invoice
}

func (m *model) estimate(p Payment) Estimate {
	e := Estimate{Amount: p.Amount, Currency: p.Currency, Prudent: p.Amount}
	hist := m.history[p.ID]
	if len(hist) == 0 {
		return e
	}
	last := hist[len(hist)-1]
	if typed, err := time.Parse(time.RFC3339, p.AmountSet); err == nil && typed.After(last.Arrived) {
		e.Manual = true
		return e
	}
	e.Amount, e.Currency, e.Prudent = *last.Amount, last.Currency, *last.Amount
	lo, hi := *last.Amount, *last.Amount
	for _, inv := range hist[max(0, len(hist)-3):] {
		if inv.Currency != last.Currency {
			continue
		}
		e.Recent = append(e.Recent, *inv.Amount)
		lo, hi = min(lo, *inv.Amount), max(hi, *inv.Amount)
	}
	if len(e.Recent) >= 2 && hi > 0 && (hi-lo)/hi > 0.10 {
		e.Variable, e.Prudent = true, hi
	}
	return e
}

// DueItem is one due date of a payment, as the calendar and the lists show it.
type DueItem struct {
	Key       string   `json:"key"`
	Type      string   `json:"type"` // payment | invoice
	Date      string   `json:"date"`
	PaymentID string   `json:"paymentId,omitempty"`
	InvoiceID string   `json:"invoiceId,omitempty"`
	Name      string   `json:"name"`
	Vendor    string   `json:"vendor,omitempty"`
	Domain    string   `json:"domain,omitempty"`
	Amount    *float64 `json:"amount"`
	Currency  string   `json:"currency"`
	Status    string   `json:"status"`
	Estimate  bool     `json:"estimate,omitempty"` // the amount is a guess: a variable service with no invoice yet
	Detail    string   `json:"detail,omitempty"`   // a deadline: what to declare or pay
}

// dueItem shows a due date; once its invoice is found, at the invoice's amount.
func (m *model) dueItem(p Payment, d time.Time) DueItem {
	st := m.dueStatus(p, d)
	est := m.estimate(p)
	amount, currency, guess := est.Amount, est.Currency, est.Variable
	if inv := m.byID[st.InvoiceID]; inv != nil && inv.Amount != nil {
		amount, currency, guess = *inv.Amount, inv.Currency, false
	}
	return DueItem{
		Key: dueKey(p.ID, d), Type: "payment", Date: d.Format(dayFmt), PaymentID: p.ID, InvoiceID: st.InvoiceID,
		Name: p.Name, Vendor: p.invoiceName(), Domain: p.Domain, Amount: &amount, Currency: currency, Status: st.Status,
		Estimate: guess,
	}
}

// PaymentView is a payment with what it costs now.
type PaymentView struct {
	Payment
	Current         float64   `json:"current"`
	CurrentCurrency string    `json:"currentCurrency"`
	Variable        bool      `json:"variable"`
	Recent          []float64 `json:"recent,omitempty"`
	Manual          bool      `json:"manual"`
	LastInvoice     string    `json:"lastInvoice,omitempty"` // date of the latest invoice paired with one of its due dates
	BillingURL      string    `json:"billingUrl,omitempty"`  // where to fetch its invoices
	Project         string    `json:"project"`               // where its due dates are filed
	ProjectBy       string    `json:"projectBy"`             // service (chosen) | keyword (found in its name) | empty
}

type stateResponse struct {
	Root        string        `json:"root"`
	Today       string        `json:"today"`
	Extractor   bool          `json:"extractor"`
	Payments    []PaymentView `json:"payments"`
	Invoices    []Invoice     `json:"invoices"`
	Missing     []DueItem     `json:"missing"`
	Upcoming    []DueItem     `json:"upcoming"`
	Todos       []Todo        `json:"todos"`
	Suggestions []Suggestion  `json:"suggestions"`
	AutoAdded   []string      `json:"autoAdded"`
	Forecast    Forecast      `json:"forecast"`
	Projects    []Project     `json:"projects"`
	Deadlines   []Deadline    `json:"deadlines"` // from 100 days back to a year ahead
	Admin       Admin         `json:"admin"`
	Billing     []BillingLink `json:"billing"` // where to fetch invoices, once per address
}

func (a *App) handleState(w http.ResponseWriter, r *http.Request) {
	m, err := a.buildModel()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	res := stateResponse{
		Root: a.root, Today: m.today.Format(dayFmt), Extractor: a.index.pdftext != "",
		Payments: []PaymentView{}, Invoices: m.invoices, Todos: m.db.Todos,
		Missing: []DueItem{}, Upcoming: []DueItem{}, Suggestions: m.suggestions, AutoAdded: m.added,
		Forecast: m.forecast(a.rates()), Projects: m.db.Projects,
		Deadlines: m.deadlines(m.today.AddDate(0, 0, -100), m.today.AddDate(1, 0, 0)), Admin: m.db.admin(),
		Billing: m.billingLinks(),
	}
	if res.AutoAdded == nil {
		res.AutoAdded = []string{}
	}
	lastInvoice := map[string]string{}
	for key, st := range m.dues {
		if inv := m.byID[st.InvoiceID]; inv != nil {
			if id, _, _ := strings.Cut(key, "@"); inv.Date > lastInvoice[id] {
				lastInvoice[id] = inv.Date
			}
		}
	}
	for _, p := range m.db.Payments {
		est := m.estimate(p)
		project, by := projectOf(m.db, Spend{Service: p.Name, match: p.Match})
		res.Payments = append(res.Payments, PaymentView{
			Payment: p, Current: est.Amount, CurrentCurrency: est.Currency,
			Variable: est.Variable, Recent: est.Recent, Manual: est.Manual, LastInvoice: lastInvoice[p.ID],
			Project: project, ProjectBy: by, BillingURL: p.billingURL(),
		})
		for _, d := range p.dues(m.today.AddDate(-3, 0, 0), m.today) {
			if it := m.dueItem(p, d); it.Status == "missing" {
				res.Missing = append(res.Missing, it)
			}
		}
		for _, d := range p.dues(m.today, m.today.AddDate(0, 0, 45)) {
			res.Upcoming = append(res.Upcoming, m.dueItem(p, d))
		}
	}
	sort.SliceStable(res.Missing, func(i, j int) bool { return res.Missing[i].Date > res.Missing[j].Date })
	sort.SliceStable(res.Upcoming, func(i, j int) bool { return res.Upcoming[i].Date < res.Upcoming[j].Date })
	writeJSON(w, res)
}

// handleCalendar returns the due dates between from and to, plus the invoices
// of that range that no due date claims (spending that was not planned).
func (a *App) handleCalendar(w http.ResponseWriter, r *http.Request) {
	from, ok1 := parseDay(r.URL.Query().Get("from"))
	to, ok2 := parseDay(r.URL.Query().Get("to"))
	if !ok1 || !ok2 || to.Before(from) || to.Sub(from) > 400*24*time.Hour {
		fail(w, http.StatusBadRequest, fmt.Errorf("from/to invalides"))
		return
	}
	m, err := a.buildModel()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	events := []DueItem{}
	for _, p := range m.db.Payments {
		for _, d := range p.dues(from, to) {
			events = append(events, m.dueItem(p, d))
		}
	}
	for _, d := range m.deadlines(from, to) {
		events = append(events, DueItem{Key: d.Key, Type: "deadline", Date: d.Date, Name: d.Title, Detail: d.Detail, Status: d.Status})
	}
	claimed := map[string]bool{}
	for _, st := range m.dues {
		claimed[st.InvoiceID] = true
	}
	lo, hi := from.Format(dayFmt), to.Format(dayFmt)
	for _, inv := range m.invoices {
		if claimed[inv.ID] || inv.Date < lo || inv.Date > hi {
			continue
		}
		name := inv.Vendor
		if name == "" {
			name = inv.Files[0].Name
		}
		events = append(events, DueItem{
			Key: "inv:" + inv.ID, Type: "invoice", Date: inv.Date, InvoiceID: inv.ID, Name: name,
			Vendor: inv.Vendor, Domain: inv.Domain, Amount: inv.Amount, Currency: inv.Currency, Status: "invoice",
		})
	}
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Date != events[j].Date {
			return events[i].Date < events[j].Date
		}
		return events[i].Name < events[j].Name
	})
	writeJSON(w, events)
}

func (a *App) handleSig(w http.ResponseWriter, r *http.Request) {
	sig, err := a.index.signature()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]string{"sig": sig, "boot": a.boot})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, err error) {
	http.Error(w, err.Error(), code)
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		fail(w, http.StatusBadRequest, fmt.Errorf("JSON invalide : %w", err))
		return false
	}
	return true
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
