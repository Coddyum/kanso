package main

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

// Extracted is what a document says about itself.
type Extracted struct {
	Kind     string   `json:"kind"` // invoice | receipt | order | statement | document
	Vendor   string   `json:"vendor,omitempty"`
	Domain   string   `json:"domain,omitempty"`
	Number   string   `json:"number,omitempty"`
	Ref      string   `json:"ref,omitempty"` // a receipt: the invoice number it pays
	Date     string   `json:"date,omitempty"`
	Amount   *float64 `json:"amount,omitempty"`
	Currency string   `json:"currency,omitempty"`
	Label    string   `json:"label,omitempty"`
	// PeriodDays is the length of the billed period when the document writes
	// one (« Oct 21–Nov 21, 2026 » → 31): a single invoice then tells its cycle.
	PeriodDays int `json:"periodDays,omitempty"`
}

type vendor struct {
	Name    string
	Domain  string
	Aliases []string
	InText  bool // may be recognised from the text, not only from the path
}

var knownVendors = []vendor{
	{"Anthropic", "anthropic.com", []string{"anthropic"}, true},
	{"Cloudflare", "cloudflare.com", []string{"cloudflare"}, true},
	{"Neon", "neon.tech", []string{"neon"}, true},
	{"OVHcloud", "ovhcloud.com", []string{"ovh", "ovhcloud"}, true},
	{"Render", "render.com", []string{"render"}, true},
	{"Stripe", "stripe.com", []string{"stripe"}, true},
	{"SeDomicilier", "sedomicilier.fr", []string{"sedomicilier", "hellodom"}, true},
	{"Bouygues Telecom", "bouyguestelecom.fr", []string{"bouygues", "bouyguestelecom"}, true},
	{"Free Mobile", "free.fr", []string{"free mobile"}, true},
	{"Free", "free.fr", []string{"free"}, false}, // a folder named « Free »; the word is too common in invoices to be read in their text
	{"Apple", "apple.com", []string{"apple"}, true},
	{"Urssaf", "urssaf.fr", []string{"urssaf", "ursaff"}, true},
	{"Insee", "insee.fr", []string{"insee"}, true},
	{"INPI", "inpi.fr", []string{"inpi"}, true},
	{"Indy", "indy.fr", []string{"indy"}, true},
	{"Impôts", "impots.gouv.fr", []string{"impot", "impots", "dgfip"}, true},
	{"Revolut", "revolut.com", []string{"revolut"}, true},
	{"GitHub", "github.com", []string{"github"}, true},
	{"Google", "google.com", []string{"google"}, true},
	{"OpenAI", "openai.com", []string{"openai"}, true},
	{"Vercel", "vercel.com", []string{"vercel"}, true},
	{"Resend", "resend.com", []string{"resend"}, true},
	{"Expo", "expo.dev", []string{"expo"}, true},
	{"Figma", "figma.com", []string{"figma"}, true},
	{"Notion", "notion.so", []string{"notion"}, true},
	{"Proton", "proton.me", []string{"proton"}, false}, // many customers have their own e-mail address on proton.me
}

var accentFolder = strings.NewReplacer(
	"é", "e", "è", "e", "ê", "e", "ë", "e", "à", "a", "â", "a", "ä", "a", "î", "i", "ï", "i",
	"ô", "o", "ö", "o", "û", "u", "ù", "u", "ü", "u", "ç", "c",
)

// words lowercases s, drops accents and turns every other character run into
// one space, padded with spaces: " free mobile mandat ".
func words(s string) string {
	s = accentFolder.Replace(strings.ToLower(s))
	var b strings.Builder
	b.WriteByte(' ')
	space := true
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			space = false
			continue
		}
		if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	if !space {
		b.WriteByte(' ')
	}
	return b.String()
}

// norm is the comparison key of a name: "OVH cloud" → "ovhcloud".
func norm(s string) string { return strings.ReplaceAll(strings.TrimSpace(words(s)), " ", "") }

func knownIn(s string) (vendor, bool) {
	w := words(s)
	for _, v := range knownVendors {
		for _, a := range v.Aliases {
			if strings.Contains(w, words(a)) {
				return v, true
			}
		}
	}
	return vendor{}, false
}

func vendorDomain(name string) string {
	if v, ok := knownIn(name); ok {
		return v.Domain
	}
	return ""
}

var monthWords = map[string]int{
	"janvier": 1, "january": 1, "jan": 1, "janv": 1,
	"fevrier": 2, "february": 2, "feb": 2, "fev": 2, "fevr": 2,
	"mars": 3, "march": 3, "mar": 3,
	"avril": 4, "april": 4, "apr": 4, "avr": 4,
	"mai": 5, "may": 5,
	"juin": 6, "june": 6, "jun": 6,
	"juillet": 7, "july": 7, "jul": 7, "juil": 7,
	"aout": 8, "august": 8, "aug": 8,
	"septembre": 9, "september": 9, "sep": 9, "sept": 9,
	"octobre": 10, "october": 10, "oct": 10,
	"novembre": 11, "november": 11, "nov": 11,
	"decembre": 12, "december": 12, "dec": 12,
}

var monthFolders = []string{"Janvier", "Fevrier", "Mars", "Avril", "Mai", "Juin", "Juillet", "Aout", "Septembre", "Octobre", "Novembre", "Decembre"}

func monthOf(word string) int { return monthWords[strings.TrimSpace(words(word))] }

var genericWords = map[string]bool{
	"projects": true, "projets": true, "project": true, "projet": true, "clients": true, "client": true,
	"contract": true, "contracts": true, "contrat": true, "contrats": true, "factures": true, "facture": true,
	"invoices": true, "invoice": true, "documents": true, "document": true, "docs": true, "divers": true,
	"perso": true, "pro": true, "justificatif": true, "justificatifs": true, "classer": true, "inbox": true,
	"recus": true, "receipts": true, "declaration": true, "mandat": true, "archives": true, "archive": true,
	"abonnements": true, "depenses": true,
}

// isTimeSegment says whether a folder name only holds a year, a month or a day.
func isTimeSegment(s string) bool {
	ws := strings.Fields(words(s))
	if len(ws) == 0 {
		return false
	}
	for _, w := range ws {
		if _, err := strconv.Atoi(w); err == nil {
			continue
		}
		if monthWords[w] == 0 {
			return false
		}
	}
	return true
}

func isGenericSegment(s string) bool {
	for _, w := range strings.Fields(words(s)) {
		if genericWords[w] {
			return true
		}
	}
	return false
}

// detectVendor: a known vendor in the path, then the first meaningful folder
// name (the owner's own sorting), then a known vendor named in the text.
func detectVendor(rel, text string) (string, string) {
	segs := strings.Split(rel, "/")
	for _, s := range segs {
		if v, ok := knownIn(s); ok {
			return v.Name, v.Domain
		}
	}
	for _, s := range segs[:len(segs)-1] {
		if isTimeSegment(s) || isGenericSegment(s) {
			continue
		}
		return strings.TrimSpace(strings.NewReplacer("-", " ", "_", " ").Replace(s)), ""
	}
	if text == "" {
		return "", ""
	}
	w := words(text)
	best, bestN := -1, 0
	for i, v := range knownVendors {
		if !v.InText {
			continue
		}
		n := 0
		for _, a := range v.Aliases {
			n += strings.Count(w, words(a))
		}
		if n > bestN {
			best, bestN = i, n
		}
	}
	if best < 0 {
		return "", ""
	}
	return knownVendors[best].Name, knownVendors[best].Domain
}

func hasAny(s string, subs ...string) bool {
	for _, x := range subs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}

func startsAny(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// startsWithMoney says whether s begins with a digit or a currency sign.
func startsWithMoney(s string) bool {
	for _, r := range s {
		return strings.ContainsRune("€$£0123456789", r) // privacy:ok (the digits, not a phone number)
	}
	return false
}

// detectKind reads the title of the document (its first lines), then its file
// name, then looks for an invoice number. A word in a sentence is not enough:
// « plateforme de réception des factures » is not an invoice.
// detectKind reads the title of the document: one of its first 3 lines, or a
// short line among the first 15 (a title under the address block, as on the
// receipts of the INPI), then its file name, then a few phrases of its text.
func detectKind(lines []string, name, text string) string {
	for i, l := range lines[:min(15, len(lines))] {
		if i >= 3 && len([]rune(l)) > 48 {
			continue
		}
		ll := strings.ToLower(l)
		switch {
		case startsAny(ll, "receipt", "reçu", "recu de paiement"):
			return "receipt"
		case startsAny(ll, "relevé", "transaction statement", "account statement"):
			return "statement"
		case startsAny(ll, "purchase order", "bon de commande"):
			return "order"
		case startsAny(ll, "invoice", "facture", "tax invoice", "bill reference"):
			return "invoice"
		}
	}
	n := words(name)
	switch {
	case hasAny(n, " receipt ", " recu "):
		return "receipt"
	case hasAny(n, " invoice ", " facture ", " bill "):
		return "invoice"
	case hasAny(n, " order ", " commande "):
		return "order"
	case hasAny(n, " transaction statement ", " releve "):
		return "statement"
	}
	if hasAny(strings.ToLower(text), "invoice number", "numéro de facture", "n° de facture", "date de facture", "invoice date", "reçu de paiement", "recu de paiement", "vaudra facture") {
		return "invoice"
	}
	return "document"
}

const monthRe = `(jan[a-z]*|f[eé]v[a-zé]*|feb[a-z]*|mar[a-z]*|apr[a-z]*|avr[a-z]*|ma[iy]|juin|june?|juil[a-z]*|july?|ao[uû]t|aug[a-z]*|sept?[a-z]*|oct[a-z]*|nov[a-z]*|d[eé]c[a-zé]*)\.?`

var (
	reMDY = regexp.MustCompile(`(?i)\b` + monthRe + `\s+(\d{1,2}),?\s+(\d{4})\b`)
	reDMY = regexp.MustCompile(`(?i)\b(\d{1,2})(?:er)?\s+` + monthRe + `\s+(\d{4})\b`)
	reNum = regexp.MustCompile(`\b(\d{1,2})/(\d{1,2})/(\d{4})\b`)
	reISO = regexp.MustCompile(`\b(\d{4})-(\d{2})-(\d{2})\b`)
)

func ymd(y, m, d int) string {
	if y < 2000 || y > 2100 || m < 1 || m > 12 || d < 1 || d > daysIn(y, m) {
		return ""
	}
	return strconv.Itoa(y) + "-" + pad2(m) + "-" + pad2(d)
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

// firstDate returns the earliest valid date written in s, whatever its format.
func firstDate(s string) string {
	best, bestAt := "", -1
	try := func(re *regexp.Regexp, conv func(m []string) string) {
		for _, loc := range re.FindAllStringSubmatchIndex(s, -1) {
			if bestAt >= 0 && loc[0] >= bestAt {
				return
			}
			m := make([]string, len(loc)/2)
			for i := range m {
				if loc[2*i] >= 0 {
					m[i] = s[loc[2*i]:loc[2*i+1]]
				}
			}
			if d := conv(m); d != "" {
				best, bestAt = d, loc[0]
				return
			}
		}
	}
	try(reMDY, func(m []string) string { return ymd(atoi(m[3]), monthOf(m[1]), atoi(m[2])) })
	try(reDMY, func(m []string) string { return ymd(atoi(m[3]), monthOf(m[2]), atoi(m[1])) })
	try(reNum, func(m []string) string { return ymd(atoi(m[3]), atoi(m[2]), atoi(m[1])) })
	try(reISO, func(m []string) string { return ymd(atoi(m[1]), atoi(m[2]), atoi(m[3])) })
	return best
}

var (
	rePeriodEN = regexp.MustCompile(`(?i)\b` + monthRe + `\s+(\d{1,2}),?\s*(?:(\d{4})\s*)?[–-]\s*(?:` + monthRe + `\s+)?(\d{1,2}),?\s+(\d{4})\b`)
	rePeriodFR = regexp.MustCompile(`(?i)\b(\d{1,2})(?:er)?\s+` + monthRe + `\s+(?:(\d{4})\s*)?[–-]\s*(\d{1,2})(?:er)?\s+` + monthRe + `\s+(\d{4})\b`)
)

// detectPeriod returns the length in days of the first billed period written
// as « Mon D–Mon D, YYYY », « Mon D – D, YYYY » or « D mois YYYY–D mois YYYY ».
func detectPeriod(text string) int {
	span := func(y1, m1, d1, y2, m2, d2 int) int {
		if m1 == 0 || m2 == 0 {
			return 0
		}
		if y1 == 0 {
			y1 = y2
			if m1 > m2 {
				y1--
			}
		}
		from, ok1 := parseDay(ymd(y1, m1, d1))
		to, ok2 := parseDay(ymd(y2, m2, d2))
		if days := daysBetween(from, to); ok1 && ok2 && days > 0 && days <= 400 {
			return days
		}
		return 0
	}
	for _, m := range rePeriodEN.FindAllStringSubmatch(text, 5) {
		m2 := monthOf(m[1])
		if m[4] != "" {
			m2 = monthOf(m[4])
		}
		if d := span(atoi(m[3]), monthOf(m[1]), atoi(m[2]), atoi(m[6]), m2, atoi(m[5])); d > 0 {
			return d
		}
	}
	for _, m := range rePeriodFR.FindAllStringSubmatch(text, 5) {
		if d := span(atoi(m[3]), monthOf(m[2]), atoi(m[1]), atoi(m[6]), monthOf(m[5]), atoi(m[4])); d > 0 {
			return d
		}
	}
	return 0
}

var dateLabels = []string{"date paid", "date de paiement", "payé le", "date of issue", "invoice date", "émission", "emission", "date de facture", "issue date", "date:", "généré le", "date"}

func detectDate(text string) string {
	lt := strings.ToLower(text)
	for _, l := range dateLabels {
		if i := strings.Index(lt, l); i >= 0 {
			if d := firstDate(lt[i:min(len(lt), i+len(l)+48)]); d != "" {
				return d
			}
		}
	}
	return firstDate(lt)
}

const numRe = `\d+(?:[ \x{00a0}\x{202f}.,]\d{3})*(?:[.,]\d{1,2})?`

var (
	reMoney      = regexp.MustCompile(`(?i)(?:([€$£])\s?(?:usd|eur)?\s?(` + numRe + `)|(` + numRe + `)\s?(€|eur\b|usd\b))`)
	reHeadline   = regexp.MustCompile(`(?i)([€$£])\s?(` + numRe + `)(?:\s?(?:usd|eur))?\s+(?:paid on|due)`)
	reHeadlineFR = regexp.MustCompile(`(?i)(` + numRe + `)\s?€\s+(?:dus? le|payés? le|prélevés? le)`)
	reBareNum    = regexp.MustCompile(`\d+[.,]\d{2}\b`)
)

var amountLabels = []struct {
	prefix   string
	lastBare bool // the amount carries no currency: take the last number of the line
}{
	{"amount paid", false}, {"montant payé", false}, {"bill total", false},
	{"total. including taxes", false}, {"total including taxes", false}, {"total ttc", false},
	{"net à payer", false}, {"montant ttc", false}, {"total à payer", false},
	{"montant prélevé", true}, {"montant total", false}, {"total", false},
	{"amount due", false}, {"montant dû", false},
}

func parseNumber(s string) (float64, bool) {
	s = strings.Map(func(r rune) rune {
		if r == ' ' || r == ' ' || r == ' ' {
			return -1
		}
		return r
	}, s)
	strip := strings.NewReplacer(".", "", ",", "")
	if i := strings.LastIndexAny(s, ".,"); i >= 0 && len(s)-i-1 >= 1 && len(s)-i-1 <= 2 {
		s = strip.Replace(s[:i]) + "." + s[i+1:]
	} else {
		s = strip.Replace(s)
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

func currencyOf(sym string) string {
	switch strings.ToLower(sym) {
	case "$", "usd":
		return "USD"
	case "£":
		return "GBP"
	}
	return "EUR"
}

func moneyIn(s string, lastBare bool) (float64, string, bool) {
	if lastBare {
		all := reBareNum.FindAllString(s, -1)
		if len(all) == 0 {
			return 0, "", false
		}
		v, ok := parseNumber(all[len(all)-1])
		return v, "EUR", ok
	}
	m := reMoney.FindStringSubmatch(s)
	if m == nil {
		return 0, "", false
	}
	sym, num := m[1], m[2]
	if num == "" {
		num, sym = m[3], m[4]
	}
	v, ok := parseNumber(num)
	return v, currencyOf(sym), ok
}

// detectAmount looks for the total paid: labelled lines first, then the
// « €21.60 paid on … » headline of Stripe-made invoices.
func detectAmount(lines []string, text string) (float64, string, bool) {
	lowered := make([]string, len(lines))
	for i, l := range lines {
		lowered[i] = strings.ToLower(l)
	}
	for _, lab := range amountLabels {
		for _, l := range lowered {
			if !strings.HasPrefix(l, lab.prefix) {
				continue
			}
			rest := strings.TrimLeft(l[len(lab.prefix):], " :")
			if lab.prefix == "total" && !startsWithMoney(rest) {
				continue
			}
			if v, cur, ok := moneyIn(rest, lab.lastBare); ok {
				return v, cur, true
			}
		}
	}
	lt := strings.ToLower(text)
	if m := reHeadline.FindStringSubmatch(lt); m != nil {
		if v, ok := parseNumber(m[2]); ok {
			return v, currencyOf(m[1]), true
		}
	}
	if m := reHeadlineFR.FindStringSubmatch(lt); m != nil {
		if v, ok := parseNumber(m[1]); ok {
			return v, "EUR", true
		}
	}
	return 0, "", false
}

func detectLabel(lines []string) string {
	header := func(s string) bool {
		return hasAny(strings.ToLower(s), "qty", "quantit", "qté", "unit", "amount", "montant", "prix", "rate")
	}
	for i, l := range lines {
		ll := strings.ToLower(l)
		if !strings.HasPrefix(ll, "description") {
			continue
		}
		if rest := strings.TrimSpace(l[len("description"):]); rest != "" && !header(rest) {
			return clip(rest, 48)
		}
		if i+1 < len(lines) {
			n := lines[i+1]
			nl := strings.ToLower(n)
			if !startsWithMoney(n) && !hasAny(nl, "subtotal", "sous-total", "total") && !header(n) {
				return clip(n, 48)
			}
		}
	}
	return ""
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

var reInvoiceNo = regexp.MustCompile(`(?i)(?:invoice number|invoice no\.?|numéro de facture|n° de facture|bill reference:?)\s*:?\s*([a-z0-9][a-z0-9_-]{3,})`)
var reReceiptNo = regexp.MustCompile(`(?i)receipt number\s*:?\s*([a-z0-9][a-z0-9-]{3,})`)

func splitLines(text string) []string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// extract reads what it can from a file's path and text (empty for non-PDFs).
func extract(rel, text string) Extracted {
	lines := splitLines(text)
	e := Extracted{Kind: detectKind(lines, path.Base(rel), text)}
	e.Vendor, e.Domain = detectVendor(rel, text)
	if text == "" {
		return e
	}
	if m := reInvoiceNo.FindStringSubmatch(text); m != nil {
		e.Number = m[1]
	}
	if e.Kind == "receipt" {
		e.Ref, e.Number = e.Number, ""
		if m := reReceiptNo.FindStringSubmatch(text); m != nil {
			e.Number = m[1]
		}
	}
	e.Date = detectDate(text)
	if v, cur, ok := detectAmount(lines, text); ok {
		e.Amount, e.Currency = &v, cur
	}
	e.Label = detectLabel(lines)
	e.PeriodDays = detectPeriod(text)
	return e
}
