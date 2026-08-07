package tui

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/yeniklas/terdut-tui/internal/api"
)

func TestNextFilter(t *testing.T) {
	tests := []struct {
		name    string
		cycle   []string
		current string
		want    string
	}{
		{"advances", incidentFilters, "", api.StatusTriggered},
		{"advances again", incidentFilters, api.StatusTriggered, api.StatusAcknowledged},
		{"wraps back to the open queue", incidentFilters, "snoozed", ""},
		{"alerts advance", alertFilters, "firing", "resolved"},
		{"alerts wrap", alertFilters, "archived", "firing"},
		{"unknown current restarts the cycle", incidentFilters, "bogus", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextFilter(tt.cycle, tt.current); got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

// "snoozed" is a pseudo-status in the filter cycle: the server has no such
// status, it is a separate query axis.
func TestIncidentQuery(t *testing.T) {
	tests := []struct {
		filter      string
		wantStatus  string
		wantSnoozed bool
	}{
		{"", "", false},
		{api.StatusTriggered, api.StatusTriggered, false},
		{api.StatusResolved, api.StatusResolved, false},
		{"snoozed", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.filter, func(t *testing.T) {
			status, snoozed := incidentQuery(tt.filter)
			if status != tt.wantStatus || snoozed != tt.wantSnoozed {
				t.Errorf("expected (%q, %v), got (%q, %v)",
					tt.wantStatus, tt.wantSnoozed, status, snoozed)
			}
		})
	}
}

func TestFilterLabel(t *testing.T) {
	if got := filterLabel(""); got != "open" {
		t.Errorf("the empty filter is the open queue, got %q", got)
	}
	if got := filterLabel("resolved"); got != "resolved" {
		t.Errorf("expected resolved, got %q", got)
	}
}

func TestNoteEvents(t *testing.T) {
	timeline := []api.IncidentEvent{
		{Type: api.EventTriggered},
		{Type: api.EventNote, Detail: "first"},
		{Type: api.EventAcknowledged},
		{Type: api.EventNote, Detail: "second"},
	}
	notes := noteEvents(timeline)
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	if notes[0].Detail != "first" || notes[1].Detail != "second" {
		t.Errorf("notes out of order: %v", notes)
	}
	if len(noteEvents(nil)) != 0 {
		t.Error("an empty timeline has no notes")
	}
}

func TestHumanDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{5 * time.Second, "moments"},
		{90 * time.Second, "1m"},
		{45 * time.Minute, "45m"},
		{2 * time.Hour, "2h"},
		{150 * time.Minute, "2h 30m"},
		{48 * time.Hour, "2d"},
		{50 * time.Hour, "2d 2h"},
	}
	for _, tt := range tests {
		if got := humanDuration(tt.d); got != tt.want {
			t.Errorf("humanDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestHumanAgo_ClampsFutureToNow(t *testing.T) {
	now := time.Now()
	// Server and client clocks disagree often enough that this must not render
	// as a negative age.
	if got := humanAgo(now, now.Add(time.Hour)); got != "moments ago" {
		t.Errorf("expected a future timestamp to clamp, got %q", got)
	}
}

func TestHumanUntil(t *testing.T) {
	now := time.Now()
	if got := humanUntil(now, now.Add(2*time.Hour)); got != "in 2h" {
		t.Errorf("expected 'in 2h', got %q", got)
	}
	if got := humanUntil(now, now.Add(-time.Minute)); got != "expired" {
		t.Errorf("a deadline in the past has expired, got %q", got)
	}
}

// MTTA and MTTR are nil until something has been acknowledged or resolved, and
// that has to read as "no data" rather than an instant response.
func TestHumanSeconds(t *testing.T) {
	if got := humanSeconds(nil); got != "—" {
		t.Errorf("expected an em dash for no data, got %q", got)
	}
	secs := 150.0
	if got := humanSeconds(&secs); got != "2m" {
		t.Errorf("expected 2m, got %q", got)
	}
}

func TestIncidentRows(t *testing.T) {
	future := time.Now().Add(time.Hour)
	rows := incidentRows([]api.Incident{
		{Title: "DiskFull", Status: api.StatusTriggered, Severity: "critical",
			AssignedTo: "admin", TriggeredAt: time.Now()},
		{Title: "Unowned", Status: api.StatusTriggered, TriggeredAt: time.Now()},
		{Title: "Quiet", Status: api.StatusTriggered, Severity: "info",
			AssignedTo: "alice", SnoozedUntil: &future, TriggeredAt: time.Now()},
	})
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[0][0] != "critical" || rows[0][3] != "admin" {
		t.Errorf("unexpected first row %v", rows[0])
	}
	if rows[1][0] != "—" || rows[1][3] != "—" {
		t.Errorf("missing severity and assignee should show an em dash, got %v", rows[1])
	}
	// bubbles' table renders plain strings, so snooze has to be marked in text.
	if rows[2][2] != "triggered (zzz)" {
		t.Errorf("expected a snooze marker in the status cell, got %q", rows[2][2])
	}
}

func TestAlertRows_ShowIncidentLink(t *testing.T) {
	id := int64(7)
	rows := alertRows([]api.Alert{
		{Name: "DiskFull", Status: "firing", IncidentID: &id},
		{Name: "Orphan", Status: "resolved"},
	})
	if rows[0][4] != "#7" {
		t.Errorf("expected #7, got %q", rows[0][4])
	}
	if rows[1][4] != "—" {
		t.Errorf("an alert with no incident shows an em dash, got %q", rows[1][4])
	}
}

// A user with no ntfy topic gets no pages of their own — the row has to say so
// rather than leaving a blank that reads as "not loaded yet".
func TestUserManageRows_ShowMissingTopic(t *testing.T) {
	topic := "terdut-niklas"
	empty := ""
	m := NewModel(nil, "http://test", time.Minute)
	m.width, m.height = 120, 40
	m.users = []api.User{
		{ID: 1, Username: "niklas", NtfyTopic: &topic},
		{ID: 2, Username: "alex"},
		// The server stores a blank topic as NULL, but a stale client or an older
		// server can still hand one back; it means the same thing.
		{ID: 3, Username: "sam", NtfyTopic: &empty},
	}
	m.rebuildUserManageTable()

	rows := m.userManageTable.Rows()
	if rows[0][2] != "terdut-niklas" {
		t.Errorf("expected the topic in the row, got %q", rows[0][2])
	}
	if rows[1][2] != "—" || rows[2][2] != "—" {
		t.Errorf("expected an em dash for nil and empty topics, got %q and %q",
			rows[1][2], rows[2][2])
	}
}

// A previous release overflowed the terminal by two columns because the padding
// budget was wrong. Columns plus bubbles' per-cell padding must land exactly on
// the window width.
func TestColumnWidthsFitTheTerminal(t *testing.T) {
	for _, width := range []int{100, 110, 140, 200} {
		for name, cols := range map[string][]int{
			"incident": widths(incidentColumns(width)),
			"alert":    widths(alertColumns(width)),
		} {
			sum := 0
			for _, w := range cols {
				sum += w
			}
			const padding = 10 // bubbles applies Padding(0, 1) to each of five cells
			if sum+padding != width {
				t.Errorf("%s columns at width %d sum to %d+%d = %d",
					name, width, sum, padding, sum+padding)
			}
		}

		// The users table is four cells, so its padding budget differs.
		sum := 0
		for _, w := range widths(userManageColumns(width)) {
			sum += w
		}
		const userPadding = 8
		if sum+userPadding != width {
			t.Errorf("user columns at width %d sum to %d+%d = %d",
				width, sum, userPadding, sum+userPadding)
		}
	}
}

// Narrow terminals fall back to minimum widths, which legitimately overflow;
// what must not happen is a negative or zero column.
func TestColumnWidthsStayPositiveWhenNarrow(t *testing.T) {
	for _, width := range []int{20, 40, 60} {
		cols := append(widths(incidentColumns(width)), widths(alertColumns(width))...)
		cols = append(cols, widths(userManageColumns(width))...)
		for _, w := range cols {
			if w < 1 {
				t.Errorf("width %d produced a non-positive column %d", width, w)
			}
		}
	}
}

func widths(cols []table.Column) []int {
	out := make([]int, len(cols))
	for i, c := range cols {
		out[i] = c.Width
	}
	return out
}

func TestTableHeight_NeverGoesBelowOne(t *testing.T) {
	if got := tableHeight(3, 10); got != 1 {
		t.Errorf("expected a floor of 1, got %d", got)
	}
	if got := tableHeight(40, 8); got != 32 {
		t.Errorf("expected 32, got %d", got)
	}
}

func TestBuildScheduleDays(t *testing.T) {
	monday := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	days := buildScheduleDays(monday, []api.ScheduleEntry{
		{Date: "2026-07-29", Username: "alice"},
	})
	if len(days) != 7 {
		t.Fatalf("expected a 7-day window, got %d", len(days))
	}
	if days[2].entry == nil || days[2].entry.Username != "alice" {
		t.Errorf("expected alice on the third day, got %+v", days[2].entry)
	}
	if days[0].entry != nil {
		t.Error("expected unassigned days to have no entry")
	}
}

// ── Schedule reassignment ─────────────────────────────────────────────────

// scheduledWeek builds a model showing the week of 2026-07-27 with the given
// entries already on the rota.
func scheduledWeek(entries []api.ScheduleEntry) Model {
	m := NewModel(nil, "http://test", time.Minute)
	m.width, m.height = 120, 40
	m.connected = true
	m.activeSection = sectionSchedule
	m.scheduleWindow = time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	m.scheduleEntries = entries
	m.scheduleDays = buildScheduleDays(m.scheduleWindow, entries)
	m.rebuildScheduleTable()
	return m
}

func TestScheduleConflicts(t *testing.T) {
	m := scheduledWeek([]api.ScheduleEntry{
		{ID: 1, Date: "2026-07-27", UserID: 1, Username: "niklas"},
		{ID: 2, Date: "2026-07-28", UserID: 3, Username: "sam"},
		{ID: 3, Date: "2026-07-29", UserID: 2, Username: "alex"},
	})
	week := []string{"2026-07-27", "2026-07-28", "2026-07-29", "2026-07-30"}

	// Assigning alex: the days niklas and sam hold are conflicts, the day alex
	// already holds is not, and the free day is not.
	taken, holders := m.scheduleConflicts(week, 2)
	if len(taken) != 2 || taken[0] != "2026-07-27" || taken[1] != "2026-07-28" {
		t.Errorf("expected the two other people's days, got %v", taken)
	}
	if len(holders) != 2 || holders[0] != "niklas" || holders[1] != "sam" {
		t.Errorf("expected both holders named once, got %v", holders)
	}
}

// Reassigning somebody to a day they already hold takes nothing from anyone, so
// it must not raise a prompt — but it still needs replace, because the server
// rejects any date that already exists.
func TestScheduleConflicts_OwnDayIsNotAConflict(t *testing.T) {
	m := scheduledWeek([]api.ScheduleEntry{
		{ID: 1, Date: "2026-07-27", UserID: 2, Username: "alex"},
	})
	dates := []string{"2026-07-27"}

	if taken, _ := m.scheduleConflicts(dates, 2); len(taken) != 0 {
		t.Errorf("expected no conflict on the user's own day, got %v", taken)
	}
	if !m.scheduleOccupied(dates) {
		t.Error("expected the day to still count as occupied, so replace is sent")
	}
}

func TestScheduleOccupied_FreeDays(t *testing.T) {
	m := scheduledWeek(nil)
	if m.scheduleOccupied([]string{"2026-07-27", "2026-07-28"}) {
		t.Error("expected an empty rota to need no replace")
	}
}

func TestDayCount(t *testing.T) {
	tests := []struct {
		taken, total int
		want         string
	}{
		{1, 1, "This day is"},
		{7, 7, "All 7 days are"},
		{3, 7, "3 of 7 days are"},
	}
	for _, tt := range tests {
		if got := dayCount(tt.taken, tt.total); got != tt.want {
			t.Errorf("dayCount(%d, %d) = %q, want %q", tt.taken, tt.total, got, tt.want)
		}
	}
}

func TestJoinNames(t *testing.T) {
	tests := []struct {
		names []string
		want  string
	}{
		{nil, "somebody else"},
		{[]string{"niklas"}, "niklas"},
		{[]string{"niklas", "alex"}, "niklas and alex"},
		{[]string{"niklas", "alex", "sam"}, "niklas, alex and sam"},
	}
	for _, tt := range tests {
		if got := joinNames(tt.names); got != tt.want {
			t.Errorf("joinNames(%v) = %q, want %q", tt.names, got, tt.want)
		}
	}
}
