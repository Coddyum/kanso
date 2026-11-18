// Package currency names the currencies the app knows: the euro and every
// currency the European Central Bank publishes a reference rate for.
package currency

// Codes are the ISO 4217 codes the app offers: the euro, the three most
// common on a freelancer's invoices, then the rest of the ECB's list.
var Codes = []string{
	"EUR", "USD", "GBP", "CHF",
	"AUD", "BRL", "CAD", "CNY", "CZK", "DKK", "HKD", "HUF", "IDR", "ILS", "INR", "ISK",
	"JPY", "KRW", "MXN", "MYR", "NOK", "NZD", "PHP", "PLN", "RON", "SEK", "SGD", "THB",
	"TRY", "ZAR",
}

var known = func() map[string]bool {
	m := make(map[string]bool, len(Codes))
	for _, c := range Codes {
		m[c] = true
	}
	return m
}()

// Known says whether code is one of Codes.
func Known(code string) bool { return known[code] }

// symbols are the signs a short notification writes instead of the code.
var symbols = map[string]string{
	"EUR": "€", "USD": "$", "GBP": "£", "JPY": "¥", "CNY": "¥", "INR": "₹", "KRW": "₩",
	"TRY": "₺", "ILS": "₪", "THB": "฿", "PHP": "₱",
}

// Symbol is a currency's sign (« € », « $ »), or its code when it has no sign of its own.
func Symbol(code string) string {
	if s := symbols[code]; s != "" {
		return s
	}
	return code
}

// Decimals is how many digits follow the decimal sign: none for the yen, the won and the króna.
func Decimals(code string) int {
	switch code {
	case "JPY", "KRW", "ISK":
		return 0
	}
	return 2
}
