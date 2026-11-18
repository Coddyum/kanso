package day

import (
	"testing"
	"time"
)

func TestAddMonths(t *testing.T) {
	cases := []struct {
		from string
		n    int
		want string
	}{
		{"2026-01-31", 1, "2026-02-28"},
		{"2028-01-31", 1, "2028-02-29"},
		{"2026-03-31", 1, "2026-04-30"},
		{"2026-11-15", 3, "2027-02-15"},
		{"2026-05-10", -5, "2025-12-10"},
	}
	for _, c := range cases {
		from, _ := Parse(c.from)
		if got := AddMonths(from, c.n).Format(Format); got != c.want {
			t.Errorf("AddMonths(%s, %d) = %s, want %s", c.from, c.n, got, c.want)
		}
	}
}

func TestGapAndBetween(t *testing.T) {
	if got := Gap("2026-03-28", "2026-04-02"); got != 5 {
		t.Errorf("Gap across the clock change = %d, want 5", got)
	}
	if got := Gap("2026-04-02", "2026-03-28"); got != 5 {
		t.Errorf("Gap backwards = %d, want 5", got)
	}
	if got := Gap("2026-04-02", "nope"); got < 1000 {
		t.Errorf("Gap with an unreadable day = %d, want huge", got)
	}
	a, _ := Parse("2026-10-24")
	b, _ := Parse("2026-11-01")
	if got := Between(a, b); got != 8 {
		t.Errorf("Between = %d, want 8", got)
	}
}

func TestYMD(t *testing.T) {
	for _, c := range []struct {
		y, m, d int
		want    string
	}{
		{2026, 2, 28, "2026-02-28"},
		{2026, 2, 29, ""},
		{1999, 1, 1, ""},
		{2026, 13, 1, ""},
		{2026, 7, 4, "2026-07-04"},
	} {
		if got := YMD(c.y, c.m, c.d); got != c.want {
			t.Errorf("YMD(%d, %d, %d) = %q, want %q", c.y, c.m, c.d, got, c.want)
		}
	}
}

func TestMonths(t *testing.T) {
	for word, want := range map[string]int{"Août": 8, "sept": 9, "December": 12, "mai": 5, "Lundi": 0} {
		if got := MonthOf(word); got != want {
			t.Errorf("MonthOf(%q) = %d, want %d", word, got, want)
		}
	}
	for name, want := range map[string]bool{"2026": true, "Octobre": true, "10 - Octobre": true, "Cloudflare": false, "": false} {
		if got := IsTimeName(name); got != want {
			t.Errorf("IsTimeName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestOf(t *testing.T) {
	in := time.Date(2026, 11, 18, 23, 59, 0, 0, time.Local)
	if got := Of(in); !got.Equal(time.Date(2026, 11, 18, 0, 0, 0, 0, time.Local)) {
		t.Errorf("Of = %v", got)
	}
}
