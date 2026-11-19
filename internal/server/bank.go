package server

import (
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/coddyum/kanso/internal/bank"
	"github.com/coddyum/kanso/internal/currency"
	"github.com/coddyum/kanso/internal/ledger"
)

// maxStatement is the largest bank statement read: years of lines weigh a few megabytes.
const maxStatement = 20 << 20

// bankPreview is what the import screen shows before anything is written.
type bankPreview struct {
	Format        string              `json:"format"`
	Columns       []string            `json:"columns"` // a CSV's header; empty when it has none
	Width         int                 `json:"width"`   // a CSV's number of columns
	Sample        [][]string          `json:"sample"`  // its first rows
	Mapping       bank.Mapping        `json:"mapping"`
	NeedsMapping  bool                `json:"needsMapping"`
	Count         int                 `json:"count"`
	From          string              `json:"from"`
	To            string              `json:"to"`
	Balance       *balanceView        `json:"balance"`
	Subscriptions []bank.Subscription `json:"subscriptions"`
}

// balanceView is the statement's latest balance, and what it makes in euros for the reserve.
type balanceView struct {
	bank.Balance
	EUR *float64 `json:"eur"` // nil when its currency has no known rate
}

// handleBankPreview reads a bank statement, the request's body (CSV or OFX),
// in memory only: it is never written anywhere. ?map=date,label,amount,debit,
// credit,balance,currency gives the columns of a CSV the user chose. It answers
// the balance and the subscriptions read, or the columns to choose.
func (s *Server) handleBankPreview(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxStatement))
	if err != nil {
		http.Error(w, s.tr(r).T("error.bankTooBig"), http.StatusBadRequest)
		return
	}
	st, err := bank.Read(data)
	if err != nil {
		http.Error(w, s.tr(r).T("error.bankUnreadable"), http.StatusBadRequest)
		return
	}
	if raw := r.URL.Query().Get("map"); raw != "" && st.Table != nil {
		m, ok := parseMapping(raw)
		if !ok {
			http.Error(w, s.tr(r).T("error.bankMapping"), http.StatusBadRequest)
			return
		}
		if st, err = st.Remap(m); err != nil {
			http.Error(w, s.tr(r).T("error.bankMapping"), http.StatusBadRequest)
			return
		}
	}
	writeJSON(w, s.preview(st))
}

func (s *Server) preview(st bank.Statement) bankPreview {
	p := bankPreview{Format: st.Format, Columns: []string{}, Sample: [][]string{}, Mapping: st.Mapping, Count: len(st.Transactions)}
	if st.Table != nil {
		p.Width, p.Sample = st.Table.Width, st.Table.Sample(6)
		if st.Table.Columns != nil {
			p.Columns = st.Table.Columns
		}
	}
	p.NeedsMapping = len(st.Transactions) == 0
	p.From, p.To = st.Span()
	if st.Balance != nil {
		bv := &balanceView{Balance: *st.Balance}
		if bv.Currency == "" {
			bv.Currency = "EUR"
		}
		if eur, ok := s.currentRates().ToEUR(bv.Amount, bv.Currency); ok {
			eur = math.Round(eur*100) / 100
			bv.EUR = &eur
		}
		p.Balance = bv
	}
	p.Subscriptions = bank.Recurring(st.Transactions, st.Currency, s.vendors)
	payments := s.book.Snapshot().Payments
	for i := range p.Subscriptions {
		sub := &p.Subscriptions[i]
		sub.Known = ledger.Tracked(payments, sub.Name)
		if !currency.Known(sub.Currency) {
			sub.Currency = "EUR"
		}
	}
	if p.Subscriptions == nil {
		p.Subscriptions = []bank.Subscription{}
	}
	return p
}

// parseMapping reads « 3,4,5,-1,-1,9,7 »: the column of the date, the label,
// the amount, the debit, the credit, the balance and the currency.
func parseMapping(raw string) (bank.Mapping, bool) {
	parts := strings.Split(raw, ",")
	if len(parts) != 7 {
		return bank.NoMapping, false
	}
	n := make([]int, 7)
	for i, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || v < -1 || v > 200 {
			return bank.NoMapping, false
		}
		n[i] = v
	}
	return bank.Mapping{Date: n[0], Label: n[1], Amount: n[2], Debit: n[3], Credit: n[4], Balance: n[5], Currency: n[6]}, true
}

// handleBankApply adds the subscriptions the user ticked as payments and,
// when asked, sets the reserve; the import step of the checklist is then done.
func (s *Server) handleBankApply(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Subscriptions []bank.Subscription `json:"subscriptions"`
		Reserve       *struct {
			Amount float64 `json:"amount"`
			Date   string  `json:"date"`
		} `json:"reserve"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	payments := make([]ledger.Payment, 0, len(req.Subscriptions))
	for _, sub := range req.Subscriptions {
		payments = append(payments, ledger.Payment{
			Name: sub.Name, Domain: sub.Domain, Amount: sub.Amount, Currency: sub.Currency,
			Cycle: sub.Cycle, Start: sub.Start, ExpectInvoice: true, SuggestKey: sub.Key,
		})
	}
	added, err := s.book.AddPayments(payments)
	if err != nil {
		s.failEdit(w, r, err)
		return
	}
	if req.Reserve != nil {
		amount := req.Reserve.Amount
		if err := s.book.SetReserve(&amount, req.Reserve.Date); err != nil {
			s.failEdit(w, r, err)
			return
		}
		_ = s.book.SetOnboardingStep("reserve", "done")
	}
	if err := s.book.SetOnboardingStep("bank", "done"); err != nil {
		s.failEdit(w, r, err)
		return
	}
	writeJSON(w, map[string]int{"added": added})
}
