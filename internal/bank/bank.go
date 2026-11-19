// Package bank reads a bank statement exported by the user (CSV or OFX) and
// what follows from it: the balance of the account and the debits that come
// back, the subscriptions. It works on the bytes it is given and keeps
// nothing: the file is never written anywhere.
package bank

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sort"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

// ErrUnreadable: the file is neither a CSV nor an OFX this package can read.
var ErrUnreadable = errors.New("bank: unreadable statement")

// ErrMapping: the columns chosen do not give a date, a label and an amount on most rows.
var ErrMapping = errors.New("bank: the columns chosen do not read")

// Transaction is one line of the statement.
type Transaction struct {
	Date     time.Time
	Label    string
	Amount   float64  // negative for a debit
	Currency string   // "" when the statement does not say
	Balance  *float64 // the balance after it, when the statement writes one
	order    int      // its place in the file
}

// Balance is the account's balance on a day.
type Balance struct {
	Amount   float64 `json:"amount"`
	Date     string  `json:"date"`
	Currency string  `json:"currency"`
}

// Decode turns a file's bytes into text: UTF-8 (with or without its BOM),
// UTF-16 with a BOM, or Windows-1252, which many banks still export.
func Decode(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		return string(data[3:])
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		return utf16Text(data[2:], binary.LittleEndian)
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return utf16Text(data[2:], binary.BigEndian)
	case utf8.Valid(data):
		return string(data)
	}
	return windows1252(data)
}

func utf16Text(data []byte, order binary.ByteOrder) string {
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = order.Uint16(data[2*i:])
	}
	return string(utf16.Decode(units))
}

// cp1252 holds the characters of Windows-1252 from 0x80 to 0x9F; the rest is Latin-1.
var cp1252 = [32]rune{
	0x20AC, 0x81, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, 0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x8D, 0x017D, 0x8F,
	0x90, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, 0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x9D, 0x017E, 0x0178,
}

func windows1252(data []byte) string {
	out := make([]rune, len(data))
	for i, b := range data {
		if b >= 0x80 && b < 0xA0 {
			out[i] = cp1252[b-0x80]
		} else {
			out[i] = rune(b)
		}
	}
	return string(out)
}

// Read reads a statement: OFX when it looks like one, CSV otherwise. A CSV
// whose columns could not be told apart comes back with Table set and no
// transactions: the user says which column is which, then Table.Read.
func Read(data []byte) (Statement, error) {
	text := Decode(data)
	if looksOFX(text) {
		return readOFX(text)
	}
	table, err := readTable(text)
	if err != nil {
		return Statement{}, err
	}
	st := Statement{Format: "csv", Table: &table}
	m, ok := table.Guess()
	st.Mapping = m
	if !ok {
		return st, nil
	}
	if st.Transactions, err = table.Read(m); err != nil {
		return st, nil
	}
	st.finish()
	return st, nil
}

// Statement is what a file holds.
type Statement struct {
	Format       string // csv | ofx
	Table        *Table // a CSV: its cells, to choose the columns
	Mapping      Mapping
	Transactions []Transaction
	Balance      *Balance // the latest balance the file writes
	Currency     string   // the account's currency, when the file says
}

// finish sorts the transactions by date and reads the latest balance a column wrote.
func (st *Statement) finish() {
	txs := st.Transactions
	if len(txs) == 0 {
		return
	}
	// A file is written newest first or oldest first: on the same day, its order says which came last.
	newestFirst := txs[0].Date.After(txs[len(txs)-1].Date)
	sort.SliceStable(txs, func(i, j int) bool {
		if !txs[i].Date.Equal(txs[j].Date) {
			return txs[i].Date.Before(txs[j].Date)
		}
		if newestFirst {
			return txs[i].order > txs[j].order
		}
		return txs[i].order < txs[j].order
	})
	if st.Balance != nil {
		return
	}
	for i := len(txs) - 1; i >= 0; i-- {
		if txs[i].Balance != nil {
			cur := txs[i].Currency
			if cur == "" {
				cur = st.Currency
			}
			st.Balance = &Balance{Amount: *txs[i].Balance, Date: txs[i].Date.Format("2006-01-02"), Currency: cur}
			return
		}
	}
}

// Remap reads a CSV again with the columns the user chose; ErrMapping when they do not read.
func (st Statement) Remap(m Mapping) (Statement, error) {
	if st.Table == nil {
		return st, ErrMapping
	}
	txs, err := st.Table.Read(m)
	if err != nil {
		return st, err
	}
	st.Mapping, st.Transactions, st.Balance = m, txs, nil
	st.finish()
	return st, nil
}

// Span is the first and last day of the transactions, "" when there are none.
func (st Statement) Span() (from, to string) {
	if len(st.Transactions) == 0 {
		return "", ""
	}
	return st.Transactions[0].Date.Format("2006-01-02"), st.Transactions[len(st.Transactions)-1].Date.Format("2006-01-02")
}
