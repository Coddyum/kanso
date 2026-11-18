package main

import "testing"

func TestResolveFolder(t *testing.T) {
	home := "/tmp/home"
	cases := map[string]string{
		"Factures":         "/tmp/home/Documents/Factures",
		"~/Desktop/Kanso":  "/tmp/home/Desktop/Kanso",
		"/Volumes/Data/X":  "/Volumes/Data/X",
		" Mes factures  ":  "/tmp/home/Documents/Mes factures",
		"~/Documents/Inc/": "/tmp/home/Documents/Inc",
	}
	for in, want := range cases {
		if got, err := resolveFolder(home, in); err != nil || got != want {
			t.Errorf("resolveFolder(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "~", "/"} {
		if _, err := resolveFolder(home, in); err == nil {
			t.Errorf("resolveFolder(%q) must be refused", in)
		}
	}
}
