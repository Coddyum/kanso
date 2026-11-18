package main

import (
	"testing"
	"time"
)

func TestUrssafQuarterlyDeadlines(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	to := time.Date(2027, 6, 30, 0, 0, 0, 0, time.Local)
	today := time.Date(2026, 11, 18, 0, 0, 0, 0, time.Local)
	var got []string
	for _, d := range deadlines(Admin{Urssaf: "quarterly"}, map[string]bool{"urssaf@2026-04-30": true}, from, to, today) {
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
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("échéance %d : got %s, want %s", i, got[i], want[i])
		}
	}
}

func TestEaster(t *testing.T) {
	for y, want := range map[int]string{2025: "2025-04-20", 2026: "2026-04-05", 2027: "2027-03-28"} {
		if got := easterSunday(y).Format(dayFmt); got != want {
			t.Errorf("Pâques %d : got %s, want %s", y, got, want)
		}
	}
}
