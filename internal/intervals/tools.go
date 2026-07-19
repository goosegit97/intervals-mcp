package intervals

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	mcputil "github.com/goosegit97/intervals-mcp/internal/mcp"
)

const (
	serverName    = "intervals"
	serverVersion = "0.1.0"
)

// --- Tool input types ---------------------------------------------------------

// GetActivitiesInput is the input for get_activities.
type GetActivitiesInput struct {
	Oldest string          `json:"oldest" jsonschema:"Start date (inclusive), local ISO-8601 day, e.g. 2026-06-01. Required."`
	Newest string          `json:"newest,omitempty" jsonschema:"End date (inclusive), local ISO-8601 day. Defaults to today if omitted."`
	Limit  mcputil.FlexInt `json:"limit,omitempty" jsonschema:"Optional maximum number of activities to return."`
}

// GetActivityDetailInput is the input for get_activity_detail.
type GetActivityDetailInput struct {
	ActivityID string           `json:"activity_id" jsonschema:"The Intervals.icu activity id, e.g. i1234567."`
	Intervals  mcputil.FlexBool `json:"intervals,omitempty" jsonschema:"When true, include the interval/lap breakdown."`
}

// GetWellnessInput is the input for get_wellness.
type GetWellnessInput struct {
	Oldest string `json:"oldest,omitempty" jsonschema:"Start date (inclusive), local ISO-8601 day, e.g. 2026-06-01."`
	Newest string `json:"newest,omitempty" jsonschema:"End date (inclusive), local ISO-8601 day."`
}

// GetAthleteProfileInput is the (empty) input for get_athlete_profile.
type GetAthleteProfileInput struct{}

// GetEventsInput is the input for get_events.
type GetEventsInput struct {
	Oldest   string          `json:"oldest,omitempty" jsonschema:"Start date (inclusive), local ISO-8601 day, e.g. 2026-06-01."`
	Newest   string          `json:"newest,omitempty" jsonschema:"End date (inclusive), local ISO-8601 day."`
	Category string          `json:"category,omitempty" jsonschema:"Optional event category filter, e.g. WORKOUT or NOTE."`
	Limit    mcputil.FlexInt `json:"limit,omitempty" jsonschema:"Optional maximum number of events to return."`
}

// --- Tool handlers ------------------------------------------------------------

// gateReason explains why a confirm/dry_run-gated write was not applied.
func gateReason(confirmHint string, dryRun bool) string {
	if dryRun {
		return "dry_run"
	}
	return confirmHint
}

func (s *Service) handleGetActivities(ctx context.Context, _ *mcp.CallToolRequest, args GetActivitiesInput) (*mcp.CallToolResult, any, error) {
	query := map[string]string{"oldest": args.Oldest}
	if args.Newest != "" {
		query["newest"] = args.Newest
	}
	if args.Limit > 0 {
		query["limit"] = strconv.Itoa(args.Limit.Int())
	}
	body, err := s.httpGet(ctx, fmt.Sprintf("/athlete/%s/activities", s.athleteID), query)
	if err != nil {
		return nil, nil, err
	}
	return mcputil.JSONResult(body), nil, nil
}

func (s *Service) handleGetActivityDetail(ctx context.Context, _ *mcp.CallToolRequest, args GetActivityDetailInput) (*mcp.CallToolResult, any, error) {
	query := map[string]string{}
	if args.Intervals {
		query["intervals"] = "true"
	}
	body, err := s.httpGet(ctx, fmt.Sprintf("/activity/%s", args.ActivityID), query)
	if err != nil {
		return nil, nil, err
	}
	return mcputil.JSONResult(body), nil, nil
}

func (s *Service) handleGetWellness(ctx context.Context, _ *mcp.CallToolRequest, args GetWellnessInput) (*mcp.CallToolResult, any, error) {
	query := map[string]string{}
	if args.Oldest != "" {
		query["oldest"] = args.Oldest
	}
	if args.Newest != "" {
		query["newest"] = args.Newest
	}
	body, err := s.httpGet(ctx, fmt.Sprintf("/athlete/%s/wellness", s.athleteID), query)
	if err != nil {
		return nil, nil, err
	}
	return mcputil.JSONResult(body), nil, nil
}

func (s *Service) handleGetAthleteProfile(ctx context.Context, _ *mcp.CallToolRequest, _ GetAthleteProfileInput) (*mcp.CallToolResult, any, error) {
	body, err := s.httpGet(ctx, fmt.Sprintf("/athlete/%s", s.athleteID), nil)
	if err != nil {
		return nil, nil, err
	}
	return mcputil.JSONResult(body), nil, nil
}

func (s *Service) handleGetEvents(ctx context.Context, _ *mcp.CallToolRequest, args GetEventsInput) (*mcp.CallToolResult, any, error) {
	query := map[string]string{}
	if args.Oldest != "" {
		query["oldest"] = args.Oldest
	}
	if args.Newest != "" {
		query["newest"] = args.Newest
	}
	if args.Category != "" {
		query["category"] = args.Category
	}
	if args.Limit > 0 {
		query["limit"] = strconv.Itoa(args.Limit.Int())
	}
	body, err := s.httpGet(ctx, fmt.Sprintf("/athlete/%s/events", s.athleteID), query)
	if err != nil {
		return nil, nil, err
	}
	return mcputil.JSONResult(body), nil, nil
}

// --- Write tool input types ---------------------------------------------------
// These honour the BINDING safety contract in the root README section 4:
// single-id only (no bulk/range deletes, never the `others` flag); confirm/dry_run
// gating; read-modify-write updates; idempotent creates via external_id.

// CreateWorkoutInput is the input for create_workout.
type CreateWorkoutInput struct {
	StartDateLocal string           `json:"start_date_local" jsonschema:"Local ISO-8601 datetime, e.g. 2026-12-01T00:00:00. Required."`
	Name           string           `json:"name" jsonschema:"Workout name. Required."`
	Description    string           `json:"description" jsonschema:"Workout steps in the Intervals.icu workout-text language (m = minutes, mtr = metres). Required."`
	Type           string           `json:"type,omitempty" jsonschema:"Activity type, e.g. Ride, Run, Swim. Defaults to Ride."`
	Target         string           `json:"target,omitempty" jsonschema:"Intensity basis: AUTO, POWER, HR or PACE. Defaults to AUTO."`
	ExternalID     string           `json:"external_id,omitempty" jsonschema:"Optional caller id for idempotent retries."`
	DryRun         mcputil.FlexBool `json:"dry_run,omitempty" jsonschema:"If true, return the request that would be sent without sending it."`
}

// UpdateEventInput is the input for update_event. Change fields are pointers so
// "not provided" is distinct from an empty value.
type UpdateEventInput struct {
	EventID        string           `json:"event_id" jsonschema:"The event id to update. Required."`
	Name           *string          `json:"name,omitempty" jsonschema:"New name, if changing."`
	Description    *string          `json:"description,omitempty" jsonschema:"New description/workout-text, if changing."`
	StartDateLocal *string          `json:"start_date_local,omitempty" jsonschema:"New local ISO-8601 datetime, if changing."`
	Type           *string          `json:"type,omitempty" jsonschema:"New activity type, if changing."`
	Target         *string          `json:"target,omitempty" jsonschema:"New intensity basis (AUTO/POWER/HR/PACE), if changing."`
	Confirm        mcputil.FlexBool `json:"confirm,omitempty" jsonschema:"Must be true to actually apply the update."`
	DryRun         mcputil.FlexBool `json:"dry_run,omitempty" jsonschema:"If true, return the merged body that would be sent without sending."`
}

// DeleteEventInput is the input for delete_event.
type DeleteEventInput struct {
	EventID string           `json:"event_id" jsonschema:"The event id to delete. Required."`
	Confirm mcputil.FlexBool `json:"confirm,omitempty" jsonschema:"Must be true to actually delete."`
	DryRun  mcputil.FlexBool `json:"dry_run,omitempty" jsonschema:"If true, return the event that would be deleted without deleting."`
}

// --- Write tool handlers ------------------------------------------------------

func (s *Service) handleCreateWorkout(ctx context.Context, _ *mcp.CallToolRequest, args CreateWorkoutInput) (*mcp.CallToolResult, any, error) {
	workoutType := args.Type
	if workoutType == "" {
		workoutType = "Ride"
	}
	if isStrengthType(workoutType) {
		return nil, nil, fmt.Errorf("strength workouts with named exercises and reps cannot be created via the intervals service (they render as generic \"Go\" steps on the watch) -- use the garmin service's create_strength_workout tool instead")
	}
	target := args.Target
	if target == "" {
		target = "AUTO"
	}
	description, err := ValidateWorkout(workoutType, target, args.Description)
	if err != nil {
		return nil, nil, err
	}
	payload := map[string]any{
		"category":         "WORKOUT",
		"start_date_local": args.StartDateLocal,
		"type":             workoutType,
		"name":             args.Name,
		"description":      description,
		"target":           target,
	}
	if args.ExternalID != "" {
		payload["external_id"] = args.ExternalID
	}

	path := fmt.Sprintf("/athlete/%s/events", s.athleteID)
	if args.DryRun {
		return mcputil.DataResult(map[string]any{"dry_run": true, "method": "POST", "path": path, "body": payload})
	}
	body, err := s.httpRequest(ctx, http.MethodPost, path, map[string]string{"upsertOnUid": "false"}, payload)
	if err != nil {
		return nil, nil, err
	}
	return mcputil.JSONResult(body), nil, nil
}

func (s *Service) handleUpdateEvent(ctx context.Context, _ *mcp.CallToolRequest, args UpdateEventInput) (*mcp.CallToolResult, any, error) {
	changes := map[string]any{}
	if args.Name != nil {
		changes["name"] = *args.Name
	}
	if args.Description != nil {
		changes["description"] = *args.Description
	}
	if args.StartDateLocal != nil {
		changes["start_date_local"] = *args.StartDateLocal
	}
	if args.Type != nil {
		changes["type"] = *args.Type
	}
	if args.Target != nil {
		changes["target"] = *args.Target
	}
	if len(changes) == 0 {
		return nil, nil, fmt.Errorf("no fields to update were provided")
	}

	if args.Type != nil && isStrengthType(*args.Type) {
		return nil, nil, fmt.Errorf("strength workouts cannot be created via the intervals service -- use the garmin service's create_strength_workout tool instead")
	}

	path := fmt.Sprintf("/athlete/%s/events/%s", s.athleteID, args.EventID)
	currentRaw, err := s.httpGet(ctx, path, nil)
	if err != nil {
		return nil, nil, err
	}

	// Validate any new description against the effective type/target (falling
	// back to the event's current values for fields not being changed).
	if args.Description != nil {
		var current map[string]any
		if err := json.Unmarshal(currentRaw, &current); err != nil {
			return nil, nil, fmt.Errorf("decoding current event: %w", err)
		}
		effType := stringField(current, "type")
		if args.Type != nil {
			effType = *args.Type
		}
		effTarget := stringField(current, "target")
		if args.Target != nil {
			effTarget = *args.Target
		}
		expanded, err := ValidateWorkout(effType, effTarget, *args.Description)
		if err != nil {
			return nil, nil, err
		}
		changes["description"] = expanded
	}

	if args.DryRun || !args.Confirm {
		return mcputil.DataResult(map[string]any{
			"applied":  false,
			"reason":   gateReason("confirm is false; pass confirm=true to apply", args.DryRun.Bool()),
			"event_id": args.EventID,
			"current":  eventSummary(currentRaw),
			"changes":  changes,
		})
	}

	var merged map[string]any
	if err := json.Unmarshal(currentRaw, &merged); err != nil {
		return nil, nil, fmt.Errorf("decoding current event: %w", err)
	}
	for key, value := range changes {
		merged[key] = value
	}
	if _, err := s.httpRequest(ctx, http.MethodPut, path, nil, merged); err != nil {
		return nil, nil, err
	}

	// Re-fetch and assert: the PUT response can report success while the parsed
	// server-side result differs from the intent (e.g. step names not capturing,
	// intensity targets dropped). Surface any mismatch instead of trusting the
	// write response alone.
	verifyRaw, err := s.httpGet(ctx, path, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("update applied but re-fetch failed: %w", err)
	}
	var after map[string]any
	if err := json.Unmarshal(verifyRaw, &after); err != nil {
		return nil, nil, fmt.Errorf("update applied but decoding re-fetch failed: %w", err)
	}
	mismatches := map[string]any{}
	for key, want := range changes {
		if got := after[key]; fmt.Sprint(got) != fmt.Sprint(want) {
			mismatches[key] = map[string]any{"intended": want, "actual": got}
		}
	}
	return mcputil.DataResult(map[string]any{
		"applied":    true,
		"event_id":   args.EventID,
		"verified":   len(mismatches) == 0,
		"mismatches": mismatches,
		"event":      eventSummary(verifyRaw),
	})
}

func (s *Service) handleDeleteEvent(ctx context.Context, _ *mcp.CallToolRequest, args DeleteEventInput) (*mcp.CallToolResult, any, error) {
	path := fmt.Sprintf("/athlete/%s/events/%s", s.athleteID, args.EventID)
	currentRaw, err := s.httpGet(ctx, path, nil)
	if err != nil {
		return nil, nil, err
	}

	if args.DryRun || !args.Confirm {
		return mcputil.DataResult(map[string]any{"deleted": false, "reason": gateReason("confirm is false; pass confirm=true to delete", args.DryRun.Bool()), "event": eventSummary(currentRaw)})
	}

	if _, err := s.httpRequest(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return nil, nil, err
	}
	return mcputil.DataResult(map[string]any{"deleted": true, "event_id": args.EventID, "event": eventSummary(currentRaw)})
}

// NewServer builds the intervals MCP server with all tools registered as
// handlers over the given Service.
func NewServer(svc *Service) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: serverName, Version: serverVersion}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_activities",
		InputSchema: mcputil.InputSchema[GetActivitiesInput]("get_activities", "limit"),
		Description: "List activities for a date range, newest first. Returns activity summary " +
			"objects (id, start_date_local, type, name, distance, moving_time, icu_training_load, " +
			"icu_ftp, etc.) in descending date order.",
	}, svc.handleGetActivities)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_activity_detail",
		InputSchema: mcputil.InputSchema[GetActivityDetailInput]("get_activity_detail", "intervals"),
		Description: "Get full detail for a single activity by id. Set intervals=true to include " +
			"the interval/lap breakdown.",
	}, svc.handleGetActivityDetail)

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_wellness",
		Description: "List daily wellness records for a date range (id is the date, plus weight, " +
			"restingHR, hrv, hrvSDNN, sleepSecs, sleepScore, sleepQuality, ctl, atl, soreness, " +
			"fatigue, stress, mood, etc.).",
	}, svc.handleGetWellness)

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_athlete_profile",
		Description: "Get the athlete profile including per-sport settings (sportSettings: ftp, " +
			"indoor_ftp, w_prime, p_max, power_zones, lthr, max_hr, hr_zones, threshold_pace, " +
			"pace_zones, pace_units).",
	}, svc.handleGetAthleteProfile)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_events",
		InputSchema: mcputil.InputSchema[GetEventsInput]("get_events", "limit"),
		Description: "List calendar events (planned workouts, notes, races) for a date range " +
			"(id, start_date_local, end_date_local, category, type, name, description, " +
			"icu_training_load, calendar_id, etc.).",
	}, svc.handleGetEvents)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_workout",
		InputSchema: mcputil.InputSchema[CreateWorkoutInput]("create_workout", "dry_run"),
		Description: "Create one planned workout on the calendar. Put the workout steps in " +
			"`description` using the Intervals.icu workout-text language (e.g. '- Warmup 10m " +
			"60%', '3x', '- 4m 105%', '- 2m 50%'); the server parses it. In that language 'm' " +
			"means minutes -- use 'mtr' for metres. Supports dry_run and external_id (idempotency). " +
			"NOTE: dry_run echoes the raw POST body only -- server-side parsing (step names, " +
			"intensity targets, repeat expansion) is NOT validated by a dry run. To confirm the " +
			"parsed structure, create a real event on a throwaway future date and re-fetch it. " +
			"NOTE: step labels appear in the Garmin Connect workout overview, but during live " +
			"execution the watch shows the step's intensity type (e.g. 'Go' for active, 'Rest' " +
			"for rest) as the primary prompt -- for named exercises with rep targets on the watch " +
			"screen, use the garmin service instead.",
	}, svc.handleCreateWorkout)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_event",
		InputSchema: mcputil.InputSchema[UpdateEventInput]("update_event", "confirm", "dry_run"),
		Description: "Update one calendar event by id (read-modify-write: fetches the event, " +
			"applies only the fields you pass, writes the merged object back -- never blanking " +
			"unspecified fields). Only applies with confirm=true; otherwise returns a preview. " +
			"NOTE: step labels appear in the Garmin Connect workout overview, but during live " +
			"execution the watch shows the step's intensity type (e.g. 'Go' for active, 'Rest' " +
			"for rest) as the primary prompt -- for named exercises with rep targets on the watch " +
			"screen, use the garmin service instead. After updating a workout, re-fetch the event " +
			"and confirm step count, names, and intensity targets match your intent (the update " +
			"response alone can report success while the parsed result differs).",
	}, svc.handleUpdateEvent)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_event",
		InputSchema: mcputil.InputSchema[DeleteEventInput]("delete_event", "confirm", "dry_run"),
		Description: "Delete one calendar event by id. IRREVERSIBLE. Fetches and returns the " +
			"event for confirmation; only deletes with confirm=true. Only ever deletes the single " +
			"event you name -- never sibling or range events.",
	}, svc.handleDeleteEvent)

	return server
}
