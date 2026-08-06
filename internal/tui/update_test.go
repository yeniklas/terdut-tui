package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yeniklas/terdut-tui/internal/api"
)

// press sends one key and returns the resulting model and command. A nil command
// means the model decided to do nothing, which is what most of these tests are
// really asserting.
func press(t *testing.T, m Model, key string) (Model, tea.Cmd) {
	t.Helper()
	var msg tea.KeyMsg
	switch key {
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

// sized returns a connected model with a usable window, which most handlers need.
func sized() Model {
	m := NewModel(nil, "http://test", time.Minute)
	m.width, m.height = 120, 40
	m.connected = true
	return m
}

// onIncident opens the incident detail view directly, skipping the fetch.
func onIncident(inc api.Incident, timeline []api.IncidentEvent) Model {
	m := sized()
	m.mode = modeIncidentDetail
	m.selectedIncident = inc
	m.timeline = timeline
	m.noteCursor = -1
	return m
}

func openIncidentFixture() api.Incident {
	return api.Incident{ID: 1, Title: "DiskFull", Status: api.StatusTriggered,
		Severity: "critical", TriggeredAt: time.Now()}
}

func resolvedIncidentFixture() api.Incident {
	now := time.Now()
	source := "manual"
	inc := openIncidentFixture()
	inc.Status = api.StatusResolved
	inc.ResolvedAt = &now
	inc.ResolutionSource = &source
	return inc
}

// Resolving is terminal on the server: a later occurrence opens a new incident
// rather than reopening this one. A stray keypress must not be able to do that.
func TestResolve_AsksBeforeDoingIt(t *testing.T) {
	m := onIncident(openIncidentFixture(), nil)

	m, cmd := press(t, m, "R")
	if m.mode != modeConfirm {
		t.Fatalf("expected a confirmation prompt, got mode %v", m.mode)
	}
	if m.confirmTarget != confirmResolveIncident {
		t.Errorf("expected the resolve target, got %v", m.confirmTarget)
	}
	if cmd != nil {
		t.Error("nothing should be sent to the server before confirming")
	}
	if !containsAll(m.confirmPrompt(), "final", "new incident") {
		t.Errorf("the prompt should say resolving is final, got %q", m.confirmPrompt())
	}
}

func TestResolve_CancelReturnsToDetailWithoutActing(t *testing.T) {
	m := onIncident(openIncidentFixture(), nil)
	m, _ = press(t, m, "R")

	m, cmd := press(t, m, "n")
	if m.mode != modeIncidentDetail {
		t.Errorf("expected to land back on the incident, got mode %v", m.mode)
	}
	if cmd != nil {
		t.Error("cancelling must not act")
	}
}

func TestResolve_ConfirmActs(t *testing.T) {
	m := onIncident(openIncidentFixture(), nil)
	m, _ = press(t, m, "R")

	m, cmd := press(t, m, "y")
	if cmd == nil {
		t.Error("confirming should issue the resolve")
	}
	if m.mode != modeIncidentDetail {
		t.Errorf("expected to return to the incident, got mode %v", m.mode)
	}
}

// The server answers 409 on all of these; saying so up front beats a round trip.
func TestResolvedIncident_RejectsWorkflowActions(t *testing.T) {
	for _, key := range []string{"a", "A", "R", "s", "z", "Z"} {
		t.Run(key, func(t *testing.T) {
			m := onIncident(resolvedIncidentFixture(), nil)
			m, cmd := press(t, m, key)
			if cmd == nil {
				t.Error("expected a status message command")
			}
			if m.statusMsg == "" {
				t.Error("expected an explanation in the status line")
			}
			if m.mode != modeIncidentDetail {
				t.Errorf("expected to stay on the incident, got mode %v", m.mode)
			}
		})
	}
}

func TestOpenIncident_AcknowledgeTwiceIsRejected(t *testing.T) {
	inc := openIncidentFixture()
	id := int64(2)
	at := time.Now()
	inc.Status = api.StatusAcknowledged
	inc.AcknowledgedByID = &id
	inc.AcknowledgedBy = "alice"
	inc.AcknowledgedAt = &at

	m, _ := press(t, onIncident(inc, nil), "a")
	if !containsAll(m.statusMsg, "already acknowledged", "alice") {
		t.Errorf("expected to be told who holds it, got %q", m.statusMsg)
	}
}

func TestOpenIncident_UnacknowledgeRequiresAnAcknowledgement(t *testing.T) {
	m, _ := press(t, onIncident(openIncidentFixture(), nil), "A")
	if m.statusMsg != "not acknowledged" {
		t.Errorf("expected 'not acknowledged', got %q", m.statusMsg)
	}
}

func TestOpenIncident_UnsnoozeRequiresASnooze(t *testing.T) {
	m, _ := press(t, onIncident(openIncidentFixture(), nil), "Z")
	if m.statusMsg != "not snoozed" {
		t.Errorf("expected 'not snoozed', got %q", m.statusMsg)
	}
}

// Archiving unresolved work only hides it, so the client refuses rather than
// letting the queue be cleared by pressing x.
func TestArchive_RefusesOpenIncident(t *testing.T) {
	t.Run("from the detail view", func(t *testing.T) {
		m, cmd := press(t, onIncident(openIncidentFixture(), nil), "x")
		if cmd == nil || m.statusMsg == "" {
			t.Error("expected a refusal message")
		}
		if m.mode != modeIncidentDetail {
			t.Errorf("expected to stay put, got mode %v", m.mode)
		}
	})

	t.Run("from the queue", func(t *testing.T) {
		m := sized()
		m.incidents = []api.Incident{openIncidentFixture()}
		m.rebuildIncidentTable()

		m, _ = press(t, m, "x")
		if m.statusMsg == "" {
			t.Error("expected a refusal message")
		}
	})
}

func TestArchive_AllowedOnResolvedIncident(t *testing.T) {
	m := sized()
	m.incidents = []api.Incident{resolvedIncidentFixture()}
	m.rebuildIncidentTable()

	m, cmd := press(t, m, "x")
	if cmd == nil {
		t.Error("archiving a resolved incident should act")
	}
	if m.statusMsg != "" {
		t.Errorf("expected no refusal, got %q", m.statusMsg)
	}
}

func TestSnooze_PromptThenSubmit(t *testing.T) {
	m := onIncident(openIncidentFixture(), nil)

	m, _ = press(t, m, "z")
	if m.mode != modeSnooze {
		t.Fatalf("expected the snooze prompt, got mode %v", m.mode)
	}

	// Typing goes to the input, not the key handler.
	for _, r := range "2h" {
		m, _ = press(t, m, string(r))
	}
	if m.snoozeInput.Value() != "2h" {
		t.Fatalf("expected the typed duration, got %q", m.snoozeInput.Value())
	}

	m, cmd := press(t, m, "enter")
	if cmd == nil {
		t.Error("expected the snooze to be sent")
	}
	if m.mode != modeIncidentDetail {
		t.Errorf("expected to return to the incident, got mode %v", m.mode)
	}
}

func TestSnooze_EmptyInputDoesNothing(t *testing.T) {
	m := onIncident(openIncidentFixture(), nil)
	m, _ = press(t, m, "z")

	m, cmd := press(t, m, "enter")
	if cmd != nil {
		t.Error("an empty duration should not be sent")
	}
	if m.mode != modeSnooze {
		t.Errorf("expected to stay on the prompt, got mode %v", m.mode)
	}
}

func TestNote_EscapeAbandonsWithoutPosting(t *testing.T) {
	m := onIncident(openIncidentFixture(), nil)
	m, _ = press(t, m, "c")
	if m.mode != modeNote {
		t.Fatalf("expected the note prompt, got mode %v", m.mode)
	}

	m, cmd := press(t, m, "esc")
	if cmd != nil {
		t.Error("escaping must not post the note")
	}
	if m.mode != modeIncidentDetail {
		t.Errorf("expected to return to the incident, got mode %v", m.mode)
	}
}

func TestNoteCursor_WrapsOverNotesOnly(t *testing.T) {
	timeline := []api.IncidentEvent{
		{ID: 1, Type: api.EventTriggered},
		{ID: 2, Type: api.EventNote, Detail: "first"},
		{ID: 3, Type: api.EventAcknowledged},
		{ID: 4, Type: api.EventNote, Detail: "second"},
	}
	m := onIncident(openIncidentFixture(), timeline)

	m, _ = press(t, m, "]")
	if m.noteCursor != 0 {
		t.Fatalf("expected the first note, got %d", m.noteCursor)
	}
	m, _ = press(t, m, "]")
	if m.noteCursor != 1 {
		t.Fatalf("expected the second note, got %d", m.noteCursor)
	}
	m, _ = press(t, m, "]")
	if m.noteCursor != 0 {
		t.Errorf("expected to wrap to the first note, got %d", m.noteCursor)
	}
	m, _ = press(t, m, "[")
	if m.noteCursor != 1 {
		t.Errorf("expected to wrap backwards to the last note, got %d", m.noteCursor)
	}
}

func TestDeleteNote_RequiresASelection(t *testing.T) {
	m := onIncident(openIncidentFixture(), []api.IncidentEvent{{Type: api.EventTriggered}})
	m, _ = press(t, m, "d")
	if m.mode == modeConfirm {
		t.Error("nothing is selected, so there is nothing to confirm")
	}
	if !containsAll(m.statusMsg, "select a note") {
		t.Errorf("expected guidance, got %q", m.statusMsg)
	}
}

func TestDeleteNote_ConfirmsThenActs(t *testing.T) {
	timeline := []api.IncidentEvent{{ID: 9, Type: api.EventNote, Detail: "hi"}}
	m := onIncident(openIncidentFixture(), timeline)

	m, _ = press(t, m, "]")
	m, _ = press(t, m, "d")
	if m.mode != modeConfirm || m.confirmTarget != confirmDeleteNote {
		t.Fatalf("expected a delete confirmation, got mode %v target %v", m.mode, m.confirmTarget)
	}
	if m.pendingDeleteID != 9 {
		t.Errorf("expected the selected note's id, got %d", m.pendingDeleteID)
	}

	m, cmd := press(t, m, "y")
	if cmd == nil {
		t.Error("confirming should issue the delete")
	}
	if m.mode != modeIncidentDetail {
		t.Errorf("expected to return to the incident, got mode %v", m.mode)
	}
}

func TestTab_CyclesEverySection(t *testing.T) {
	m := sized()
	if m.activeSection != sectionIncidents {
		t.Fatal("incidents is the section the client opens on")
	}

	want := []section{sectionAlerts, sectionStats, sectionArchived, sectionSchedule,
		sectionUsers, sectionIncidents}
	for i, expected := range want {
		m, _ = press(t, m, "tab")
		if m.activeSection != expected {
			t.Fatalf("after %d tabs expected section %v, got %v", i+1, expected, m.activeSection)
		}
	}
}

func TestFilter_CyclesPerSection(t *testing.T) {
	m := sized()
	m, _ = press(t, m, "f")
	if m.incidentFilter != api.StatusTriggered {
		t.Errorf("expected the incident filter to advance, got %q", m.incidentFilter)
	}

	m.activeSection = sectionAlerts
	m, _ = press(t, m, "f")
	if m.alertFilter != "resolved" {
		t.Errorf("expected the alert filter to advance, got %q", m.alertFilter)
	}
	if m.incidentFilter != api.StatusTriggered {
		t.Error("the two filters are independent")
	}
}

// Stats is a section like any other: no key of its own, no mode of its own, and
// it loads once on first visit rather than on every tab-in — the three empty
// slices a quiet server returns are a real answer, not a missing one.
func TestStats_IsAnOrdinarySection(t *testing.T) {
	m := sized()
	m.activeSection = sectionAlerts

	m, cmd := press(t, m, "tab")
	if m.activeSection != sectionStats {
		t.Fatalf("expected the stats section, got %v", m.activeSection)
	}
	if m.mode != modeDashboard {
		t.Errorf("stats is a section, not a mode: got mode %v", m.mode)
	}
	if cmd == nil {
		t.Error("the first visit should fetch")
	}

	m.statsLoaded = true
	m.statsLoading = false
	if cmd := m.loadSectionIfEmpty(); cmd != nil {
		t.Error("a second visit should reuse what was already fetched")
	}
}

// S used to open the stats overlay from anywhere. It is gone, and must not
// disturb the view it is pressed in.
func TestStats_KeyIsGone(t *testing.T) {
	m, _ := press(t, sized(), "S")
	if m.activeSection != sectionIncidents || m.mode != modeDashboard {
		t.Errorf("S should do nothing on the queue, got section %v mode %v",
			m.activeSection, m.mode)
	}

	m, _ = press(t, onIncident(openIncidentFixture(), nil), "S")
	if m.mode != modeIncidentDetail {
		t.Errorf("S should leave the incident open, got mode %v", m.mode)
	}
}

// The overlay never auto-refreshed, because the tick skipped every non-dashboard
// mode. As a section it rides the tick like the rest.
func TestStats_RefreshesOnTick(t *testing.T) {
	m := sized()
	m.activeSection = sectionStats
	if m.refreshActiveSection() == nil {
		t.Error("the stats section should refresh on the tick")
	}
}

// Alerts carry no workflow state, so the detail view offers nothing but a way
// through to the incident.
func TestAlertDetail_IsReadOnly(t *testing.T) {
	m := sized()
	m.mode = modeAlertDetail
	m.selectedAlert = api.Alert{ID: 3, Name: "DiskFull", Status: "firing"}

	for _, key := range []string{"a", "A", "R", "c", "x", "z"} {
		next, cmd := press(t, m, key)
		if cmd != nil {
			t.Errorf("key %q should do nothing on an alert", key)
		}
		if next.mode != modeAlertDetail {
			t.Errorf("key %q changed mode to %v", key, next.mode)
		}
	}
}

func TestAlertDetail_JumpToIncident(t *testing.T) {
	m := sized()
	m.mode = modeAlertDetail

	t.Run("without an incident", func(t *testing.T) {
		m.selectedAlert = api.Alert{ID: 3, Name: "Orphan"}
		next, _ := press(t, m, "i")
		if next.mode != modeAlertDetail || next.statusMsg == "" {
			t.Errorf("expected a refusal, got mode %v msg %q", next.mode, next.statusMsg)
		}
	})

	t.Run("with an incident", func(t *testing.T) {
		id := int64(7)
		m.selectedAlert = api.Alert{ID: 3, Name: "DiskFull", IncidentID: &id}
		next, cmd := press(t, m, "i")
		if next.mode != modeIncidentDetail {
			t.Fatalf("expected the incident view, got mode %v", next.mode)
		}
		if next.selectedIncident.ID != 7 {
			t.Errorf("expected incident 7, got %d", next.selectedIncident.ID)
		}
		if cmd == nil {
			t.Error("expected the incident to be fetched")
		}
	})
}

// A refresh underneath a prompt would move the ground under the user.
func TestRefreshTick_SkipsModalStates(t *testing.T) {
	modal := []mode{modeNote, modeSnooze, modeConfirm, modeUserPicker, modeUserCreate}
	for _, md := range modal {
		m := sized()
		m.mode = md
		if cmd := m.refreshActiveSection(); cmd != nil {
			t.Errorf("mode %v should not auto-refresh", md)
		}
	}

	m := sized()
	if cmd := m.refreshActiveSection(); cmd == nil {
		t.Error("the dashboard should auto-refresh")
	}
}

func TestIncidentsFetched_ClearsLoading(t *testing.T) {
	m := sized()
	m.loading = true
	next, _ := m.Update(incidentsFetchedMsg{incidents: []api.Incident{openIncidentFixture()}})
	got := next.(Model)
	if got.loading {
		t.Error("expected loading to clear")
	}
	if len(got.incidents) != 1 {
		t.Errorf("expected the incidents stored, got %d", len(got.incidents))
	}
}

// A note deleted elsewhere must not leave the cursor pointing past the end.
func TestIncidentDetailFetched_ClampsNoteCursor(t *testing.T) {
	m := onIncident(openIncidentFixture(), nil)
	m.noteCursor = 3

	next, _ := m.Update(incidentDetailFetchedMsg{
		incident: openIncidentFixture(),
		timeline: []api.IncidentEvent{{Type: api.EventTriggered}},
	})
	if got := next.(Model).noteCursor; got != -1 {
		t.Errorf("expected the cursor reset, got %d", got)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
