package brand

import (
	"os"
	"strings"
	"testing"
)

// TestShellAgrees keeps scripts/brand.sh equal to the Go constants.
func TestShellAgrees(t *testing.T) {
	data, err := os.ReadFile("../../scripts/brand.sh")
	if err != nil {
		t.Fatal(err)
	}
	vars := map[string]string{}
	for _, l := range strings.Split(string(data), "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(l), "="); ok && !strings.HasPrefix(k, "#") {
			vars[k] = strings.Trim(v, `"`)
		}
	}
	want := map[string]string{"NAME": Name, "ID": ID, "DATA_DIR": DataDir, "BUNDLE_ID": BundleID}
	for k, v := range want {
		if vars[k] != v {
			t.Errorf("scripts/brand.sh %s = %q, internal/brand says %q", k, vars[k], v)
		}
	}
}
