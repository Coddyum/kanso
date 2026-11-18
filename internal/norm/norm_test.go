package norm

import "testing"

func TestWords(t *testing.T) {
	cases := map[string]string{
		"Free Mobile – Mandat": " free mobile mandat ",
		"Électricité_Août":     " electricite aout ",
		"":                     " ",
		"OVH.cloud":            " ovh cloud ",
	}
	for in, want := range cases {
		if got := Words(in); got != want {
			t.Errorf("Words(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestKey(t *testing.T) {
	if got := Key("OVH cloud"); got != "ovhcloud" {
		t.Errorf("Key = %q", got)
	}
	if Key("Société Générale") != Key("societe-generale") {
		t.Error("accents and separators must not change the key")
	}
}

func TestClip(t *testing.T) {
	if got := Clip("abcdef", 4); got != "abc…" {
		t.Errorf("Clip = %q", got)
	}
	if got := Clip("abc", 4); got != "abc" {
		t.Errorf("Clip short = %q", got)
	}
}
