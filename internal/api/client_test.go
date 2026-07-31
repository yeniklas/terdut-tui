package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// call records what the client actually put on the wire. The paths and methods
// are the contract with terdut-server, and getting one wrong is exactly how this
// client broke when the server split alerts from incidents.
type call struct {
	method string
	path   string
	query  string
	body   string
	auth   string
}

// stub serves one canned response and records the request that fetched it.
func stub(t *testing.T, status int, response string) (*Client, *call) {
	t.Helper()
	got := &call{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got.method, got.path, got.query = r.Method, r.URL.Path, r.URL.RawQuery
		got.body, got.auth = string(body), r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, response)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "test-key"), got
}

func TestClient_SendsBearerToken(t *testing.T) {
	c, got := stub(t, http.StatusOK, `[]`)
	if _, err := c.ListIncidents("", false, false, 0); err != nil {
		t.Fatalf("list: %v", err)
	}
	if got.auth != "Bearer test-key" {
		t.Errorf("expected bearer token, got %q", got.auth)
	}
}

// Every incident action, with the method and path terdut-server exposes.
func TestClient_IncidentEndpoints(t *testing.T) {
	tests := []struct {
		name   string
		invoke func(*Client) error
		method string
		path   string
		// resp defaults to a JSON object; endpoints returning a list need an array.
		resp string
	}{
		{"get", func(c *Client) error { _, err := c.GetIncident(7); return err },
			http.MethodGet, "/api/incidents/7", ""},
		{"timeline", func(c *Client) error { _, err := c.GetIncidentTimeline(7); return err },
			http.MethodGet, "/api/incidents/7/timeline", `[]`},
		{"acknowledge", func(c *Client) error { _, err := c.AcknowledgeIncident(7); return err },
			http.MethodPost, "/api/incidents/7/acknowledge", ""},
		{"unacknowledge", func(c *Client) error { return c.UnacknowledgeIncident(7) },
			http.MethodDelete, "/api/incidents/7/acknowledge", ""},
		{"resolve", func(c *Client) error { _, err := c.ResolveIncident(7); return err },
			http.MethodPost, "/api/incidents/7/resolve", ""},
		{"assign", func(c *Client) error { _, err := c.AssignIncident(7, 3); return err },
			http.MethodPost, "/api/incidents/7/assign", ""},
		{"snooze", func(c *Client) error { _, err := c.SnoozeIncident(7, "2h"); return err },
			http.MethodPost, "/api/incidents/7/snooze", ""},
		{"unsnooze", func(c *Client) error { return c.UnsnoozeIncident(7) },
			http.MethodDelete, "/api/incidents/7/snooze", ""},
		{"archive", func(c *Client) error { _, err := c.ArchiveIncident(7); return err },
			http.MethodPost, "/api/incidents/7/archive", ""},
		{"unarchive", func(c *Client) error { return c.UnarchiveIncident(7) },
			http.MethodDelete, "/api/incidents/7/archive", ""},
		{"add note", func(c *Client) error { _, err := c.AddNote(7, "hi"); return err },
			http.MethodPost, "/api/incidents/7/notes", ""},
		{"delete note", func(c *Client) error { return c.DeleteNote(7, 12) },
			http.MethodDelete, "/api/incidents/7/notes/12", ""},
		{"stats", func(c *Client) error { _, err := c.GetIncidentStats(); return err },
			http.MethodGet, "/api/stats/incidents", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.resp
			if resp == "" {
				resp = `{}`
			}
			c, got := stub(t, http.StatusOK, resp)
			if err := tt.invoke(c); err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			if got.method != tt.method || got.path != tt.path {
				t.Errorf("expected %s %s, got %s %s", tt.method, tt.path, got.method, got.path)
			}
		})
	}
}

func TestListIncidents_Filters(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		archived bool
		snoozed  bool
		limit    int
		want     string
	}{
		{"default is the open queue", "", false, false, 0, ""},
		{"status", "triggered", false, false, 0, "status=triggered"},
		{"archived", "resolved", true, false, 0, "archived=true&status=resolved"},
		{"snoozed", "", false, true, 0, "snoozed=true"},
		{"limit", "", false, false, 500, "limit=500"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, got := stub(t, http.StatusOK, `[]`)
			if _, err := c.ListIncidents(tt.status, tt.archived, tt.snoozed, tt.limit); err != nil {
				t.Fatalf("list: %v", err)
			}
			if got.query != tt.want {
				t.Errorf("expected query %q, got %q", tt.want, got.query)
			}
		})
	}
}

func TestClient_RequestBodies(t *testing.T) {
	t.Run("assign", func(t *testing.T) {
		c, got := stub(t, http.StatusOK, `{}`)
		if _, err := c.AssignIncident(1, 42); err != nil {
			t.Fatalf("assign: %v", err)
		}
		var body struct {
			UserID int64 `json:"user_id"`
		}
		if err := json.Unmarshal([]byte(got.body), &body); err != nil {
			t.Fatalf("decode body %q: %v", got.body, err)
		}
		if body.UserID != 42 {
			t.Errorf("expected user_id 42, got %d", body.UserID)
		}
	})

	t.Run("snooze", func(t *testing.T) {
		c, got := stub(t, http.StatusOK, `{}`)
		if _, err := c.SnoozeIncident(1, "90m"); err != nil {
			t.Fatalf("snooze: %v", err)
		}
		var body struct {
			Duration string `json:"duration"`
		}
		if err := json.Unmarshal([]byte(got.body), &body); err != nil {
			t.Fatalf("decode body %q: %v", got.body, err)
		}
		if body.Duration != "90m" {
			t.Errorf("expected duration 90m, got %q", body.Duration)
		}
	})
}

// The 409 on re-resolving is the server telling the user why nothing happened,
// so the message has to survive into the error the TUI displays.
func TestClient_SurfacesServerErrorMessage(t *testing.T) {
	c, _ := stub(t, http.StatusConflict, `{"error":"incident is resolved"}`)
	_, err := c.ResolveIncident(1)
	if err == nil {
		t.Fatal("expected an error on 409")
	}
	if !strings.Contains(err.Error(), "incident is resolved") || !strings.Contains(err.Error(), "409") {
		t.Errorf("expected status and server message in %q", err.Error())
	}
}

func TestClient_ErrorWithoutBody(t *testing.T) {
	c, _ := stub(t, http.StatusInternalServerError, ``)
	if _, err := c.GetIncident(1); err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("expected a 500 error, got %v", err)
	}
}

// Nobody on call is a normal state, not a failure.
func TestGetCurrentOnCall_404IsNotAnError(t *testing.T) {
	c, _ := stub(t, http.StatusNotFound, `{"error":"no one is on call today"}`)
	entry, err := c.GetCurrentOnCall()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if entry != nil {
		t.Errorf("expected nil entry, got %+v", entry)
	}
}

// Optional fields are omitted by the server rather than sent null, so decoding
// has to leave them zero instead of failing.
func TestIncident_DecodesSparseServerShape(t *testing.T) {
	c, _ := stub(t, http.StatusOK, `{
		"id": 1,
		"group_key": "{}:{alertname=\"DiskFull\"}",
		"title": "DiskFull (namespace=prod)",
		"group_labels": {"alertname": "DiskFull", "namespace": "prod"},
		"status": "triggered",
		"severity": "critical",
		"triggered_at": "2026-07-30T10:00:00Z"
	}`)

	inc, err := c.GetIncident(1)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if inc.Title != "DiskFull (namespace=prod)" || inc.Severity != "critical" {
		t.Errorf("unexpected incident %+v", inc)
	}
	if inc.GroupLabels["namespace"] != "prod" {
		t.Errorf("expected group labels decoded, got %v", inc.GroupLabels)
	}
	if !inc.IsOpen() {
		t.Error("an incident with no resolved_at is open")
	}
	if inc.IsSnoozed() {
		t.Error("an incident with no snoozed_until is not snoozed")
	}
	if inc.AcknowledgedByID != nil || inc.AssignedToID != nil {
		t.Error("expected acknowledgement and assignment to be absent")
	}
}

// A snooze expires by falling into the past; the server sweeps nothing, so the
// client is what decides a stale snooze no longer counts.
func TestIncident_IsSnoozed(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)

	tests := []struct {
		name  string
		until *time.Time
		want  bool
	}{
		{"never snoozed", nil, false},
		{"snooze in the past has expired", &past, false},
		{"snooze in the future holds", &future, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (Incident{SnoozedUntil: tt.until}).IsSnoozed(); got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestIncident_IsOpen(t *testing.T) {
	now := time.Now()
	if !(Incident{}).IsOpen() {
		t.Error("no resolved_at means open")
	}
	if (Incident{ResolvedAt: &now}).IsOpen() {
		t.Error("resolved_at means closed")
	}
}

func TestAlert_DecodesIncidentLink(t *testing.T) {
	c, _ := stub(t, http.StatusOK, `{"id":3,"name":"DiskFull","status":"firing","incident_id":7}`)
	a, err := c.GetAlert(3)
	if err != nil {
		t.Fatalf("get alert: %v", err)
	}
	if a.IncidentID == nil || *a.IncidentID != 7 {
		t.Errorf("expected incident_id 7, got %v", a.IncidentID)
	}
}

func TestListAlerts_ArchivedFilter(t *testing.T) {
	c, got := stub(t, http.StatusOK, `[]`)
	if _, err := c.ListAlerts("", true, 50); err != nil {
		t.Fatalf("list alerts: %v", err)
	}
	if got.path != "/api/alerts" || got.query != "archived=true&limit=50" {
		t.Errorf("unexpected request %s?%s", got.path, got.query)
	}
}

// MTTA and MTTR are null until something has been acknowledged or resolved. That
// is "no data", and it must not decode to a confident zero.
func TestIncidentStats_NullAveragesStayNil(t *testing.T) {
	c, _ := stub(t, http.StatusOK,
		`{"total":2,"triggered":2,"acknowledged":0,"resolved":0,"mtta_seconds":null,"mttr_seconds":null}`)
	stats, err := c.GetIncidentStats()
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.Total != 2 || stats.Triggered != 2 {
		t.Errorf("unexpected counts %+v", stats)
	}
	if stats.MTTASeconds != nil || stats.MTTRSeconds != nil {
		t.Errorf("expected nil averages, got %v / %v", stats.MTTASeconds, stats.MTTRSeconds)
	}
}
