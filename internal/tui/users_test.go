package tui

import (
	"strings"
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

func TestPassword_OpensForTheSelectedUserAndLooksUpWhoIAm(t *testing.T) {
	m, cmd := press(t, threeUsers(), "p")
	if m.mode != modePasswordSet || m.selectedUser.Username != "erik" {
		t.Fatalf("expected the password form for erik, got mode %v for %q", m.mode, m.selectedUser.Username)
	}
	if !m.pwLoading || cmd == nil {
		t.Error("the form should look up /api/me before it is usable")
	}
	if !strings.Contains(m.View(), "Checking who this key belongs to") {
		t.Error("the form should say it is waiting")
	}
}

func TestPassword_SomeoneElseNeedsNoCurrentPassword(t *testing.T) {
	m, _ := press(t, threeUsers(), "p")
	next, _ := m.Update(meFetchedMsg{me: api.Me{User: api.User{ID: 1}, HasPassword: true}})
	m = next.(Model)
	if m.pwNeedCurrent || m.pwFocus != pwNew {
		t.Errorf("setting erik's password as niklas should not ask for a current one")
	}
	if strings.Contains(m.View(), "Current password") {
		t.Error("the current-password field should be hidden")
	}
}

func TestPassword_OwnExistingPasswordNeedsCurrent(t *testing.T) {
	m := threeUsers()
	m.userManageTable.SetCursor(0)
	m, _ = press(t, m, "p")
	next, _ := m.Update(meFetchedMsg{me: api.Me{User: api.User{ID: 1}, HasPassword: true}})
	m = next.(Model)
	if !m.pwNeedCurrent || m.pwFocus != pwCurrent {
		t.Fatal("changing your own existing password should ask for the current one first")
	}
	m = typeInto(t, m, "correct horse")
	m, _ = press(t, m, "tab")
	m = typeInto(t, m, "a brand new secret")
	m, _ = press(t, m, "tab")
	m = typeInto(t, m, "a brand new secret")
	m, cmd := press(t, m, "enter")
	if cmd == nil || m.mode != modeDashboard {
		t.Errorf("a complete form should submit (mode %v)", m.mode)
	}
}

func TestPassword_OwnFirstPasswordNeedsNoCurrent(t *testing.T) {
	m := threeUsers()
	m.userManageTable.SetCursor(0)
	m, _ = press(t, m, "p")
	next, _ := m.Update(meFetchedMsg{me: api.Me{User: api.User{ID: 1}, HasPassword: false}})
	if next.(Model).pwNeedCurrent {
		t.Error("there is no current password to ask for yet")
	}
}

func TestPassword_RejectsBeforeSending(t *testing.T) {
	cases := []struct{ name, pw, repeat, want string }{
		{"too short", "short", "short", "at least 10"},
		{"mismatch", "a brand new secret", "a different secret", "do not match"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := press(t, threeUsers(), "p")
			next, _ := m.Update(meFetchedMsg{me: api.Me{User: api.User{ID: 1}}})
			m = typeInto(t, next.(Model), tc.pw)
			m, _ = press(t, m, "tab")
			m = typeInto(t, m, tc.repeat)
			m, cmd := press(t, m, "enter")
			if m.mode != modePasswordSet {
				t.Error("the form should stay open")
			}
			if !strings.Contains(m.statusMsg, tc.want) {
				t.Errorf("status %q should mention %q", m.statusMsg, tc.want)
			}
			if cmd == nil {
				return
			}
			// Only the clear-status timer may be scheduled, never a request.
			if _, ok := cmd().(clearStatusMsg); !ok {
				t.Error("nothing should be sent to the server")
			}
		})
	}
}

func TestPassword_EscapeCancels(t *testing.T) {
	m, _ := press(t, threeUsers(), "p")
	m, _ = press(t, m, "esc")
	if m.mode != modeDashboard {
		t.Errorf("esc should close the form, got mode %v", m.mode)
	}
	// A lookup arriving after the form closed must not reopen anything.
	next, _ := m.Update(meFetchedMsg{me: api.Me{User: api.User{ID: 3}, HasPassword: true}})
	if next.(Model).mode != modeDashboard {
		t.Error("a late /api/me answer reopened the form")
	}
}

func TestPassword_ErrorClosesTheFormWithAMessage(t *testing.T) {
	m, _ := press(t, threeUsers(), "p")
	next, _ := m.Update(userActionErrMsg{errTest("server returned 403: current password is incorrect")})
	m = next.(Model)
	if m.mode != modeDashboard || !strings.Contains(m.statusMsg, "current password is incorrect") {
		t.Errorf("expected the server's message on the dashboard, got %q in mode %v", m.statusMsg, m.mode)
	}
}

func typeInto(t *testing.T, m Model, s string) Model {
	t.Helper()
	next, _ := m.Update(runes(s))
	return next.(Model)
}

func runes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func keyUp() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyUp} }

type errTest string

func (e errTest) Error() string { return string(e) }
