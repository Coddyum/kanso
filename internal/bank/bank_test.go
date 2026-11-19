package bank

import (
	"fmt"
	"strings"
	"testing"

	"github.com/coddyum/kanso/internal/vendor"
)

// monthly writes n monthly lines with line(i) from the oldest, for a statement body.
func monthly(n int, line func(i int) string) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(line(i) + "\n")
	}
	return b.String()
}

func TestRevolut(t *testing.T) {
	csv := "Type,Product,Started Date,Completed Date,Description,Amount,Fee,Currency,State,Balance\n" +
		monthly(6, func(i int) string {
			return fmt.Sprintf("CARD_PAYMENT,Current,2026-%02d-03 10:00:00,2026-%02d-04 09:12:44,Notion,-10.00,0.00,EUR,COMPLETED,%d.50", i+5, i+5, 1000-i*10)
		}) +
		"TOPUP,Current,2026-10-20 08:00:00,2026-10-20 08:00:01,Payment from Client,2500.00,0.00,EUR,COMPLETED,3440.50\n" +
		"CARD_PAYMENT,Current,2026-10-21 12:00:00,2026-10-21 12:00:01,Boulangerie,-4.20,0.00,EUR,COMPLETED,3436.30\n"
	st, err := Read([]byte(csv))
	if err != nil || st.Format != "csv" {
		t.Fatalf("Read = %v, %v", st.Format, err)
	}
	m := st.Mapping
	if m.Date != 3 || m.Label != 4 || m.Amount != 5 || m.Balance != 9 || m.Currency != 7 {
		t.Errorf("mapping = %+v", m)
	}
	if len(st.Transactions) != 8 || st.Balance == nil || st.Balance.Amount != 3436.30 || st.Balance.Date != "2026-10-21" {
		t.Fatalf("transactions = %d, balance = %+v", len(st.Transactions), st.Balance)
	}
	subs := Recurring(st.Transactions, "", vendor.NewCatalog())
	if len(subs) != 1 || subs[0].Name != "Notion" || subs[0].Cycle != "monthly" || subs[0].Amount != 10 ||
		subs[0].Start != "2026-10-04" || subs[0].Domain != "notion.com" || subs[0].Count != 6 {
		t.Errorf("subscriptions = %+v", subs)
	}
}

func TestFrenchBanks(t *testing.T) {
	// Boursorama: « ; », decimals with a comma, the balance in its own column, newest first.
	bourso := "dateOp;dateVal;label;category;categoryParent;supplierFound;amount;comment;accountNum;accountLabel;accountbalance\n" +
		"2026-11-05;2026-11-05;\"PRLV SEPA FREE MOBILE ECH/051126 ID EMETTEUR/FR12ZZZ\";Téléphone;Abonnements;free mobile;-19,99;;00012345;BoursoBank;\"1 204,51\"\n" +
		"2026-10-05;2026-10-05;\"PRLV SEPA FREE MOBILE ECH/051026 ID EMETTEUR/FR12ZZZ\";Téléphone;Abonnements;free mobile;-19,99;;00012345;BoursoBank;\"1 224,50\"\n" +
		"2026-09-05;2026-09-05;\"PRLV SEPA FREE MOBILE ECH/050926 ID EMETTEUR/FR12ZZZ\";Téléphone;Abonnements;free mobile;-19,99;;00012345;BoursoBank;\"1 244,49\"\n"
	st, err := Read([]byte(bourso))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mapping.Date != 0 || st.Mapping.Label != 2 || st.Mapping.Amount != 6 || st.Mapping.Balance != 10 {
		t.Errorf("boursorama mapping = %+v", st.Mapping)
	}
	if st.Balance == nil || st.Balance.Amount != 1204.51 || st.Balance.Date != "2026-11-05" {
		t.Errorf("boursorama balance = %+v", st.Balance)
	}
	subs := Recurring(st.Transactions, "EUR", vendor.NewCatalog())
	if len(subs) != 1 || subs[0].Name != "Free Mobile" || subs[0].Amount != 19.99 {
		t.Errorf("boursorama subscriptions = %+v", subs)
	}

	// Crédit Agricole: Windows-1252, lines before the table, debit and credit apart, dd/mm/yyyy.
	ca := "Compte de dépôt;n° 123\n\nSolde au 18/11/2026 2 000,00 EUR\n\nDate;Libellé;Débit euros;Crédit euros;\n" +
		monthly(4, func(i int) string {
			return fmt.Sprintf("%02d/%02d/2026;\"PRLV SEPA OVH SAS\";5,99;;", 12, 11-i)
		}) + "15/11/2026;\"VIR CLIENT\";;1 500,00;\n"
	latin := []byte{}
	for _, r := range ca {
		latin = append(latin, byte(r)) // every character of this text is in Latin-1
	}
	st, err = Read(latin)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mapping.Debit != 2 || st.Mapping.Credit != 3 || st.Mapping.Label != 1 || len(st.Transactions) != 5 {
		t.Fatalf("crédit agricole: mapping %+v, %d transactions", st.Mapping, len(st.Transactions))
	}
	if st.Transactions[0].Date.Format("2006-01-02") != "2026-08-12" || st.Transactions[4].Amount != 1500 {
		t.Errorf("crédit agricole transactions = %+v", st.Transactions)
	}
	subs = Recurring(st.Transactions, "EUR", vendor.NewCatalog())
	if len(subs) != 1 || subs[0].Name != "OVHcloud" || subs[0].Amount != 5.99 || subs[0].Domain != "ovhcloud.com" {
		t.Errorf("crédit agricole subscriptions = %+v", subs)
	}
}

func TestNoHeaderAndUS(t *testing.T) {
	// LCL: no header; the amount right after the date.
	lcl := monthly(3, func(i int) string {
		return fmt.Sprintf("%02d/%02d/2026;-12,99;Carte;;CB NETFLIX.COM %02d/%02d;", 2, 9+i, 2, 9+i)
	})
	st, err := Read([]byte(lcl))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mapping.Amount != 1 || st.Mapping.Label != 4 || len(st.Transactions) != 3 {
		t.Fatalf("lcl: mapping %+v, %d transactions", st.Mapping, len(st.Transactions))
	}
	if subs := Recurring(st.Transactions, "EUR", vendor.NewCatalog()); len(subs) != 1 || subs[0].Name != "Netflix" || subs[0].Domain != "netflix.com" {
		t.Errorf("lcl subscriptions = %+v", subs)
	}

	// Headers in an unknown language, a column of references: the amount is the column with debits.
	odd := "kol1;kol2;kol3;kol4\n21.11.2025;71766;-4,00;GitHub\n22.11.2025;54937;-27,36;Monoprix\n23.11.2025;12001;1 200,00;Client\n"
	st, err = Read([]byte(odd))
	if err != nil || st.Mapping.Amount != 2 || st.Mapping.Label != 3 || st.Mapping.Balance != -1 || len(st.Table.Columns) != 4 {
		t.Errorf("unknown headers: %v, mapping %+v, columns %q", err, st.Mapping, st.Table.Columns)
	}

	// A US bank: month first, « , » and quoted amounts with a thousands comma.
	us := "Transaction Date,Post Date,Description,Category,Type,Amount,Memo\n" +
		"11/28/2026,11/29/2026,GITHUB INC,Software,Sale,\"-4.00\",\n" +
		"10/28/2026,10/29/2026,GITHUB INC,Software,Sale,\"-4.00\",\n" +
		"10/02/2026,10/03/2026,PAYROLL,Income,Credit,\"1,234.56\",\n"
	st, err = Read([]byte(us))
	if err != nil {
		t.Fatal(err)
	}
	if got := st.Transactions[len(st.Transactions)-1].Date.Format("2006-01-02"); got != "2026-11-28" {
		t.Errorf("us: last date %s", got)
	}
	if st.Transactions[0].Amount != 1234.56 {
		t.Errorf("us: amount %v", st.Transactions[0].Amount)
	}
	if subs := Recurring(st.Transactions, "USD", vendor.NewCatalog()); len(subs) != 1 || subs[0].Name != "GitHub" || subs[0].Currency != "USD" {
		t.Errorf("us subscriptions (two debits of the same amount a month apart) = %+v", subs)
	}
}

func TestOFX(t *testing.T) {
	ofx := "OFXHEADER:100\nDATA:OFXSGML\n\n<OFX><BANKMSGSRSV1><STMTTRNRS><STMTRS><CURDEF>EUR<BANKTRANLIST>\n" +
		monthly(4, func(i int) string {
			return fmt.Sprintf("<STMTTRN><TRNTYPE>DEBIT<DTPOSTED>2026%02d15120000[-5:EST]<TRNAMT>-23,50<FITID>%d<NAME>PRLV SEPA FIGMA<MEMO>FIGMA INC</STMTTRN>", 7+i, i)
		}) +
		"</BANKTRANLIST><LEDGERBAL><BALAMT>842.10<DTASOF>20261118</LEDGERBAL></STMTRS></STMTTRNRS></BANKMSGSRSV1></OFX>\n"
	st, err := Read([]byte(ofx))
	if err != nil || st.Format != "ofx" || len(st.Transactions) != 4 {
		t.Fatalf("ofx: %v, %d transactions", err, len(st.Transactions))
	}
	if st.Balance == nil || st.Balance.Amount != 842.10 || st.Balance.Date != "2026-11-18" || st.Balance.Currency != "EUR" {
		t.Errorf("ofx balance = %+v", st.Balance)
	}
	if subs := Recurring(st.Transactions, st.Currency, vendor.NewCatalog()); len(subs) != 1 || subs[0].Name != "Figma" || subs[0].Amount != 23.5 {
		t.Errorf("ofx subscriptions = %+v", subs)
	}
}

func TestNotSubscriptions(t *testing.T) {
	csv := "Date;Libellé;Montant\n" +
		// groceries: same shop, amounts all over the place
		"03/09/2026;CB CARREFOUR;-82,10\n03/10/2026;CB CARREFOUR;-12,40\n03/11/2026;CB CARREFOUR;-140,00\n" +
		// a stopped subscription: its latest debit is months before the end of the file
		"01/03/2026;PRLV SPOTIFY;-10,99\n01/04/2026;PRLV SPOTIFY;-10,99\n01/05/2026;PRLV SPOTIFY;-10,99\n" +
		// URSSAF: the administrative deadlines follow it
		"05/08/2026;PRLV URSSAF;-300,00\n05/09/2026;PRLV URSSAF;-310,00\n05/10/2026;PRLV URSSAF;-305,00\n" +
		"18/11/2026;VIR CLIENT;1000,00\n"
	st, err := Read([]byte(csv))
	if err != nil {
		t.Fatal(err)
	}
	if subs := Recurring(st.Transactions, "EUR", vendor.NewCatalog()); len(subs) != 0 {
		t.Errorf("none of these is a running subscription: %+v", subs)
	}
}

func TestUnreadableAndMapping(t *testing.T) {
	if _, err := Read([]byte("hello\nworld\n")); err != ErrUnreadable {
		t.Errorf("plain text: %v", err)
	}
	// No text column: no label, the user chooses which column names the payee.
	csv := "x;y;z\n01/02/2026;1001;12,00\n02/02/2026;1002;13,00\n"
	st, err := Read([]byte(csv))
	if err != nil || st.Table == nil || len(st.Transactions) != 0 {
		t.Fatalf("a table whose columns cannot be told apart: %v, %d transactions", err, len(st.Transactions))
	}
	m := NoMapping
	m.Date, m.Label, m.Amount = 0, 1, 2
	txs, err := st.Table.Read(m)
	if err != nil || len(txs) != 2 || txs[1].Amount != 13 {
		t.Errorf("chosen mapping: %v, %+v", err, txs)
	}
	m.Amount = 9
	if _, err := st.Table.Read(m); err != ErrMapping {
		t.Errorf("a column out of the table: %v", err)
	}
}

func TestAmounts(t *testing.T) {
	cases := []struct {
		in   string
		dec  byte
		want float64
	}{
		{"-1 234,56", ',', -1234.56}, {"1.234,56", ',', 1234.56}, {"1,234.56", '.', 1234.56}, {"12,99-", ',', -12.99},
		{"(12.99)", '.', -12.99}, {"€12", '.', 12}, {"EUR 8.89", '.', 8.89}, {"1'234.50", '.', 1234.5}, {"−5,00", ',', -5},
	}
	for _, c := range cases {
		if got, ok := parseAmount(c.in, c.dec); !ok || got != c.want {
			t.Errorf("parseAmount(%q) = %v, %v; want %v", c.in, got, ok, c.want)
		}
	}
	if _, ok := parseAmount("CB NETFLIX 12/11", '.'); ok {
		t.Error("a label is not an amount")
	}
}
