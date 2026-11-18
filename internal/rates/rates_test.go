package rates

import "testing"

const sample = `<Cube><Cube time='2026-11-18'>
<Cube currency='USD' rate='1.1600'/>
<Cube currency='GBP' rate='0.8500'/>
</Cube></Cube>`

func TestParseECB(t *testing.T) {
	r, ok := parseECB([]byte(sample))
	if !ok || r.Date != "2026-11-18" || r.PerEUR["USD"] != 1.16 || r.PerEUR["GBP"] != 0.85 {
		t.Fatalf("parseECB = %+v, %v", r, ok)
	}
	if _, ok := parseECB([]byte("<html>maintenance</html>")); ok {
		t.Error("a page with no rate must not replace the cached rates")
	}
}

func TestToEUR(t *testing.T) {
	r, _ := parseECB([]byte(sample))
	cases := []struct {
		amount   float64
		currency string
		want     float64
		ok       bool
	}{
		{10, "EUR", 10, true},
		{10, "", 10, true},
		{11.6, "USD", 10, true},
		{8.5, "GBP", 10, true},
		{10, "CHF", 0, false},
	}
	for _, c := range cases {
		got, ok := r.ToEUR(c.amount, c.currency)
		if ok != c.ok || (ok && (got-c.want > 1e-9 || c.want-got > 1e-9)) {
			t.Errorf("ToEUR(%v %s) = %v, %v; want %v, %v", c.amount, c.currency, got, ok, c.want, c.ok)
		}
	}
}
