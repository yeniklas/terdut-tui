package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yeniklas/terdut-tui/internal/api"
)

// ansi matches the escape sequences lipgloss emits when it decides the output
// supports colour, so assertions can be made against the text alone.
var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(s string) string { return ansi.ReplaceAllString(s, "") }

func mustContain(t *testing.T, got string, wants ...string) {
	t.Helper()
	got = plain(got)
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("expected output to contain %q\n--- got ---\n%s", w, got)
		}
	}
}

func TestIncidentDetail_RendersTheWholeStory(t *testing.T) {
	now := time.Now()
	ackID := int64(1)
	alertID := int64(3)
	inc := api.Incident{
		ID: 1, Title: "DiskFull (namespace=prod)", Status: api.StatusAcknowledged,
		Severity:    "critical",
		GroupLabels: map[string]string{"alertname": "DiskFull", "namespace": "prod"},
		TriggeredAt: now.Add(-2 * time.Hour),
		AssignedTo:  "admin", AcknowledgedByID: &ackID, AcknowledgedBy: "admin",
		AcknowledgedAt: &now,
		Alerts: []api.Alert{
			{ID: 3, Name: "DiskFull", Status: "firing",
				Labels: map[string]string{"instance": "node-1"}, ReceivedAt: now},
			{ID: 4, Name: "DiskFull", Status: "resolved",
				Labels: map[string]string{"instance": "node-2"}, ReceivedAt: now},
		},
	}
	timeline := []api.IncidentEvent{
		{Type: api.EventTriggered, CreatedAt: now},
		{Type: api.EventAssigned, Username: "admin", CreatedAt: now},
		{Type: api.EventAlertAdded, AlertID: &alertID, CreatedAt: now},
		{Type: api.EventAcknowledged, Username: "admin", CreatedAt: now},
		{Type: api.EventNote, Username: "admin", Detail: "draining node-2", CreatedAt: now},
	}

	out := buildIncidentDetailContent(inc, timeline, -1, 110)
	mustContain(t, out,
		"DiskFull (namespace=prod)", "ACKNOWLEDGED", "CRITICAL",
		"Assigned:", "admin",
		"Grouped By", "namespace", "prod",
		"Alerts (2)", "node-1", "node-2",
		"Timeline (5 events, 1 notes)",
		"Incident opened", "Assigned to admin", "Alert #3 joined", "Acknowledged by admin",
		"admin wrote", "draining node-2",
	)
}

func TestIncidentDetail_ShowsSnooze(t *testing.T) {
	future := time.Now().Add(2 * time.Hour)
	inc := api.Incident{
		Title: "Noisy", Status: api.StatusTriggered,
		TriggeredAt: time.Now(), SnoozedUntil: &future,
	}
	// The exact remaining time is humanUntil's business, not this test's — a few
	// microseconds of elapsed clock turn "in 2h" into "in 1h 59m".
	mustContain(t, buildIncidentDetailContent(inc, nil, -1, 110),
		"TRIGGERED (snoozed)", "Snoozed:", "until", "in 1h")
}

// An expired snooze is not a snooze, so it must not be reported as one.
func TestIncidentDetail_HidesExpiredSnooze(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	inc := api.Incident{
		Title: "Noisy", Status: api.StatusTriggered,
		TriggeredAt: time.Now(), SnoozedUntil: &past,
	}
	if strings.Contains(plain(buildIncidentDetailContent(inc, nil, -1, 110)), "Snoozed:") {
		t.Error("an expired snooze should not be rendered")
	}
}

func TestIncidentDetail_ShowsResolutionSource(t *testing.T) {
	now := time.Now()
	source := "manual"
	inc := api.Incident{
		Title: "Done", Status: api.StatusResolved, TriggeredAt: now.Add(-time.Hour),
		ResolvedAt: &now, ResolutionSource: &source,
	}
	mustContain(t, buildIncidentDetailContent(inc, nil, -1, 110),
		"RESOLVED", "Resolved:", "manual")
}

func TestIncidentDetail_UnassignedAndUnacknowledged(t *testing.T) {
	inc := api.Incident{Title: "Fresh", Status: api.StatusTriggered, TriggeredAt: time.Now()}
	mustContain(t, buildIncidentDetailContent(inc, nil, -1, 110), "nobody", "not acknowledged")
}

func TestIncidentDetail_EmptyTimeline(t *testing.T) {
	inc := api.Incident{Title: "Fresh", Status: api.StatusTriggered, TriggeredAt: time.Now()}
	mustContain(t, buildIncidentDetailContent(inc, nil, -1, 110), "Nothing recorded yet")
}

func TestIncidentDetail_MarksSelectedNote(t *testing.T) {
	now := time.Now()
	timeline := []api.IncidentEvent{
		{Type: api.EventNote, Username: "admin", Detail: "first", CreatedAt: now},
		{Type: api.EventNote, Username: "alice", Detail: "second", CreatedAt: now},
	}
	inc := api.Incident{Title: "X", Status: api.StatusTriggered, TriggeredAt: now}

	out := plain(buildIncidentDetailContent(inc, timeline, 1, 110))
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "alice") && !strings.HasPrefix(line, "> ") {
			t.Errorf("expected the selected note marked, got %q", line)
		}
		if strings.Contains(line, "admin wrote") && strings.HasPrefix(line, "> ") {
			t.Errorf("expected the unselected note unmarked, got %q", line)
		}
	}
}

func TestIncidentStatusLabel(t *testing.T) {
	future := time.Now().Add(time.Hour)
	tests := []struct {
		name string
		inc  api.Incident
		want string
	}{
		{"triggered", api.Incident{Status: api.StatusTriggered}, "● TRIGGERED"},
		{"acknowledged", api.Incident{Status: api.StatusAcknowledged}, "◐ ACKNOWLEDGED"},
		{"resolved", api.Incident{Status: api.StatusResolved}, "✓ RESOLVED"},
		{"snoozed", api.Incident{Status: api.StatusTriggered, SnoozedUntil: &future},
			"● TRIGGERED (snoozed)"},
		// A status this client does not know about still has to render.
		{"unknown", api.Incident{Status: "escalated"}, "ESCALATED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := incidentStatusLabel(tt.inc); got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

// The server may add event types after this client ships. An unknown one must
// still appear on the timeline rather than silently vanishing.
func TestEventLabel_UnknownTypeFallsBackToItsName(t *testing.T) {
	got := eventLabel(api.IncidentEvent{Type: "escalated", Detail: "to sre-oncall"})
	mustContain(t, got, "escalated", "to sre-oncall")
}

func TestEventLabel_KnownTypes(t *testing.T) {
	alertID := int64(9)
	tests := []struct {
		event api.IncidentEvent
		want  string
	}{
		{api.IncidentEvent{Type: api.EventTriggered}, "Incident opened"},
		{api.IncidentEvent{Type: api.EventAlertAdded, AlertID: &alertID}, "Alert #9 joined"},
		{api.IncidentEvent{Type: api.EventAlertResolved, AlertID: &alertID}, "Alert #9 resolved"},
		{api.IncidentEvent{Type: api.EventAcknowledged, Username: "bo"}, "Acknowledged by bo"},
		{api.IncidentEvent{Type: api.EventAssigned, Username: "bo"}, "Assigned to bo"},
		{api.IncidentEvent{Type: api.EventSnoozed, Detail: "2026-08-01T00:00:00Z"},
			"Snoozed until 2026-08-01T00:00:00Z"},
		{api.IncidentEvent{Type: api.EventResolved, Username: "bo"}, "Resolved by bo"},
		// No user means the server closed it via the alert cascade.
		{api.IncidentEvent{Type: api.EventResolved}, "all alerts stopped firing"},
	}
	for _, tt := range tests {
		t.Run(tt.event.Type, func(t *testing.T) {
			mustContain(t, eventLabel(tt.event), tt.want)
		})
	}
}

func TestAlertDetail_SaysItIsReadOnlyAndLinksTheIncident(t *testing.T) {
	now := time.Now()
	id := int64(7)
	alert := api.Alert{
		ID: 3, Name: "DiskFull", Status: "firing", StartsAt: now.Add(-time.Hour),
		ReceivedAt: now, IncidentID: &id,
		Labels:      map[string]string{"instance": "node-1", "severity": "critical"},
		Annotations: map[string]string{"summary": "disk 90%"},
	}
	mustContain(t, buildAlertDetailContent(alert, 110),
		"DiskFull", "FIRING", "Incident:", "#7", "press i to open it",
		"instance", "node-1", "summary", "disk 90%",
		"Alerts are read-only")
}

func TestAlertDetail_NoIncident(t *testing.T) {
	alert := api.Alert{ID: 3, Name: "Orphan", Status: "resolved", ReceivedAt: time.Now()}
	mustContain(t, buildAlertDetailContent(alert, 110), "Incident:", "none")
}

func TestAlertDetail_ShowsResolutionSource(t *testing.T) {
	source := "expiry"
	alert := api.Alert{Name: "Gone", Status: "resolved", ReceivedAt: time.Now(),
		ResolutionSource: &source}
	mustContain(t, buildAlertDetailContent(alert, 110), "RESOLVED", "expiry")
}

// Null MTTA means nothing has been acknowledged, which is a different claim
// from an instant response.
func TestStats_RendersDashForMissingAverages(t *testing.T) {
	stats := &api.IncidentStats{Total: 2, Triggered: 2}
	out := buildStatsContent(stats, nil, nil, nil, 110)
	mustContain(t, out, "Incident Response", "Mean time to acknowledge", "—",
		"nothing has been acknowledged or resolved yet")
}

func TestStats_RendersAverages(t *testing.T) {
	mtta, mttr := 150.0, 3600.0
	stats := &api.IncidentStats{Total: 3, Resolved: 1, MTTASeconds: &mtta, MTTRSeconds: &mttr}
	out := buildStatsContent(stats, []api.TopAlert{{Name: "DiskFull", Count: 4}}, nil, nil, 110)
	mustContain(t, out, "2m", "1h", "Top Alerts", "DiskFull")
}

func TestStats_HandlesNoIncidentData(t *testing.T) {
	mustContain(t, buildStatsContent(nil, nil, nil, nil, 110), "Incident Response", "No data")
}

func TestView_TabsAndDashboardRender(t *testing.T) {
	m := sized()
	m.incidents = []api.Incident{{
		ID: 1, Title: "DiskFull", Status: api.StatusTriggered, Severity: "critical",
		AssignedTo: "admin", TriggeredAt: time.Now(),
	}}
	m.incidentStats = &api.IncidentStats{Triggered: 1}
	m.rebuildIncidentTable()

	mustContain(t, m.View(),
		"Incidents", "Alerts", "Stats", "Archived", "Schedule", "Users",
		"Triggered: 1", "filter: open",
		"DiskFull", "critical", "admin",
		"enter·detail")
}

func TestView_EmptyStates(t *testing.T) {
	m := sized()
	m.loading = false
	mustContain(t, m.View(), "No open incidents.")

	m.activeSection = sectionArchived
	mustContain(t, m.View(), "No archived incidents.")
}

// The stats page renders inside the normal section chrome now, so it has to
// survive the real path: a window size message sizes the viewport and fills it.
func TestView_StatsSectionRendersInPlace(t *testing.T) {
	m := NewModel(nil, "http://test", time.Minute)
	m.connected = true
	m.incidentStats = &api.IncidentStats{Total: 3, Triggered: 1}
	m.topAlerts = []api.TopAlert{{Name: "DiskFull", Count: 4}}
	m.statsLoaded = true

	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = next.(Model)
	m.activeSection = sectionStats

	out := m.View()
	mustContain(t, out, "Stats", "Incident Response", "Top Alerts", "DiskFull",
		"tab·section")
	if strings.Contains(plain(out), "Loading statistics") {
		t.Error("loaded stats should not show the loading placeholder")
	}
}

func TestView_ConnectionError(t *testing.T) {
	m := sized()
	m.connected = false
	m.err = errFixture{}
	mustContain(t, m.View(), "Error:", "Press r to retry")
}

// The footer is the only place the terminal states are explained, so the
// destructive one has to be visible before it is pressed.
func TestFooter_IncidentDetailOffersResolveOnlyWhileOpen(t *testing.T) {
	open := onIncident(openIncidentFixture(), nil)
	mustContain(t, open.renderFooter(), "R·resolve", "z·snooze", "a·ack")

	closed := onIncident(resolvedIncidentFixture(), nil)
	if strings.Contains(plain(closed.renderFooter()), "R·resolve") {
		t.Error("a resolved incident should not offer resolve")
	}
	mustContain(t, closed.renderFooter(), "x·archive", "c·note")
}

func TestView_ZeroWidthRendersNothing(t *testing.T) {
	m := NewModel(nil, "http://test", time.Minute)
	if m.View() != "" {
		t.Error("expected no output before the first window size message")
	}
}

type errFixture struct{}

func (errFixture) Error() string { return "connection refused" }
