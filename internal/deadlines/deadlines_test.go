package deadlines

import (
	"os"
	"testing"
	"time"

	"github.com/coddyum/kanso/internal/day"
	"github.com/coddyum/kanso/internal/i18n"
)

func date(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.Local) }

// french is the French of the real language files.
var french = func() i18n.Lang {
	c, err := i18n.Load(os.DirFS("../../web"))
	if err != nil {
		panic(err)
	}
	return c.Lang("fr")
}()

func TestUrssafQuarterly(t *testing.T) {
	var got []string
	for _, d := range List(Settings{Urssaf: "quarterly"}, map[string]bool{"urssaf@2026-04-30": true}, date(2026, 1, 1), date(2027, 6, 30), date(2026, 11, 18), french) {
		got = append(got, d.Date+" "+d.Status)
	}
	want := []string{
		"2026-02-02 past", // T4 2025 : le 31 janvier 2026 est un samedi
		"2026-04-30 done", // T1 2026
		"2026-07-31 past", // T2
		"2026-11-02 past", // T3 : le 31 octobre 2026 est un samedi, le 1er novembre un dimanche férié
		"2027-02-01 todo", // T4 : le 31 janvier 2027 est un dimanche
		"2027-04-30 todo", // T1 2027
	}
	assertList(t, got, want)
}

func TestUrssafMonthly(t *testing.T) {
	var got []string
	for _, d := range List(Settings{Urssaf: "monthly"}, nil, date(2026, 6, 1), date(2026, 9, 30), date(2026, 1, 1), french) {
		got = append(got, d.Date+" "+d.Detail)
	}
	want := []string{
		"2026-06-01 chiffre d'affaires d'avril 2026, à déclarer même s'il est à 0\u00a0€", // le 31 mai 2026 est un dimanche
		"2026-06-30 chiffre d'affaires de mai 2026, à déclarer même s'il est à 0\u00a0€",
		"2026-07-31 chiffre d'affaires de juin 2026, à déclarer même s'il est à 0\u00a0€",
		"2026-08-31 chiffre d'affaires de juillet 2026, à déclarer même s'il est à 0\u00a0€",
		"2026-09-30 chiffre d'affaires d'août 2026, à déclarer même s'il est à 0\u00a0€",
	}
	assertList(t, got, want)
}

func TestCFEAndIncomeTax(t *testing.T) {
	var got []string
	for _, d := range List(Settings{Urssaf: "off", CFE: true, IR: true}, nil, date(2026, 1, 1), date(2026, 12, 31), date(2026, 1, 1), french) {
		got = append(got, d.Date+" "+d.Kind)
	}
	assertList(t, got, []string{"2026-05-20 ir", "2026-12-15 cfe"})
}

func TestEaster(t *testing.T) {
	for y, want := range map[int]string{2025: "2025-04-20", 2026: "2026-04-05", 2027: "2027-03-28"} {
		if got := easterSunday(y).Format(day.Format); got != want {
			t.Errorf("Pâques %d : got %s, want %s", y, got, want)
		}
	}
}

func TestWorkingDay(t *testing.T) {
	cases := map[time.Time]string{
		date(2026, 5, 1):  "2026-05-04", // 1er mai, un vendredi férié, puis le week-end
		date(2026, 5, 14): "2026-05-15", // Ascension 2026
		date(2026, 6, 2):  "2026-06-02", // un mardi ordinaire
	}
	for in, want := range cases {
		if got := workingDay(in).Format(day.Format); got != want {
			t.Errorf("workingDay(%s) = %s, want %s", in.Format(day.Format), got, want)
		}
	}
}

func assertList(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d : got %q, want %q", i, got[i], want[i])
		}
	}
}
