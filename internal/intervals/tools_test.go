package intervals

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeAPI is a minimal in-memory Intervals.icu stand-in. Each key is
// "METHOD path"; the value is the JSON body to return. Requests are recorded
// as "METHOD path?query" so tests can assert exactly what was sent.
type fakeAPI struct {
	t        *testing.T
	routes   map[string]string
	requests []string
	bodies   []string
}

func (f *fakeAPI) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.t.Helper()
		if user, pass, ok := r.BasicAuth(); !ok || user != "API_KEY" || pass != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"bad auth"}`))
			return
		}
		recorded := r.Method + " " + r.URL.Path
		if r.URL.RawQuery != "" {
			recorded += "?" + r.URL.RawQuery
		}
		f.requests = append(f.requests, recorded)
		var body strings.Builder
		if r.Body != nil {
			buf := make([]byte, 64*1024)
			for {
				n, err := r.Body.Read(buf)
				body.Write(buf[:n])
				if err != nil {
					break
				}
			}
		}
		f.bodies = append(f.bodies, body.String())
		resp, ok := f.routes[r.Method+" "+r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"no route"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	})
}

// newTestService starts a fake Intervals API and returns a Service pointed at
// it plus the fake for request assertions.
func newTestService(t *testing.T, routes map[string]string) (*Service, *fakeAPI) {
	t.Helper()
	fake := &fakeAPI{t: t, routes: routes}
	ts := httptest.NewServer(fake.handler())
	t.Cleanup(ts.Close)
	return NewServiceWithClient(ts.Client(), ts.URL, "test-key", "i1"), fake
}

// connect wires an in-memory client session to the server for testing.
func connect(t *testing.T, svc *Service) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := NewServer(svc).Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

// callTool invokes a tool over the session and decodes the JSON text content.
func callTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) (map[string]any, *mcp.CallToolResult) {
	t.Helper()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool %s transport error: %v", name, err)
	}
	text := resultText(res)
	var decoded map[string]any
	if !res.IsError && text != "" {
		if err := json.Unmarshal([]byte(text), &decoded); err != nil {
			// Non-object results (e.g. arrays) are fine; leave decoded nil.
			decoded = nil
		}
	}
	return decoded, res
}

func resultText(res *mcp.CallToolResult) string {
	text := ""
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	return text
}

func TestListTools(t *testing.T) {
	svc, _ := newTestService(t, nil)
	session := connect(t, svc)
	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	got := make([]string, 0, len(res.Tools))
	for _, tool := range res.Tools {
		got = append(got, tool.Name)
	}
	sort.Strings(got)

	want := []string{
		"create_workout", "delete_event", "get_activities", "get_activity_detail",
		"get_athlete_profile", "get_events", "get_wellness", "update_event",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("tools mismatch:\n got=%v\nwant=%v", got, want)
	}
}

func TestGetActivitiesRequiresOldest(t *testing.T) {
	svc, _ := newTestService(t, nil)
	session := connect(t, svc)
	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tool := range res.Tools {
		if tool.Name != "get_activities" {
			continue
		}
		schema, ok := tool.InputSchema.(map[string]any)
		if !ok {
			t.Fatalf("InputSchema is %T, want map[string]any", tool.InputSchema)
		}
		req, _ := schema["required"].([]any)
		if len(req) != 1 || req[0] != "oldest" {
			t.Fatalf("get_activities required = %v, want [oldest]", req)
		}
		return
	}
	t.Fatal("get_activities not found")
}

func TestGetActivitiesRequestShape(t *testing.T) {
	svc, fake := newTestService(t, map[string]string{
		"GET /athlete/i1/activities": `[{"id":"a1"}]`,
	})
	session := connect(t, svc)
	_, res := callTool(t, session, "get_activities", map[string]any{
		"oldest": "2026-06-01", "newest": "2026-06-16", "limit": 5,
	})
	if res.IsError {
		t.Fatalf("tool error: %s", resultText(res))
	}
	want := "GET /athlete/i1/activities?limit=5&newest=2026-06-16&oldest=2026-06-01"
	if len(fake.requests) != 1 || fake.requests[0] != want {
		t.Fatalf("requests = %v, want [%s]", fake.requests, want)
	}
}

// TestBadKeySurfaces401 confirms the auth failure path produces the actionable
// hint (previously verified against the live API).
func TestBadKeySurfaces401(t *testing.T) {
	fake := &fakeAPI{t: t, routes: nil}
	ts := httptest.NewServer(fake.handler())
	t.Cleanup(ts.Close)
	svc := NewServiceWithClient(ts.Client(), ts.URL, "wrong-key", "i1")

	session := connect(t, svc)
	_, res := callTool(t, session, "get_athlete_profile", map[string]any{})
	if !res.IsError {
		t.Fatal("expected a tool error with a bad key")
	}
	text := resultText(res)
	if !strings.Contains(text, "401") || !strings.Contains(text, "authentication failed") {
		t.Fatalf("unexpected error text: %q", text)
	}
}

func TestNewServiceMissingEnv(t *testing.T) {
	t.Setenv("INTERVALS_API_KEY", "")
	t.Setenv("INTERVALS_ATHLETE_ID", "")
	if _, err := NewService(); err == nil {
		t.Fatal("expected an error when env is unset")
	} else if !strings.Contains(err.Error(), "INTERVALS_API_KEY") {
		t.Fatalf("error should name the missing variable: %v", err)
	}
}

func TestCreateWorkoutDryRunSendsNothing(t *testing.T) {
	svc, fake := newTestService(t, nil)
	session := connect(t, svc)
	decoded, res := callTool(t, session, "create_workout", map[string]any{
		"start_date_local": "2030-01-01T00:00:00",
		"name":             "Test ride",
		"description":      "- Warmup 10m 60%",
		"dry_run":          true,
	})
	if res.IsError {
		t.Fatalf("tool error: %s", resultText(res))
	}
	if decoded["dry_run"] != true {
		t.Fatalf("dry_run flag missing from preview: %v", decoded)
	}
	if len(fake.requests) != 0 {
		t.Fatalf("dry_run must not hit the API; requests = %v", fake.requests)
	}
}

func TestCreateWorkoutPostsEvent(t *testing.T) {
	svc, fake := newTestService(t, map[string]string{
		"POST /athlete/i1/events": `{"id":42}`,
	})
	session := connect(t, svc)
	_, res := callTool(t, session, "create_workout", map[string]any{
		"start_date_local": "2030-01-01T00:00:00",
		"name":             "Test ride",
		"description":      "- Warmup 10m 60%",
		"external_id":      "ext-1",
	})
	if res.IsError {
		t.Fatalf("tool error: %s", resultText(res))
	}
	want := "POST /athlete/i1/events?upsertOnUid=false"
	if len(fake.requests) != 1 || fake.requests[0] != want {
		t.Fatalf("requests = %v, want [%s]", fake.requests, want)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(fake.bodies[0]), &payload); err != nil {
		t.Fatalf("decoding POST body: %v", err)
	}
	if payload["category"] != "WORKOUT" || payload["external_id"] != "ext-1" || payload["type"] != "Ride" {
		t.Fatalf("unexpected POST body: %v", payload)
	}
}

func TestUpdateEventGatedWithoutConfirm(t *testing.T) {
	svc, fake := newTestService(t, map[string]string{
		"GET /athlete/i1/events/e1": `{"id":"e1","name":"Old","type":"Ride","target":"AUTO"}`,
	})
	session := connect(t, svc)
	decoded, res := callTool(t, session, "update_event", map[string]any{
		"event_id": "e1", "name": "New name",
	})
	if res.IsError {
		t.Fatalf("tool error: %s", resultText(res))
	}
	if decoded["applied"] != false {
		t.Fatalf("update must not apply without confirm: %v", decoded)
	}
	// Only the preview fetch — never a PUT.
	if len(fake.requests) != 1 || fake.requests[0] != "GET /athlete/i1/events/e1" {
		t.Fatalf("requests = %v, want a single GET", fake.requests)
	}
}

func TestUpdateEventConfirmReadModifyWrite(t *testing.T) {
	svc, fake := newTestService(t, map[string]string{
		"GET /athlete/i1/events/e1": `{"id":"e1","name":"New name","description":"keep me","type":"Ride","target":"AUTO"}`,
		"PUT /athlete/i1/events/e1": `{"id":"e1"}`,
	})
	session := connect(t, svc)
	decoded, res := callTool(t, session, "update_event", map[string]any{
		"event_id": "e1", "name": "New name", "confirm": true,
	})
	if res.IsError {
		t.Fatalf("tool error: %s", resultText(res))
	}
	if decoded["applied"] != true || decoded["verified"] != true {
		t.Fatalf("unexpected result: %v", decoded)
	}
	want := []string{"GET /athlete/i1/events/e1", "PUT /athlete/i1/events/e1", "GET /athlete/i1/events/e1"}
	if strings.Join(fake.requests, ",") != strings.Join(want, ",") {
		t.Fatalf("requests = %v, want %v", fake.requests, want)
	}
	// Read-modify-write: the PUT body must carry unmodified fields too.
	var put map[string]any
	if err := json.Unmarshal([]byte(fake.bodies[1]), &put); err != nil {
		t.Fatalf("decoding PUT body: %v", err)
	}
	if put["description"] != "keep me" || put["name"] != "New name" {
		t.Fatalf("PUT body must merge onto the fetched event: %v", put)
	}
}

func TestDeleteEventGatedWithoutConfirm(t *testing.T) {
	svc, fake := newTestService(t, map[string]string{
		"GET /athlete/i1/events/e1": `{"id":"e1","name":"Doomed"}`,
	})
	session := connect(t, svc)
	decoded, res := callTool(t, session, "delete_event", map[string]any{"event_id": "e1"})
	if res.IsError {
		t.Fatalf("tool error: %s", resultText(res))
	}
	if decoded["deleted"] != false {
		t.Fatalf("delete must not apply without confirm: %v", decoded)
	}
	if len(fake.requests) != 1 || fake.requests[0] != "GET /athlete/i1/events/e1" {
		t.Fatalf("requests = %v, want a single GET", fake.requests)
	}
}

func TestDeleteEventConfirm(t *testing.T) {
	svc, fake := newTestService(t, map[string]string{
		"GET /athlete/i1/events/e1":    `{"id":"e1","name":"Doomed"}`,
		"DELETE /athlete/i1/events/e1": `{}`,
	})
	session := connect(t, svc)
	decoded, res := callTool(t, session, "delete_event", map[string]any{
		"event_id": "e1", "confirm": true,
	})
	if res.IsError {
		t.Fatalf("tool error: %s", resultText(res))
	}
	if decoded["deleted"] != true {
		t.Fatalf("unexpected result: %v", decoded)
	}
	want := []string{"GET /athlete/i1/events/e1", "DELETE /athlete/i1/events/e1"}
	if strings.Join(fake.requests, ",") != strings.Join(want, ",") {
		t.Fatalf("requests = %v, want %v", fake.requests, want)
	}
}
