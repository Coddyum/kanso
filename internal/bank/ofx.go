package bank

import (
	"regexp"
	"strings"
)

// OFX, in its two shapes: SGML (OFX 1, tags left open) and XML (OFX 2).
var (
	reTxn     = regexp.MustCompile(`(?is)<STMTTRN>(.*?)(?:</STMTTRN>|<STMTTRN>|</BANKTRANLIST>)`)
	reLedger  = regexp.MustCompile(`(?is)<LEDGERBAL>(.*?)(?:</LEDGERBAL>|<AVAILBAL>|</STMTRS>)`)
	reOFXHead = regexp.MustCompile(`(?i)OFXHEADER|<OFX>`)
)

func looksOFX(text string) bool { return reOFXHead.MatchString(text[:min(len(text), 4000)]) }

// tag is the value of <name> in an OFX block: up to the next tag or the end of the line.
func tag(block, name string) string {
	i := strings.Index(strings.ToUpper(block), "<"+name+">")
	if i < 0 {
		return ""
	}
	v := block[i+len(name)+2:]
	if j := strings.IndexAny(v, "<\r\n"); j >= 0 {
		v = v[:j]
	}
	return strings.TrimSpace(htmlUnescape(v))
}

var htmlEntities = strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&apos;", "'")

func htmlUnescape(s string) string { return htmlEntities.Replace(s) }

// readOFX reads the transactions (date, amount, name and memo) and the ledger balance.
func readOFX(text string) (Statement, error) {
	st := Statement{Format: "ofx", Currency: strings.ToUpper(tag(text, "CURDEF"))}
	for i, m := range reTxn.FindAllStringSubmatch(text, -1) {
		block := m[1]
		d, ok := parseDate(tag(block, "DTPOSTED"), "dmy")
		if !ok {
			continue
		}
		raw := tag(block, "TRNAMT")
		amount, ok := parseAmount(raw, decimalOf([]string{raw}))
		if !ok {
			continue
		}
		label := tag(block, "NAME")
		if memo := tag(block, "MEMO"); memo != "" && !strings.Contains(strings.ToLower(label), strings.ToLower(memo)) {
			label = strings.TrimSpace(label + " " + memo)
		}
		cur := strings.ToUpper(tag(block, "CURRENCY"))
		if c := tag(block, "CURSYM"); c != "" {
			cur = strings.ToUpper(c)
		}
		st.Transactions = append(st.Transactions, Transaction{Date: d, Label: strings.Join(strings.Fields(label), " "), Amount: amount, Currency: cur, order: i})
	}
	if len(st.Transactions) == 0 {
		return st, ErrUnreadable
	}
	if m := reLedger.FindStringSubmatch(text); m != nil {
		raw := tag(m[1], "BALAMT")
		d, okD := parseDate(tag(m[1], "DTASOF"), "dmy")
		if v, ok := parseAmount(raw, decimalOf([]string{raw})); ok && okD {
			st.Balance = &Balance{Amount: v, Date: d.Format("2006-01-02"), Currency: st.Currency}
		}
	}
	st.finish()
	return st, nil
}
