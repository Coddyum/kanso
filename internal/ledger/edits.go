package ledger

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/coddyum/kanso/internal/currency"
	"github.com/coddyum/kanso/internal/day"
	"github.com/coddyum/kanso/internal/deadlines"
	"github.com/coddyum/kanso/internal/i18n"
	"github.com/coddyum/kanso/internal/vendor"
)

// Invalid is a refusal the user can fix: a missing name, an unknown project…
// It holds the key of its sentence in web/locales (error.*), which the server
// writes in the user's language. Any other error of an edit is the disk's.
type Invalid string

func (e Invalid) Error() string { return string(e) }

// NewID is a random id of 12 hex characters.
func NewID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

var cycles = map[string]bool{"once": true, "weekly": true, "monthly": true, "quarterly": true, "yearly": true}

// SavePayment creates a payment (no id) or replaces one. A typed amount
// (amountChanged) wins over the invoices until a newer one arrives; a renamed
// payment keeps its project. Name, cycle, start date and a positive amount are required.
func (b *Book) SavePayment(p Payment, amountChanged bool) (Payment, error) {
	p.Name, p.Vendor, p.Note = strings.TrimSpace(p.Name), strings.TrimSpace(p.Vendor), strings.TrimSpace(p.Note)
	p.Match = strings.TrimSpace(p.Match)
	p.Billing = vendor.CleanURL(p.Billing)
	p.Domain = vendor.CleanDomain(p.Domain)
	if p.Domain == "" {
		p.Domain = b.vendors.Domain(p.InvoiceName())
	}
	if p.Currency == "" {
		p.Currency = "EUR"
	}
	if !currency.Known(p.Currency) {
		return p, Invalid("error.unknownCurrency")
	}
	if _, ok := day.Parse(p.Start); !ok || p.Name == "" || !cycles[p.Cycle] || p.Amount < 0 {
		return p, Invalid("error.paymentFields")
	}
	if _, ok := day.Parse(p.End); p.End != "" && !ok {
		p.End = ""
	}
	if amountChanged {
		p.AmountSet = time.Now().Format(time.RFC3339)
	}
	err := b.store.Update(func(db *DB) error {
		if p.ID == "" {
			p.ID = NewID()
			db.Payments = append(db.Payments, p)
			return nil
		}
		for i := range db.Payments {
			if db.Payments[i].ID == p.ID {
				if !amountChanged {
					p.AmountSet = db.Payments[i].AmountSet
				}
				moveServiceProject(db, db.Payments[i].Name, p.Name, p.ID)
				db.Payments[i] = p
				return nil
			}
		}
		return Invalid("error.paymentNotFound")
	})
	return p, err
}

// moveServiceProject keeps the project of a renamed payment, unless another
// payment still carries the old name or the new name already has one.
func moveServiceProject(db *DB, oldName, newName, id string) {
	from, to := "svc:"+ServiceKey(oldName), "svc:"+ServiceKey(newName)
	if from == to || db.Assign[from] == "" || db.Assign[to] != "" {
		return
	}
	for _, other := range db.Payments {
		if other.ID != id && ServiceKey(other.Name) == ServiceKey(oldName) {
			return
		}
	}
	db.Assign[to] = db.Assign[from]
	delete(db.Assign, from)
}

// DeletePayment removes a payment. Its subscription is dismissed, so the next
// look at the folder does not add it back.
func (b *Book) DeletePayment(id string) error {
	return b.store.Update(func(db *DB) error {
		kept := db.Payments[:0]
		for _, p := range db.Payments {
			if p.ID != id {
				kept = append(kept, p)
				continue
			}
			db.Dismissed[suggestKey(p.InvoiceName(), p.Match)] = true
			if p.SuggestKey != "" {
				db.Dismissed[p.SuggestKey] = true
			}
		}
		db.Payments = kept
		return nil
	})
}

// SaveTodo creates a reminder (unknown id) or changes its text and state.
func (b *Book) SaveTodo(t Todo) (Todo, error) {
	t.Text = strings.TrimSpace(t.Text)
	if t.Text == "" {
		return t, Invalid("error.emptyText")
	}
	err := b.store.Update(func(db *DB) error {
		for i := range db.Todos {
			if db.Todos[i].ID == t.ID {
				db.Todos[i].Text, db.Todos[i].Done = t.Text, t.Done
				return nil
			}
		}
		t.ID, t.Created = NewID(), time.Now().Format(day.Format)
		db.Todos = append(db.Todos, t)
		return nil
	})
	return t, err
}

// DeleteTodo removes a reminder.
func (b *Book) DeleteTodo(id string) error {
	return b.store.Update(func(db *DB) error {
		kept := db.Todos[:0]
		for _, t := range db.Todos {
			if t.ID != id {
				kept = append(kept, t)
			}
		}
		db.Todos = kept
		return nil
	})
}

// Dismiss marks a due date as « no invoice », or hides a suggestion; off undoes it.
func (b *Book) Dismiss(key string, on bool) error {
	if key == "" {
		return Invalid("error.missingKey")
	}
	return b.store.Update(func(db *DB) error {
		if on {
			db.Dismissed[key] = true
		} else {
			delete(db.Dismissed, key)
		}
		return nil
	})
}

// SetOverride saves the corrections of a file (by hash); nil resets it to what was read.
func (b *Book) SetOverride(hash string, o *Override) error {
	if hash == "" {
		return Invalid("error.missingFile")
	}
	if o != nil {
		if _, ok := day.Parse(o.Date); o.Date != "" && !ok {
			o.Date = ""
		}
		o.Vendor, o.Label = strings.TrimSpace(o.Vendor), strings.TrimSpace(o.Label)
		if o.Currency != "" && !currency.Known(o.Currency) {
			return Invalid("error.unknownCurrency")
		}
	}
	return b.store.Update(func(db *DB) error {
		if o == nil {
			delete(db.Overrides, hash)
		} else {
			db.Overrides[hash] = *o
		}
		return nil
	})
}

// SetReserve saves the balance of the account on date (today when it is not
// a day); a nil amount removes it.
func (b *Book) SetReserve(amount *float64, date string) error {
	if _, ok := day.Parse(date); !ok {
		date = time.Now().Format(day.Format)
	}
	return b.store.Update(func(db *DB) error {
		if amount == nil {
			db.Reserve = nil
		} else {
			db.Reserve = &Reserve{Amount: *amount, Date: date}
		}
		return nil
	})
}

// Link remembers the file (by hash) dropped for a due date: it is that due date's invoice.
func (b *Book) Link(dueKey, hash string) error {
	return b.store.Update(func(db *DB) error { db.Links[dueKey] = hash; return nil })
}

// SaveProject creates a project (no id) or renames and recolours one. Names
// are unique; a project with no valid colour gets the first one unused.
func (b *Book) SaveProject(p Project) (Project, error) {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || p.Name == Unfiled {
		return p, Invalid("error.projectName")
	}
	var kws []string
	for _, k := range p.Keywords {
		if k = strings.TrimSpace(k); k != "" {
			kws = append(kws, k)
		}
	}
	p.Keywords = kws
	err := b.store.Update(func(db *DB) error {
		used := map[string]bool{}
		for _, other := range db.Projects {
			if other.ID != p.ID && strings.EqualFold(other.Name, p.Name) {
				return Invalid("error.projectExists")
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
			p.ID = NewID()
			db.Projects = append(db.Projects, p)
			return nil
		}
		for i := range db.Projects {
			if db.Projects[i].ID == p.ID {
				db.Projects[i] = p
				return nil
			}
		}
		return Invalid("error.projectNotFound")
	})
	return p, err
}

func validColor(c string) bool {
	for _, x := range projectColors {
		if x == c {
			return true
		}
	}
	return false
}

// DeleteProject removes a project and every choice that filed something in it.
func (b *Book) DeleteProject(id string) error {
	return b.store.Update(func(db *DB) error {
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
}

// Assign files a service (kind « service ») or one invoice (« invoice ») in a
// project; Unfiled keeps it out of every project, "" goes back to the automatic choice.
func (b *Book) Assign(kind, ref, project string) error {
	key := ""
	switch kind {
	case "service":
		key = "svc:" + ServiceKey(ref)
	case "invoice":
		key = "inv:" + strings.TrimSpace(ref)
	}
	if key == "" || strings.HasSuffix(key, ":") {
		return Invalid("error.missingRef")
	}
	return b.store.Update(func(db *DB) error {
		if project == "" {
			delete(db.Assign, key)
			return nil
		}
		if project != Unfiled && !hasProject(db.Projects, project) {
			return Invalid("error.projectNotFound")
		}
		db.Assign[key] = project
		return nil
	})
}

// SetDone ticks an administrative due date as done, or unticks it.
func (b *Book) SetDone(key string, done bool) error {
	if key == "" {
		return Invalid("error.missingDeadline")
	}
	return b.store.Update(func(db *DB) error {
		if done {
			db.Done[key] = true
		} else {
			delete(db.Done, key)
		}
		return nil
	})
}

// SetSettings saves which administrative due dates apply.
func (b *Book) SetSettings(s deadlines.Settings) error {
	if !s.Valid() {
		return Invalid("error.unknownRhythm")
	}
	return b.store.Update(func(db *DB) error {
		db.Admin = &s
		return nil
	})
}

// Lang is the language chosen in the app, "" when it follows the Mac.
func (b *Book) Lang() string { return b.store.Snapshot().Lang }

// SetLang chooses the app's language; "" follows the Mac again.
func (b *Book) SetLang(code string) error {
	if code != "" && !i18n.Known(code) {
		return Invalid("error.unknownLang")
	}
	return b.store.Update(func(db *DB) error {
		db.Lang = code
		return nil
	})
}
