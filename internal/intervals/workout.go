package intervals

import (
	"fmt"
	"regexp"
	"strings"
)

// This file holds the pre-send validation for the Intervals.icu workout-text
// language. Every constraint here was discovered empirically by pushing to the
// live API and reading the result on the watch: the API accepts malformed
// steps silently and the wrong workout lands on the device with no error. We
// catch those classes before the POST instead.

// durationToken matches an Intervals.icu duration/distance leading a step, e.g.
// "3m", "45s", "1.5h", "500mtr", "2km", "100yd". Intensity tokens (%/W/zones)
// are deliberately excluded so a legitimate "4m 105%" cycling step is not read
// as "duration before a name".
var durationToken = regexp.MustCompile(`^\d+(\.\d+)?(s|m|h|mtr|km|yd)$`)

// paceToken matches an mm:ss pace such as "1:30" or "1:45/100m".
var paceToken = regexp.MustCompile(`\b\d+:\d{2}\b`)

// distanceToken matches a distance such as "500mtr", "2km", "100yd", "400m".
var distanceToken = regexp.MustCompile(`\b\d+(\.\d+)?(mtr|km|yd|m)\b`)

// bracketRepeat matches the inline "Nx [ ... ]" repeat form that collapses
// silently to a single cycle. The native newline "3x" form is left
// untouched — it parses correctly.
var bracketRepeat = regexp.MustCompile(`(?i)(\d+)\s*x\s*\[([^\]]*)\]`)

// zoneToken matches a bare zone word such as "z2"; wattOrPercentToken matches a
// single value or range with a W/% suffix ("250w", "70-81%"). Both classify a
// word as intensity rather than part of an exercise name.
var (
	zoneToken          = regexp.MustCompile(`^z\d$`)
	wattOrPercentToken = regexp.MustCompile(`(?i)^\d+(-\d+)?(w|%)$`)
)

// bpmValue matches an absolute bpm target ("130-150bpm"), which Intervals.icu
// does not parse; hrBeforeValue matches "HR" leading its value ("hr z2"), the
// wrong order. Both leave no target on the watch.
var (
	bpmValue      = regexp.MustCompile(`\d+\s*-?\s*\d*\s*bpm`)
	hrBeforeValue = regexp.MustCompile(`(^|\s)hr\s+z?\d`)
)

// isStrengthType reports whether a type string is a Garmin/Intervals strength
// variant that must be redirected to the garmin service.
func isStrengthType(workoutType string) bool {
	switch strings.ToLower(strings.TrimSpace(workoutType)) {
	case "weighttraining", "weight training", "strength", "workout_strength":
		return true
	}
	return false
}

// ValidateWorkout applies every workout-text constraint relevant to the given
// type/target before the description is sent to the API. It returns a single
// descriptive error on the first violation, or a possibly-rewritten description
// with bracket repeats expanded into explicit steps.
//
// Strength (WeightTraining) creation is rejected upstream in the handler and
// redirected to the garmin service; this function still enforces
// name-before-duration so the rule is testable in isolation.
func ValidateWorkout(workoutType, target, description string) (string, error) {
	expanded, err := expandBracketRepeats(description)
	if err != nil {
		return "", err
	}
	for _, step := range stepLines(expanded) {
		if err := validateNoCombinedExercise(step); err != nil {
			return "", err
		}
		if err := validateStepNameOrder(step); err != nil {
			return "", err
		}
		if strings.EqualFold(workoutType, "Swim") {
			if err := validateSwimStep(step); err != nil {
				return "", err
			}
		}
		if strings.EqualFold(target, "HR") {
			if err := validateHRIntensity(step); err != nil {
				return "", err
			}
		}
	}
	return expanded, nil
}

// stepLines splits a workout description into individual step strings, dropping
// blank lines and the leading "- " list marker.
func stepLines(description string) []string {
	var steps []string
	for _, raw := range strings.Split(description, "\n") {
		line := strings.TrimSpace(raw)
		line = strings.TrimPrefix(line, "-")
		line = strings.TrimSpace(line)
		if line != "" {
			steps = append(steps, line)
		}
	}
	return steps
}

// expandBracketRepeats expands any "Nx [ a / b ]" block into N explicit copies
// of its child steps (one per line), so the collapsing inline form never reaches
// the API. It errors if a bracket block is empty.
func expandBracketRepeats(description string) (string, error) {
	var expandErr error
	out := bracketRepeat.ReplaceAllStringFunc(description, func(match string) string {
		m := bracketRepeat.FindStringSubmatch(match)
		count := m[1]
		inner := strings.TrimSpace(m[2])
		if inner == "" {
			expandErr = fmt.Errorf("empty repeat block %q: nothing to expand", match)
			return match
		}
		children := splitRepeatChildren(inner)
		n := 0
		fmt.Sscanf(count, "%d", &n)
		if n < 1 {
			expandErr = fmt.Errorf("invalid repeat count in %q", match)
			return match
		}
		var lines []string
		for i := 0; i < n; i++ {
			for _, child := range children {
				lines = append(lines, "- "+child)
			}
		}
		return strings.Join(lines, "\n")
	})
	if expandErr != nil {
		return "", expandErr
	}
	return out, nil
}

// splitRepeatChildren splits the inside of a repeat bracket on "/" separators.
func splitRepeatChildren(inner string) []string {
	var children []string
	for _, part := range strings.Split(inner, "/") {
		part = strings.TrimSpace(part)
		part = strings.TrimPrefix(part, "-")
		part = strings.TrimSpace(part)
		if part != "" {
			children = append(children, part)
		}
	}
	return children
}

// validateStepNameOrder enforces name-before-duration. A step that
// leads with a duration token followed by a name word silently shows "Go" on the
// watch instead of the label. "3m Leg press" is rejected; "Leg press 3m" passes;
// "4m 105%" passes (105% is intensity, not a name).
func validateStepNameOrder(step string) error {
	fields := strings.Fields(step)
	if len(fields) == 0 {
		return nil
	}
	if !durationToken.MatchString(strings.ToLower(fields[0])) {
		return nil
	}
	for _, w := range fields[1:] {
		if !isIntensityToken(w) && !durationToken.MatchString(strings.ToLower(w)) {
			return fmt.Errorf("step %q leads with a duration before its name; put the label first, e.g. %q", step, reorderNameFirst(fields))
		}
	}
	return nil
}

// reorderNameFirst moves a leading duration token after the rest of the step, to
// suggest a corrected form in the error message.
func reorderNameFirst(fields []string) string {
	if len(fields) < 2 {
		return strings.Join(fields, " ")
	}
	return strings.Join(append(append([]string{}, fields[1:]...), fields[0]), " ")
}

// isIntensityToken reports whether a word is an intensity/target token rather
// than part of an exercise name: percentages, watts, HR zones, LTHR, etc.
func isIntensityToken(word string) bool {
	w := strings.TrimSuffix(word, "%")
	if w != word { // had a trailing %
		return true
	}
	upper := strings.ToUpper(word)
	switch upper {
	case "HR", "LTHR", "FTP", "MAX", "RAMP", "FREE":
		return true
	}
	if zoneToken.MatchString(strings.ToLower(word)) {
		return true
	}
	if wattOrPercentToken.MatchString(word) {
		return true
	}
	return false
}

// validateNoCombinedExercise rejects steps that merge two exercises into one,
// which produce the wrong name or dropped sets on the watch.
func validateNoCombinedExercise(step string) error {
	if strings.Contains(step, " + ") || strings.Contains(step, " & ") {
		return fmt.Errorf("step %q combines two exercises; emit one exercise per step", step)
	}
	return nil
}

// validateSwimStep enforces the distance-based swim format.
// Pace-based steps drop their distance and show junk durations on the watch.
func validateSwimStep(step string) error {
	if paceToken.MatchString(step) {
		return fmt.Errorf("swim step %q is pace-based; use the distance form, e.g. \"500mtr Continuous\"", step)
	}
	if !distanceToken.MatchString(step) {
		return fmt.Errorf("swim step %q has no distance; use the distance form, e.g. \"500mtr Continuous\"", step)
	}
	return nil
}

// validateHRIntensity rejects malformed HR intensity strings. HR
// targets are order-dependent: "Z2 HR", "70-81% HR", "75-85% LTHR" parse;
// "hr Z2" and bare "130-150bpm" do not and leave no target on the watch.
func validateHRIntensity(step string) error {
	lower := strings.ToLower(step)
	if bpmValue.MatchString(lower) {
		return fmt.Errorf("HR step %q uses bpm; use a zone or %% with an HR suffix, e.g. \"Z2 HR\" or \"70-81%% HR\"", step)
	}
	if hrBeforeValue.MatchString(lower) {
		return fmt.Errorf("HR step %q puts HR before the value; use value-first, e.g. \"Z2 HR\" or \"70-81%% HR\"", step)
	}
	return nil
}
