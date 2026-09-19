package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yeniklas/terdut-tui/internal/api"
)

// threeUsers is the Users section with the cursor on the last of three users.
func threeUsers() Model {
	m := onUsers([]api.User{
		{ID: 1, Username: "niklas"},
		{ID: 2, Username: "anna"},
		{ID: 3, Username: "erik"},
	})
	m.userManageTable.SetCursor(2)
	return m
}

// The table used to see k before the section did, take it as "up", and the
// handler then opened API keys for the user above the one selected.
func TestUsers_APIKeysOpenForTheSelectedUser(t *testing.T) {
	m, _ := press(t, threeUsers(), "k")
	if m.mode != modeAPIKeyMenu {
		t.Fatalf("expected the API key menu, got mode %v", m.mode)
	}
	if m.selectedUser.Username != "erik" {
		t.Errorf("API keys opened for %s, want erik", m.selectedUser.Username)
	}
}

// Same collision with d, which the table read as half a page down: the delete
// confirmation named a different user than the one under the cursor.
func TestUsers_DeleteTargetsTheSelectedUser(t *testing.T) {
	m := threeUsers()
	m.userManageTable.SetCursor(0)
	m, _ = press(t, m, "d")
	if m.mode != modeConfirm || m.selectedUser.Username != "niklas" {
		t.Errorf("delete asked about %q in mode %v, want niklas", m.selectedUser.Username, m.mode)
	}
}

func TestSchedule_DeleteTargetsTheSelectedDay(t *testing.T) {
	m := sized()
	m.activeSection = sectionSchedule
	m.scheduleEntries = []api.ScheduleEntry{
		{ID: 10, UserID: 1, Username: "niklas", Date: m.scheduleWindow.Format("2006-01-02")},
		{ID: 11, UserID: 2, Username: "anna", Date: m.scheduleWindow.AddDate(0, 0, 1).Format("2006-01-02")},
	}
	m.scheduleDays = buildScheduleDays(m.scheduleWindow, m.scheduleEntries)
	m.rebuildScheduleTable()
	m.scheduleTable.SetCursor(0)
	m, _ = press(t, m, "d")
	if m.pendingDeleteEntry == nil || m.pendingDeleteEntry.ID != 10 {
		t.Errorf("schedule delete targeted %+v, want entry 10", m.pendingDeleteEntry)
	}
}

// f cycles the filter; it must not also page the cursor down.
func TestFilter_DoesNotMoveTheCursor(t *testing.T) {
	m := sized()
	m.incidents = make([]api.Incident, 40)
	for i := range m.incidents {
		m.incidents[i] = api.Incident{ID: int64(i + 1), Title: "x", Status: api.StatusTriggered, TriggeredAt: time.Now()}
	}
	m.rebuildIncidentTable()
	m, _ = press(t, m, "f")
	if c := m.incidentTable.Cursor(); c != 0 {
		t.Errorf("f moved the cursor to %d", c)
	}
}

// The arrow keys still move the users table, now that k is an action there.
func TestUsers_ArrowKeysStillNavigate(t *testing.T) {
	m := threeUsers()
	next, _ := m.Update(keyUp())
	if c := next.(Model).userManageTable.Cursor(); c != 1 {
		t.Errorf("up arrow left the cursor on %d, want 1", c)
	}
}

func keyUp() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyUp} }
