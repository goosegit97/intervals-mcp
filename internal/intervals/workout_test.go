package intervals

import "testing"

// TestStepNameOrder: name-before-duration on labelled steps.
func TestStepNameOrder(t *testing.T) {
	cases := []struct {
		step    string
		wantErr bool
	}{
		{"3m Leg press Press lap", true},
		{"Leg press 3m Press lap", false},
		{"Warmup 10m 60%", false},
		{"4m 105%", false}, // duration-first but only intensity follows: legit cycling step
	}
	for _, c := range cases {
		err := validateStepNameOrder(c.step)
		if (err != nil) != c.wantErr {
			t.Errorf("validateStepNameOrder(%q) err=%v, wantErr=%v", c.step, err, c.wantErr)
		}
	}
}

// TestExpandBracketRepeats: the inline "Nx [ ... ]" form must
// expand to explicit steps, none of which retain bracket repeat syntax.
func TestExpandBracketRepeats(t *testing.T) {
	out, err := expandBracketRepeats("2x [Leg press 3m / Rest 1m]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	steps := stepLines(out)
	if len(steps) != 4 {
		t.Fatalf("expected 4 steps, got %d: %#v", len(steps), steps)
	}
	for _, s := range steps {
		if got, _ := expandBracketRepeats(s); got != s {
			t.Errorf("step %q still contains a bracket repeat", s)
		}
	}
}

// TestSwimStepFormat: distance-based swim steps only.
func TestSwimStepFormat(t *testing.T) {
	if err := validateSwimStep("100 1:30 pace"); err == nil {
		t.Error("expected pace-based swim step to be rejected")
	}
	if err := validateSwimStep("500mtr Continuous"); err != nil {
		t.Errorf("expected distance-based swim step to pass, got %v", err)
	}
}

// TestHRIntensity: HR intensity syntax is order-dependent.
func TestHRIntensity(t *testing.T) {
	bad := []string{"hr Z2", "130-150bpm"}
	good := []string{"Z2 HR", "70-81% HR", "75-85% LTHR"}
	for _, s := range bad {
		if err := validateHRIntensity(s); err == nil {
			t.Errorf("expected %q to be rejected", s)
		}
	}
	for _, s := range good {
		if err := validateHRIntensity(s); err != nil {
			t.Errorf("expected %q to pass, got %v", s, err)
		}
	}
}

// TestCombinedExercise: one exercise per step.
func TestCombinedExercise(t *testing.T) {
	if err := validateNoCombinedExercise("Bicep curl + Tricep pushdown 3m"); err == nil {
		t.Error("expected combined exercise step to be rejected")
	}
	if err := validateNoCombinedExercise("Bicep curl 3m"); err != nil {
		t.Errorf("expected single exercise step to pass, got %v", err)
	}
}

// TestStrengthTypeRedirect: strength types are recognised so
// the handler can redirect them to the garmin service.
func TestStrengthTypeRedirect(t *testing.T) {
	for _, s := range []string{"WeightTraining", "weight training", "Strength"} {
		if !isStrengthType(s) {
			t.Errorf("expected %q to be recognised as a strength type", s)
		}
	}
	if isStrengthType("Ride") {
		t.Error("Ride should not be a strength type")
	}
}
