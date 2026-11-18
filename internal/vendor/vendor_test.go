package vendor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFind(t *testing.T) {
	c := NewCatalog()
	cases := map[string]string{"2026/OVH/Mars": "OVHcloud", "Free Mobile - mandat": "Free Mobile", "free": "Free", "Rien de connu": ""}
	for in, want := range cases {
		v, _ := c.Find(in)
		if v.Name != want {
			t.Errorf("Find(%q) = %q, want %q", in, v.Name, want)
		}
	}
	if v, ok := c.InText("Votre forfait Free ce mois-ci"); ok {
		t.Errorf("a word too common is not read in a text: %q", v.Name)
	}
}

func TestLoadLocal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vendors.json")
	data := `{"vendors":[{"name":"Atelier Lune","domain":"https://www.atelier-lune.fr/x"}],"billing":{"atelier-lune.fr":"atelier-lune.fr/factures"}}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	c := NewCatalog()
	if err := c.LoadLocal(path); err != nil {
		t.Fatal(err)
	}
	if v, ok := c.Find("2026/atelier lune/facture.pdf"); !ok || v.Domain != "atelier-lune.fr" {
		t.Errorf("Find = %+v, %v", v, ok)
	}
	if got := c.BillingPage("atelier-lune.fr", ""); got != "https://atelier-lune.fr/factures" {
		t.Errorf("BillingPage = %q", got)
	}
	if err := NewCatalog().LoadLocal(filepath.Join(t.TempDir(), "absent.json")); err != nil {
		t.Errorf("a missing file adds nothing: %v", err)
	}
	if NewCatalog().BillingPage("atelier-lune.fr", "") != "" {
		t.Error("one catalog's vendors must not leak into another")
	}
}

func TestMatchesAndClean(t *testing.T) {
	if !Matches("OVH", "OVHcloud") || Matches("", "OVH") || Matches("Render", "Neon") {
		t.Error("Matches")
	}
	for in, want := range map[string]string{"https://www.Render.com/pricing": "render.com", "neon.tech": "neon.tech"} {
		if got := CleanDomain(in); got != want {
			t.Errorf("CleanDomain(%q) = %q", in, got)
		}
	}
	for in, want := range map[string]string{"site.fr/x": "https://site.fr/x", "javascript:alert(1)": "", "": ""} {
		if got := CleanURL(in); got != want {
			t.Errorf("CleanURL(%q) = %q, want %q", in, got, want)
		}
	}
}
