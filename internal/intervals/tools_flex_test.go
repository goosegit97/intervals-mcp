package intervals

import (
	"encoding/json"
	"testing"

	mcputil "github.com/goosegit97/intervals-mcp/internal/mcp"
)

// Regression: some MCP clients send non-string arguments (booleans, integers)
// as JSON-encoded strings. The intervals inputs and published schemas must
// tolerate both encodings.

func TestInputsTolerateStringifiedScalars(t *testing.T) {
	var del DeleteEventInput
	if err := json.Unmarshal([]byte(`{"event_id":"123","confirm":"true","dry_run":"false"}`), &del); err != nil {
		t.Fatalf("DeleteEventInput stringified: %v", err)
	}
	if !del.Confirm.Bool() || del.DryRun.Bool() {
		t.Errorf("DeleteEventInput = confirm %v, dry_run %v; want true, false", del.Confirm, del.DryRun)
	}

	var upd UpdateEventInput
	if err := json.Unmarshal([]byte(`{"event_id":"123","confirm":true,"dry_run":"true"}`), &upd); err != nil {
		t.Fatalf("UpdateEventInput mixed encodings: %v", err)
	}
	if !upd.Confirm.Bool() || !upd.DryRun.Bool() {
		t.Errorf("UpdateEventInput = confirm %v, dry_run %v; want true, true", upd.Confirm, upd.DryRun)
	}

	var acts GetActivitiesInput
	if err := json.Unmarshal([]byte(`{"oldest":"2026-06-01","limit":"25"}`), &acts); err != nil {
		t.Fatalf("GetActivitiesInput stringified limit: %v", err)
	}
	if acts.Limit.Int() != 25 {
		t.Errorf("GetActivitiesInput.Limit = %d, want 25", acts.Limit.Int())
	}

	var detail GetActivityDetailInput
	if err := json.Unmarshal([]byte(`{"activity_id":"i1","intervals":"true"}`), &detail); err != nil {
		t.Fatalf("GetActivityDetailInput stringified intervals: %v", err)
	}
	if !detail.Intervals.Bool() {
		t.Error("GetActivityDetailInput.Intervals = false, want true")
	}
}

func TestInputSchemaWidensStringifiedScalars(t *testing.T) {
	schema := mcputil.InputSchema[DeleteEventInput]("delete_event", "confirm", "dry_run")
	for _, prop := range []string{"confirm", "dry_run"} {
		p := schema.Properties[prop]
		if p == nil {
			t.Fatalf("property %q missing", prop)
		}
		want := []string{"boolean", "string", "null"}
		if p.Type != "" || len(p.Types) != 3 || p.Types[0] != want[0] || p.Types[1] != want[1] || p.Types[2] != want[2] {
			t.Errorf("%s: Type=%q Types=%v, want Types=%v", prop, p.Type, p.Types, want)
		}
	}
	if id := schema.Properties["event_id"]; id == nil || id.Type != "string" {
		t.Errorf("event_id property changed unexpectedly: %+v", id)
	}
}
