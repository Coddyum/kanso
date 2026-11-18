package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPath(t *testing.T) {
	c := checker{}
	cases := map[string]string{
		"main.go":                      "",
		"web/icon.svg":                 "",
		"README.md":                    "",
		"2026/Cloudflare/facture.pdf":  "facture, reçu ou export bancaire",
		"import/releve.CSV":            "facture, reçu ou export bancaire",
		"ticket.heic":                  "facture, reçu ou export bancaire",
		"db.json":                      "données de l'app ou du dossier perso",
		"Inc/.kanso/vendors.json":      "données de l'app ou du dossier perso",
		"renommage/appliquer.py":       "données de l'app ou du dossier perso",
		"extract/testdata/facture.pdf": "",
	}
	for p, want := range cases {
		if got := c.path(p); got != want {
			t.Errorf("path(%q) = %q, want %q", p, got, want)
		}
	}
}

func TestLine(t *testing.T) {
	c := checker{words: []string{"dupont", "12 rue des lilas"}}
	cases := []struct {
		file, text string
		want       []string
	}{
		{"main.go", `port := flag.Int("port", 48219, "port HTTP")`, nil},
		{"main.go", `const at = 1763500000000`, nil},
		{"tools/app/main.swift", `withCompletionHandler done: @escaping () -> Void`, nil},
		{"README.md", "IBAN : FR76 3000 6000 0112 3456 7890 189", []string{"IBAN"}},        // privacy:ok
		{"README.md", "écris à jean@exemple-perso.fr", []string{"adresse e-mail"}},         // privacy:ok
		{"README.md", "écris à contact@example.com", nil},                                  // privacy:ok
		{"scripts/a.sh", `DIR="/Users/jean/Documents/Inc"`, []string{"dossier personnel"}}, // privacy:ok
		{"scripts/a.sh", `DIR="$HOME/Documents/Inc"`, nil},
		{"README.md", "appelle le 06 12 34 56 78", []string{"numéro de téléphone"}}, // privacy:ok
		{"README.md", "SIRET 123 456 789 00012", []string{"numéro SIRET"}},          // privacy:ok
		{"README.md", "Facture de M. Dupont", []string{"mot de la liste privée"}},
		{"README.md", "IBAN FR76 3000 6000 0112 3456 7890 189 privacy:ok", nil}, // privacy:ok
		{"x/testdata/a.txt", "FR76 3000 6000 0112 3456 7890 189", nil},          // privacy:ok
		{"x/testdata/a.txt", "12 rue des Lilas", []string{"mot de la liste privée"}},
		{"README.md", "M. Dupont privacy:ok", []string{"mot de la liste privée"}},
	}
	for _, tc := range cases {
		if got := c.line(tc.file, tc.text); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("line(%q, %q) = %v, want %v", tc.file, tc.text, got, tc.want)
		}
	}
}

func TestHunkStart(t *testing.T) {
	for h, want := range map[string]int{"@@ -0,0 +1,12 @@": 1, "@@ -40 +41 @@ func x()": 41} {
		if got := hunkStart(h); got != want {
			t.Errorf("hunkStart(%q) = %d, want %d", h, got, want)
		}
	}
}

func TestMessage(t *testing.T) {
	c := checker{words: []string{"dupont"}}
	path := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	text := "fix: les factures de Dupont\n\nCorps sans rien\n# Commentaire de git : Dupont\n"
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := c.message(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []finding{{"message du commit", 1, "mot de la liste privée"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("message = %v, want %v", got, want)
	}
}
