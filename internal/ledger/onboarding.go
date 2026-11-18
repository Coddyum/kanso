package ledger

// Onboarding is where the first-run checklist stands: each step done or
// skipped, and whether the user put the list away. A database created by this
// version starts with one; an older database has none, and its user, who
// already set the app up, sees the list only when Settings opens it again.
type Onboarding struct {
	Steps  map[string]string `json:"steps"`  // step → done | skipped
	Closed bool              `json:"closed"` // put away by the user
}

// OnboardingSteps are the steps of the checklist, in the order they are shown.
var OnboardingSteps = []string{"lang", "folder", "reserve", "notify"}

var stepStates = map[string]bool{"todo": true, "done": true, "skipped": true}

// OnboardingStep is one line of the checklist.
type OnboardingStep struct {
	ID    string `json:"id"`
	State string `json:"state"` // todo | done | skipped
}

// OnboardingView is what the page needs: whether to open the list by itself, and its steps.
type OnboardingView struct {
	Show  bool             `json:"show"`
	Steps []OnboardingStep `json:"steps"`
}

// OnboardingView reads where the checklist stands.
func (d DB) OnboardingView() OnboardingView {
	v := OnboardingView{Show: d.Onboarding != nil && !d.Onboarding.Closed, Steps: []OnboardingStep{}}
	for _, id := range OnboardingSteps {
		state := "todo"
		if d.Onboarding != nil && d.Onboarding.Steps[id] != "" {
			state = d.Onboarding.Steps[id]
		}
		v.Steps = append(v.Steps, OnboardingStep{ID: id, State: state})
	}
	return v
}

// SetOnboardingStep marks a step done, skipped, or to do again.
func (b *Book) SetOnboardingStep(step, state string) error {
	known := false
	for _, id := range OnboardingSteps {
		known = known || id == step
	}
	if !known || !stepStates[state] {
		return Invalid("error.unknownStep")
	}
	return b.store.Update(func(db *DB) error {
		ob := db.onboarding()
		if state == "todo" {
			delete(ob.Steps, step)
		} else {
			ob.Steps[step] = state
		}
		return nil
	})
}

// ShowOnboarding opens the checklist again (Settings), or puts it away.
func (b *Book) ShowOnboarding(show bool) error {
	return b.store.Update(func(db *DB) error {
		db.onboarding().Closed = !show
		return nil
	})
}

// onboarding is the database's checklist, created when it has none.
func (d *DB) onboarding() *Onboarding {
	if d.Onboarding == nil {
		d.Onboarding = &Onboarding{}
	}
	if d.Onboarding.Steps == nil {
		d.Onboarding.Steps = map[string]string{}
	}
	return d.Onboarding
}
