package bank

import (
	"encoding/csv"
	"strings"

	"github.com/coddyum/kanso/internal/norm"
)

// Table is a CSV cut into cells: its header (when it has one) and its rows.
type Table struct {
	Columns []string   `json:"columns"` // the header's cells; empty when the file has none
	Rows    [][]string `json:"-"`
	Width   int        `json:"width"` // the number of columns of the data rows
}

// Sample is the first rows, for the screen where the user picks the columns.
func (t Table) Sample(n int) [][]string {
	return t.Rows[:min(n, len(t.Rows))]
}

// Mapping says which column holds what; -1 for none. Amount is one signed
// column, or Debit and Credit two columns of positive amounts.
type Mapping struct {
	Date     int `json:"date"`
	Label    int `json:"label"`
	Amount   int `json:"amount"`
	Debit    int `json:"debit"`
	Credit   int `json:"credit"`
	Balance  int `json:"balance"`
	Currency int `json:"currency"`
}

// NoMapping is a mapping with no column.
var NoMapping = Mapping{-1, -1, -1, -1, -1, -1, -1}

// separators are the cell separators tried, the French one first: it wins a tie.
var separators = []rune{';', ',', '\t', '|'}

// readTable cuts text into cells: the separator is the one that gives the
// most lines of the same width; the header is the last line before the data
// that has no date in it. Lines before it (an account's name…) are dropped.
func readTable(text string) (Table, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var best [][]string
	bestWidth, bestScore := 0, 0
	for _, sep := range separators {
		rows := splitLines(lines, sep)
		width, score := modeWidth(rows)
		if width >= 2 && score > bestScore {
			best, bestWidth, bestScore = rows, width, score
		}
	}
	if bestScore < 2 {
		return Table{}, ErrUnreadable
	}
	var rows [][]string
	for _, r := range best {
		if len(r) >= bestWidth-1 && len(r) >= 2 {
			rows = append(rows, pad(r, bestWidth))
		}
	}
	first := -1 // the first row holding a date
	for i, r := range rows {
		if hasDate(r) {
			first = i
			break
		}
	}
	if first < 0 {
		return Table{}, ErrUnreadable
	}
	t := Table{Width: bestWidth}
	if first > 0 && !hasNumber(rows[first-1]) {
		t.Columns = rows[first-1]
	}
	for _, r := range rows[first:] {
		if hasDate(r) {
			t.Rows = append(t.Rows, r)
		}
	}
	return t, nil
}

// splitLines reads each line on its own: a broken quote spoils one line, not the rest.
func splitLines(lines []string, sep rune) [][]string {
	var rows [][]string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		r := csv.NewReader(strings.NewReader(line))
		r.Comma, r.LazyQuotes, r.FieldsPerRecord, r.TrimLeadingSpace = sep, true, -1, true
		cells, err := r.Read()
		if err != nil {
			continue
		}
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		rows = append(rows, cells)
	}
	return rows
}

// modeWidth is the most common number of cells per line (2 or more), and how many lines have it.
func modeWidth(rows [][]string) (int, int) {
	count := map[int]int{}
	for _, r := range rows {
		if len(r) >= 2 {
			count[len(r)]++
		}
	}
	width, score := 0, 0
	for w, n := range count {
		if n > score || (n == score && w > width) {
			width, score = w, n
		}
	}
	return width, score
}

// pad gives a row width cells: a trailing separator often drops the last, empty one.
func pad(r []string, width int) []string {
	for len(r) < width {
		r = append(r, "")
	}
	return r[:width]
}

func hasDate(r []string) bool {
	for _, c := range r {
		if _, ok := splitDate(c); ok {
			return true
		}
	}
	return false
}

func hasNumber(r []string) bool {
	for _, c := range r {
		if looksAmount(c) {
			return true
		}
	}
	return false
}

// Header words, compared without accents or spaces (« Date de l'opération » → « datedeloperation »).
var (
	dateWords     = []string{"date", "fecha", "datum", "data", "posted", "booking"}
	dateFirst     = []string{"operation", "completed", "transaction", "comptable", "booking", "posted"}
	dateLater     = []string{"valeur", "value", "valuta", "valor"}
	labelHeads    = []string{"libelle", "label", "description", "concepto", "payee", "beneficiaire", "merchant", "intitule", "wording", "narrative", "counterparty", "contrepartie"}
	labelLesser   = []string{"detail", "memo", "reference", "name", "nom", "objet"}
	amountWords   = []string{"montant", "amount", "importe", "betrag", "somme", "total", "valeur"}
	amountNot     = []string{"fee", "frais", "commission", "tax", "tva", "vat", "original", "date"}
	debitWords    = []string{"debit", "withdrawal", "sortie", "depense", "cargo", "moneyout", "paidout"}
	creditWords   = []string{"credit", "deposit", "entree", "recette", "abono", "moneyin", "paidin"}
	balanceWords  = []string{"solde", "balance", "saldo", "kontostand"}
	currencyWords = []string{"devise", "currency", "moneda", "wahrung", "ccy"}
)

func hasWord(header string, words []string) bool {
	k := norm.Key(header)
	for _, w := range words {
		if strings.Contains(k, w) {
			return true
		}
	}
	return false
}

// Guess reads the header, then the cells, to tell the columns apart. ok is
// false when a date, a label or an amount is missing, or when the mapping
// does not read most rows: the user then chooses.
func (t Table) Guess() (Mapping, bool) {
	m := NoMapping
	score := func(col int) (date, amount, text float64) { return t.share(col) }
	bestDate, bestLabel := -100, -100
	for col := 0; col < t.Width; col++ {
		h := ""
		if col < len(t.Columns) {
			h = t.Columns[col]
		}
		dates, amounts, texts := score(col)
		switch {
		case dates >= 0.8:
			s := 0
			if hasWord(h, dateWords) {
				s += 2
			}
			if hasWord(h, dateFirst) {
				s++
			}
			if hasWord(h, dateLater) {
				s -= 2
			}
			if s > bestDate {
				m.Date, bestDate = col, s
			}
		case hasWord(h, currencyWords):
			m.Currency = col
		case amounts >= 0.6 && hasWord(h, balanceWords):
			m.Balance = col
		case amounts >= 0.3 && hasWord(h, debitWords) && !hasWord(h, creditWords):
			m.Debit = col
		case amounts >= 0.3 && hasWord(h, creditWords):
			m.Credit = col
		case amounts >= 0.6 && hasWord(h, amountWords) && !hasWord(h, amountNot):
			if m.Amount < 0 {
				m.Amount = col
			}
		case texts >= 0.5:
			s := int(t.avgLen(col)) / 8 // the longer texts are the labels
			if hasWord(h, labelHeads) {
				s += 10
			} else if hasWord(h, labelLesser) {
				s += 4
			}
			if s > bestLabel {
				m.Label, bestLabel = col, s
			}
		}
	}
	if m.Amount < 0 && m.Debit < 0 {
		m = t.guessCells(m)
	}
	if m.Amount < 0 && (m.Debit < 0 || m.Credit < 0) {
		m.Debit, m.Credit = -1, -1
	}
	if m.Date < 0 || m.Label < 0 || (m.Amount < 0 && m.Debit < 0) {
		return m, false
	}
	txs, err := t.Read(m)
	return m, err == nil && len(txs) > 0
}

// guessCells finds the amount when no header names it: among the full
// numeric columns not taken, the one with the most debits (a minus sign);
// when one other column has decimals too, it is the balance. A column of
// whole numbers with no sign is a reference, never an amount.
func (t Table) guessCells(m Mapping) Mapping {
	best, bestNeg := -1, 0
	var decimals []int
	for col := 0; col < t.Width; col++ {
		if _, amounts, _ := t.share(col); amounts < 0.9 || col == m.Date || col == m.Balance || col == m.Currency {
			continue
		}
		neg, dec := 0, 0
		for _, r := range t.Rows {
			if strings.ContainsAny(r[col], "-\u2212(") {
				neg++
			}
			if d := strings.TrimRightFunc(r[col], func(c rune) bool { return c < '0' || c > '9' }); strings.LastIndexAny(d, ".,") >= len(d)-3 && strings.LastIndexAny(d, ".,") >= 0 {
				dec++
			}
		}
		if neg > bestNeg {
			best, bestNeg = col, neg
		}
		if dec*2 >= len(t.Rows) {
			decimals = append(decimals, col)
		}
	}
	if best < 0 {
		return m
	}
	m.Amount = best
	if m.Balance < 0 {
		for _, col := range decimals {
			if col != best {
				m.Balance = col
				break
			}
		}
	}
	return m
}

// share is the part of a column's non-empty cells that read as a date, as an amount, as text.
func (t Table) share(col int) (dates, amounts, texts float64) {
	n := 0
	for _, r := range t.Rows {
		c := r[col]
		if c == "" {
			continue
		}
		n++
		if _, ok := splitDate(c); ok {
			dates++
		} else if looksAmount(c) {
			amounts++
		} else {
			texts++
		}
	}
	if n == 0 {
		return 0, 0, 0
	}
	filled := float64(n) / float64(len(t.Rows))
	return dates / float64(n) * filled, amounts / float64(n), texts / float64(n) * filled
}

func (t Table) avgLen(col int) float64 {
	total := 0
	for _, r := range t.Rows {
		total += len([]rune(r[col]))
	}
	return float64(total) / float64(max(1, len(t.Rows)))
}

// Read turns the rows into transactions with m. ErrMapping when a needed
// column is out of the table or fewer than 4 rows in 5 read.
func (t Table) Read(m Mapping) ([]Transaction, error) {
	in := func(c int) bool { return c >= 0 && c < t.Width }
	if !in(m.Date) || !in(m.Label) || (!in(m.Amount) && !in(m.Debit) && !in(m.Credit)) {
		return nil, ErrMapping
	}
	order := dateOrder(t.column(m.Date))
	decAmount := decimalOf(t.column(m.Amount), t.column(m.Debit), t.column(m.Credit))
	decBalance := decimalOf(t.column(m.Balance))
	var txs []Transaction
	for i, r := range t.Rows {
		d, ok := parseDate(r[m.Date], order)
		if !ok {
			continue
		}
		var amount float64
		switch {
		case in(m.Amount):
			if amount, ok = parseAmount(r[m.Amount], decAmount); !ok {
				continue
			}
		default:
			debit, okD := cell(r, m.Debit, decAmount)
			credit, okC := cell(r, m.Credit, decAmount)
			if !okD && !okC {
				continue
			}
			amount = abs(credit) - abs(debit)
		}
		tx := Transaction{Date: d, Label: strings.Join(strings.Fields(r[m.Label]), " "), Amount: amount, order: i}
		if in(m.Currency) {
			tx.Currency = strings.ToUpper(strings.TrimSpace(r[m.Currency]))
		}
		if b, ok := cell(r, m.Balance, decBalance); ok {
			tx.Balance = &b
		}
		txs = append(txs, tx)
	}
	if len(txs) == 0 || len(txs)*5 < len(t.Rows)*4 {
		return nil, ErrMapping
	}
	return txs, nil
}

// column is the cells of col, nil when there is no such column.
func (t Table) column(col int) []string {
	if col < 0 || col >= t.Width {
		return nil
	}
	out := make([]string, len(t.Rows))
	for i, r := range t.Rows {
		out[i] = r[col]
	}
	return out
}

func cell(r []string, col int, dec byte) (float64, bool) {
	if col < 0 || col >= len(r) || strings.TrimSpace(r[col]) == "" {
		return 0, false
	}
	return parseAmount(r[col], dec)
}

func abs(v float64) float64 { return max(v, -v) }
