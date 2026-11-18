package ledger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/coddyum/kanso/internal/vendor"
)

// TestOnboarding: a new database opens the checklist, an older one does not;
// steps are ticked, skipped, taken up again, and the list put away and reopened.
func TestOnboarding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.json")
	b, err := Open(path, vendor.NewCatalog())
	if err != nil {
		t.Fatal(err)
	}
	v := b.Snapshot().OnboardingView()
	if !v.Show || len(v.Steps) != len(OnboardingSteps) || v.Steps[0].State != "todo" {
		t.Fatalf("a new database shows the checklist: %+v", v)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("a new database is written at once: %v", err)
	}
	if err := b.SetOnboardingStep("lang", "done"); err != nil {
		t.Fatal(err)
	}
	if err := b.SetOnboardingStep("folder", "skipped"); err != nil {
		t.Fatal(err)
	}
	if err := b.SetOnboardingStep("nope", "done"); !isInvalid(err) {
		t.Errorf("an unknown step: %v", err)
	}
	if err := b.SetOnboardingStep("lang", "maybe"); !isInvalid(err) {
		t.Errorf("an unknown state: %v", err)
	}
	if err := b.SetOnboardingStep("folder", "todo"); err != nil {
		t.Fatal(err)
	}
	v = b.Snapshot().OnboardingView()
	if v.Steps[0].State != "done" || v.Steps[1].State != "todo" {
		t.Errorf("steps = %+v", v.Steps)
	}
	if err := b.ShowOnboarding(false); err != nil || b.Snapshot().OnboardingView().Show {
		t.Errorf("put away: %v", err)
	}
	if err := b.ShowOnboarding(true); err != nil || !b.Snapshot().OnboardingView().Show {
		t.Errorf("opened again: %v", err)
	}

	old := filepath.Join(t.TempDir(), "db.json")
	if err := os.WriteFile(old, []byte(`{"payments":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	b2, err := Open(old, vendor.NewCatalog())
	if err != nil {
		t.Fatal(err)
	}
	if b2.Snapshot().OnboardingView().Show || b2.Version() != 0 {
		t.Error("a database from before the checklist neither shows it nor is written at opening")
	}
}
