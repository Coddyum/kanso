package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var cycles = map[string]bool{"once": true, "weekly": true, "monthly": true, "quarterly": true, "yearly": true}

// cleanDomain turns « https://www.render.com/pricing » into « render.com ».
func cleanDomain(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	s = strings.TrimPrefix(s, "www.")
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	return s
}

func (a *App) handleSavePayment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Payment
		AmountChanged bool `json:"amountChanged"` // the owner typed a new amount: it wins until the next invoice
	}
	if !readJSON(w, r, &req) {
		return
	}
	p := req.Payment
	p.Name, p.Vendor, p.Note = strings.TrimSpace(p.Name), strings.TrimSpace(p.Vendor), strings.TrimSpace(p.Note)
	p.Match = strings.TrimSpace(p.Match)
	p.Billing = cleanURL(p.Billing)
	p.Domain = cleanDomain(p.Domain)
	if p.Domain == "" {
		p.Domain = vendorDomain(p.invoiceName())
	}
	if p.Currency == "" {
		p.Currency = "EUR"
	}
	if _, ok := parseDay(p.Start); !ok || p.Name == "" || !cycles[p.Cycle] || p.Amount < 0 {
		fail(w, http.StatusBadRequest, errors.New("nom, montant, fréquence et date sont requis"))
		return
	}
	if _, ok := parseDay(p.End); p.End != "" && !ok {
		p.End = ""
	}
	err := a.store.update(func(db *DB) error {
		if req.AmountChanged {
			p.AmountSet = time.Now().Format(time.RFC3339)
		}
		if p.ID == "" {
			p.ID = newID()
			db.Payments = append(db.Payments, p)
			return nil
		}
		for i := range db.Payments {
			if db.Payments[i].ID == p.ID {
				if !req.AmountChanged {
					p.AmountSet = db.Payments[i].AmountSet
				}
				moveServiceProject(db, db.Payments[i].Name, p.Name, p.ID)
				db.Payments[i] = p
				return nil
			}
		}
		return fmt.Errorf("paiement %s introuvable", p.ID)
	})
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, p)
}

func (a *App) handleDeletePayment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := a.store.update(func(db *DB) error {
		kept := db.Payments[:0]
		for _, p := range db.Payments {
			if p.ID != id {
				kept = append(kept, p)
				continue
			}
			// A deleted subscription is not added back by the next look at the folder.
			db.Dismissed[suggestKey(p.invoiceName(), p.Match)] = true
			if p.SuggestKey != "" {
				db.Dismissed[p.SuggestKey] = true
			}
		}
		db.Payments = kept
		return nil
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (a *App) handleSaveTodo(w http.ResponseWriter, r *http.Request) {
	var t Todo
	if !readJSON(w, r, &t) {
		return
	}
	t.Text = strings.TrimSpace(t.Text)
	if t.Text == "" {
		fail(w, http.StatusBadRequest, errors.New("texte vide"))
		return
	}
	err := a.store.update(func(db *DB) error {
		for i := range db.Todos {
			if db.Todos[i].ID == t.ID {
				db.Todos[i].Text, db.Todos[i].Done = t.Text, t.Done
				return nil
			}
		}
		t.ID, t.Created = newID(), time.Now().Format(dayFmt)
		db.Todos = append(db.Todos, t)
		return nil
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, t)
}

func (a *App) handleDeleteTodo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := a.store.update(func(db *DB) error {
		kept := db.Todos[:0]
		for _, t := range db.Todos {
			if t.ID != id {
				kept = append(kept, t)
			}
		}
		db.Todos = kept
		return nil
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// handleDismiss marks a due date as « no invoice », or hides a suggestion.
func (a *App) handleDismiss(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key string `json:"key"`
		On  bool   `json:"on"`
	}
	if !readJSON(w, r, &req) || req.Key == "" {
		return
	}
	err := a.store.update(func(db *DB) error {
		if req.On {
			db.Dismissed[req.Key] = true
		} else {
			delete(db.Dismissed, req.Key)
		}
		return nil
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// handleOverride saves the corrections of an invoice; a null override resets it.
func (a *App) handleOverride(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       string    `json:"id"`
		Override *Override `json:"override"`
	}
	if !readJSON(w, r, &req) || req.ID == "" {
		return
	}
	if o := req.Override; o != nil {
		if _, ok := parseDay(o.Date); o.Date != "" && !ok {
			o.Date = ""
		}
		o.Vendor, o.Label = strings.TrimSpace(o.Vendor), strings.TrimSpace(o.Label)
	}
	err := a.store.update(func(db *DB) error {
		if req.Override == nil {
			delete(db.Overrides, req.ID)
		} else {
			db.Overrides[req.ID] = *req.Override
		}
		return nil
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// handleReserve saves the balance of the account (today when no date is given);
// a null amount removes it.
func (a *App) handleReserve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Amount *float64 `json:"amount"`
		Date   string   `json:"date"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if _, ok := parseDay(req.Date); !ok {
		req.Date = time.Now().Format(dayFmt)
	}
	err := a.store.update(func(db *DB) error {
		if req.Amount == nil {
			db.Reserve = nil
		} else {
			db.Reserve = &Reserve{Amount: *req.Amount, Date: req.Date}
		}
		return nil
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// moveServiceProject keeps the project of a renamed payment, unless another
// payment still carries the old name or the new name already has one.
func moveServiceProject(db *DB, oldName, newName, id string) {
	from, to := "svc:"+serviceKey(oldName), "svc:"+serviceKey(newName)
	if from == to || db.Assign[from] == "" || db.Assign[to] != "" {
		return
	}
	for _, other := range db.Payments {
		if other.ID != id && serviceKey(other.Name) == serviceKey(oldName) {
			return
		}
	}
	db.Assign[to] = db.Assign[from]
	delete(db.Assign, from)
}
