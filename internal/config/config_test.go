package config

import "testing"

func TestLoadSave(t *testing.T) {
	home := t.TempDir()
	c, err := Load(home)
	if err != nil || c.Root != "" || c.Port != DefaultPort {
		t.Fatalf("no file: %+v, %v; want the default", c, err)
	}
	if err := Save(home, Config{Root: "relative/path"}); err == nil {
		t.Error("a relative folder must be refused")
	}
	if err := Save(home, Config{Root: "/tmp/kanso-test/Invoices", Port: 0}); err != nil {
		t.Fatal(err)
	}
	c, err = Load(home)
	if err != nil || c.Root != "/tmp/kanso-test/Invoices" || c.Port != DefaultPort {
		t.Errorf("Load = %+v, %v", c, err)
	}
}
