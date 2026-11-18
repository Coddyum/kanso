package currency

import "testing"

func TestCodes(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range Codes {
		if len(c) != 3 || seen[c] {
			t.Errorf("code %q is not a new three-letter code", c)
		}
		seen[c] = true
	}
	if Codes[0] != "EUR" || !Known("CHF") || Known("BGN") || Known("eur") {
		t.Error("EUR first, CHF known, BGN (euro since 2026) and lower case unknown")
	}
}

func TestSymbol(t *testing.T) {
	for code, want := range map[string]string{"EUR": "€", "USD": "$", "GBP": "£", "CHF": "CHF"} {
		if got := Symbol(code); got != want {
			t.Errorf("Symbol(%s) = %q, want %q", code, got, want)
		}
	}
}
