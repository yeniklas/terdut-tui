package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/yeniklas/terdut-tui/internal/api"
)

var sectionNames = []string{"Incidents", "Alerts", "Archived", "Schedule", "Users"}

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(),
		m.renderTabs(),
		m.renderBody(),
		m.renderFooter(),
	)
}

func (m Model) renderHeader() string {
	title := styleHeader.Render("terdut-tui")
	right := styleMuted.Render(m.serverURL)
	return spread(title, right, m.width)
}

func (m Model) renderTabs() string {
	var tabs []string
	for i, name := range sectionNames {
		if section(i) == m.activeSection {
			tabs = append(tabs, styleTabActive.Render(name))
		} else {
			tabs = append(tabs, styleTabInactive.Render(name))
		}
	}
	sep := styleMuted.Render(strings.Repeat("─", m.width))
	return strings.Join(tabs, "") + "\n" + sep
}

func (m Model) renderBody() string {
	if m.err != nil {
		return "\n" + styleError.Render(fmt.Sprintf("  Error: %v", m.err)) +
			"\n" + styleMuted.Render("  Press r to retry.")
	}
	if !m.connected {
		return "\n" + styleMuted.Render("  Connecting…")
	}

	switch m.mode {
	case modeIncidentDetail, modeAlertDetail:
		return m.renderDetail()
	case modeNote:
		return m.renderPrompt(styleHeader.Render("Note: ") + m.noteInput.View())
	case modeSnooze:
		return m.renderPrompt(styleHeader.Render("Snooze for: ") + m.snoozeInput.View())
	case modeConfirm:
		switch m.confirmTarget {
		case confirmDeleteNote, confirmResolveIncident:
			return m.renderDetail()
		case confirmDeleteUser:
			return m.renderUsers()
		default:
			return m.renderSchedule()
		}
	case modeStats:
		return m.renderStats()
	case modeUserPicker:
		return m.renderUserPicker()
	case modeUserCreate:
		return m.renderUserCreate()
	case modeAPIKeyMenu:
		return m.renderAPIKeyMenu()
	case modeAPIKeyCreate:
		return m.renderAPIKeyCreate()
	case modeAPIKeyReveal:
		return m.renderAPIKeyReveal()
	case modeAPIKeyRevokeByID:
		return m.renderAPIKeyRevokeByID()
	default:
		return m.renderDashboard()
	}
}

func (m Model) renderFooter() string {
	withStatus := func(actions string) string {
		rendered := styleFooter.Render(actions)
		if m.statusMsg != "" {
			return styleStatus.Render("  "+m.statusMsg) + "\n" + rendered
		}
		return "\n" + rendered
	}

	switch m.mode {
	case modeIncidentDetail:
		if !m.selectedIncident.IsOpen() {
			return withStatus("  x·archive  c·note  [/]·select  d·del  S·stats  esc·back")
		}
		return withStatus("  a·ack  A·unack  R·resolve  s·assign  z·snooze  Z·unsnooze  c·note  [/]·select  d·del  S·stats  esc·back")

	case modeAlertDetail:
		return withStatus("  i·open incident  S·stats  esc·back")

	case modeNote:
		return "\n" + styleFooter.Render("  enter·submit  esc·cancel")

	case modeSnooze:
		return "\n" + styleFooter.Render("  enter·snooze  esc·cancel   (e.g. 30m, 2h, 24h)")

	case modeConfirm:
		return "\n" + styleError.Render("  "+m.confirmPrompt())

	case modeStats:
		return withStatus("  esc·back")

	case modeUserPicker:
		if m.pickerTarget == pickerIncidentAssignee {
			return withStatus("  j/k·navigate  enter·assign incident  esc·cancel")
		}
		scope := "day"
		if m.pickerAssignWeek {
			scope = "week"
		}
		return withStatus(fmt.Sprintf("  j/k·navigate  enter·assign %s  esc·cancel", scope))

	case modeUserCreate:
		return withStatus("  tab·next field  enter·create  esc·cancel")

	case modeAPIKeyMenu:
		return withStatus("  n·new key  r·revoke by ID  esc·back")

	case modeAPIKeyCreate:
		return withStatus("  enter·create  esc·back")

	case modeAPIKeyReveal:
		return withStatus("  c·copy to clipboard  esc·done")

	case modeAPIKeyRevokeByID:
		return withStatus("  enter·revoke  esc·back")

	default:
		switch m.activeSection {
		case sectionIncidents:
			return withStatus("  enter·detail  x·archive  f·filter  S·stats  r·refresh  tab·section  q·quit")
		case sectionAlerts:
			return withStatus("  enter·detail  f·filter  S·stats  r·refresh  tab·section  q·quit")
		case sectionArchived:
			return withStatus("  enter·detail  x·unarchive  r·refresh  tab·section  q·quit")
		case sectionSchedule:
			return withStatus("  +·assign day  W·assign week  d·del  ←/→·shift week  tab·section  r·refresh  q·quit")
		case sectionUsers:
			return withStatus("  n·new user  d·delete  k·API keys  r·refresh  tab·section  q·quit")
		}
		return "\n" + styleFooter.Render(m.help.ShortHelpView(m.keys.ShortHelp()))
	}
}

func (m Model) confirmPrompt() string {
	switch m.confirmTarget {
	case confirmDeleteNote:
		return "Delete this note? [y/N]"
	case confirmResolveIncident:
		// Manual resolution is terminal on the server, so say so before asking.
		return "Resolve incident? This is final — a new occurrence opens a new incident. [y/N]"
	case confirmDeleteScheduleEntry:
		if m.pendingDeleteEntry != nil {
			return fmt.Sprintf("Delete on-call for %s (%s)? [y/N]",
				m.pendingDeleteEntry.Date, m.pendingDeleteEntry.Username)
		}
		return "Delete schedule entry? [y/N]"
	case confirmDeleteUser:
		return fmt.Sprintf("Delete user %s (cascades all API keys)? [y/N]", m.selectedUser.Username)
	}
	return "Are you sure? [y/N]"
}

// ── Dashboard ──────────────────────────────────────────────────────────────

func (m Model) renderDashboard() string {
	switch m.activeSection {
	case sectionIncidents:
		return m.renderIncidents()
	case sectionAlerts:
		return m.renderAlerts()
	case sectionArchived:
		return m.renderArchived()
	case sectionSchedule:
		return m.renderSchedule()
	case sectionUsers:
		return m.renderUsers()
	}
	return ""
}

func (m Model) renderIncidents() string {
	bar := m.renderIncidentStatsBar()
	var content string
	switch {
	case m.loading && len(m.incidents) == 0:
		content = styleMuted.Render("  Loading incidents…")
	case len(m.incidents) == 0:
		content = styleMuted.Render(
			fmt.Sprintf("  No %s incidents.", filterLabel(m.incidentFilter)))
	default:
		content = m.incidentTable.View()
	}
	return lipgloss.JoinVertical(lipgloss.Left, bar, content)
}

func (m Model) renderAlerts() string {
	bar := m.renderAlertStatsBar()
	var content string
	switch {
	case m.loading && len(m.alerts) == 0:
		content = styleMuted.Render("  Loading alerts…")
	case len(m.alerts) == 0:
		content = styleMuted.Render(fmt.Sprintf("  No %s alerts.", filterLabel(m.alertFilter)))
	default:
		content = m.alertTable.View()
	}
	return lipgloss.JoinVertical(lipgloss.Left, bar, content)
}

func (m Model) renderArchived() string {
	if m.archivedLoading {
		return "\n" + styleMuted.Render("  Loading archived incidents…")
	}
	if len(m.archivedIncidents) == 0 {
		return "\n" + styleMuted.Render("  No archived incidents.")
	}
	return "\n" + m.archivedTable.View()
}

func (m Model) renderIncidentStatsBar() string {
	var triggered, acked, resolved int
	mtta, mttr := "—", "—"
	if m.incidentStats != nil {
		triggered = m.incidentStats.Triggered
		acked = m.incidentStats.Acknowledged
		resolved = m.incidentStats.Resolved
		mtta = humanSeconds(m.incidentStats.MTTASeconds)
		mttr = humanSeconds(m.incidentStats.MTTRSeconds)
	}
	left := fmt.Sprintf("  %s  %s  %s   %s",
		styleTriggered.Render(fmt.Sprintf("Triggered: %d", triggered)),
		styleAcknowledged.Render(fmt.Sprintf("Acked: %d", acked)),
		styleResolved.Render(fmt.Sprintf("Resolved: %d", resolved)),
		styleMuted.Render(fmt.Sprintf("MTTA %s · MTTR %s", mtta, mttr)),
	)
	right := styleMuted.Render(fmt.Sprintf("filter: %s  [f]  ", filterLabel(m.incidentFilter)))
	return spread(left, right, m.width)
}

func (m Model) renderAlertStatsBar() string {
	total, firing, resolved := 0, 0, 0
	if m.alertStats != nil {
		total = m.alertStats.Total
		firing = m.alertStats.Firing
		resolved = m.alertStats.Resolved
	}
	left := fmt.Sprintf("  Total: %d  %s  %s",
		total,
		styleFiring.Render(fmt.Sprintf("Firing: %d", firing)),
		styleResolved.Render(fmt.Sprintf("Resolved: %d", resolved)),
	)
	right := styleMuted.Render(fmt.Sprintf("filter: %s  [f]  ", filterLabel(m.alertFilter)))
	return spread(left, right, m.width)
}

// spread pushes left and right to the edges of a width.
func spread(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return left + strings.Repeat(" ", gap) + right
}

// ── Schedule ───────────────────────────────────────────────────────────────

func (m Model) renderSchedule() string {
	if m.scheduleLoading {
		return "\n" + styleMuted.Render("  Loading schedule…")
	}

	var onCallLine string
	if m.currentOnCall != nil {
		onCallLine = fmt.Sprintf("  On-call today: %s",
			styleAlertName.Render(m.currentOnCall.Username))
	} else {
		onCallLine = styleMuted.Render("  On-call today: nobody scheduled")
	}

	from := m.scheduleWindow
	to := m.scheduleWindow.AddDate(0, 0, 6)
	windowLabel := styleMuted.Render(fmt.Sprintf("  %s — %s",
		from.Format("Jan 02"), to.Format("Jan 02, 2006")))

	header := "\n" + spread(onCallLine, windowLabel, m.width) + "\n"
	return header + m.scheduleTable.View()
}

func (m Model) renderUserPicker() string {
	if m.usersLoading {
		return "\n" + styleMuted.Render("  Loading users…")
	}

	if m.pickerTarget == pickerIncidentAssignee {
		header := fmt.Sprintf("\n  Assign %s to:\n\n",
			styleBold.Render(m.selectedIncident.Title))
		return header + m.userPickerTable.View()
	}

	var scope string
	cursor := m.scheduleTable.Cursor()
	if cursor >= 0 && cursor < len(m.scheduleDays) {
		d := m.scheduleDays[cursor].date
		if m.pickerAssignWeek {
			weekday := int(d.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			monday := d.AddDate(0, 0, -(weekday - 1))
			sunday := monday.AddDate(0, 0, 6)
			_, week := d.ISOWeek()
			scope = fmt.Sprintf("week W%02d (%s–%s)",
				week, monday.Format("Jan 02"), sunday.Format("Jan 02"))
		} else if d.Format("2006-01-02") == time.Now().UTC().Format("2006-01-02") {
			scope = "Today (" + d.Format("Mon") + ")"
		} else {
			scope = d.Format("Jan 02 (Mon)")
		}
	}

	header := fmt.Sprintf("\n  Assign on-call for %s — select a user:\n\n", styleBold.Render(scope))
	return header + m.userPickerTable.View()
}

// ── Detail ─────────────────────────────────────────────────────────────────

func (m Model) renderDetail() string {
	if m.detailLoading {
		return "\n" + styleMuted.Render("  Loading…")
	}
	return m.detailViewport.View()
}

// renderPrompt puts an input line under the detail pane.
func (m Model) renderPrompt(prompt string) string {
	sep := styleMuted.Render(strings.Repeat("─", m.width))
	return m.detailViewport.View() + "\n" + sep + "\n" + prompt
}

// ── Stats ──────────────────────────────────────────────────────────────────

func (m Model) renderStats() string {
	if m.statsLoading {
		return "\n" + styleMuted.Render("  Loading statistics…")
	}
	return m.statsViewport.View()
}

// ── Content builders ───────────────────────────────────────────────────────

func buildIncidentDetailContent(inc api.Incident, timeline []api.IncidentEvent, cursor, width int) string {
	now := time.Now()
	var b strings.Builder
	contentW := width - 4

	// Title + status header
	title := styleAlertName.Render(inc.Title)
	status := incidentStatusStyle(inc.Status).Render(incidentStatusLabel(inc))
	if inc.Severity != "" {
		status += "  " + severityStyle(inc.Severity).Render(strings.ToUpper(inc.Severity))
	}
	gap := contentW - lipgloss.Width(title) - lipgloss.Width(status)
	if gap < 1 {
		gap = 1
	}
	b.WriteString("\n  " + title + strings.Repeat(" ", gap) + status + "\n\n")

	// Timing and ownership
	b.WriteString(fmt.Sprintf("  Triggered:  %s  (%s)\n",
		inc.TriggeredAt.UTC().Format("2006-01-02 15:04 UTC"), humanAgo(now, inc.TriggeredAt)))

	if inc.AssignedTo != "" {
		b.WriteString(fmt.Sprintf("  Assigned:   %s\n", styleBold.Render(inc.AssignedTo)))
	} else {
		b.WriteString(styleMuted.Render("  Assigned:   nobody\n"))
	}

	if inc.AcknowledgedByID != nil {
		ackAt := ""
		if inc.AcknowledgedAt != nil {
			ackAt = "  at " + inc.AcknowledgedAt.UTC().Format("2006-01-02 15:04 UTC")
		}
		b.WriteString(styleResolved.Render(
			fmt.Sprintf("  Acked:      %s%s\n", inc.AcknowledgedBy, ackAt)))
	} else {
		b.WriteString(styleMuted.Render("  Acked:      not acknowledged\n"))
	}

	if inc.IsSnoozed() {
		b.WriteString(styleSnoozed.Render(fmt.Sprintf("  Snoozed:    until %s  (%s)\n",
			inc.SnoozedUntil.UTC().Format("2006-01-02 15:04 UTC"), humanUntil(now, *inc.SnoozedUntil))))
	}

	if inc.ResolvedAt != nil {
		source := ""
		if inc.ResolutionSource != nil {
			source = "  · " + *inc.ResolutionSource
		}
		b.WriteString(fmt.Sprintf("  Resolved:   %s  (%s)%s\n",
			inc.ResolvedAt.UTC().Format("2006-01-02 15:04 UTC"), humanAgo(now, *inc.ResolvedAt), source))
	}
	if inc.ArchivedAt != nil {
		b.WriteString(styleMuted.Render("  Archived:   " +
			inc.ArchivedAt.UTC().Format("2006-01-02 15:04 UTC") + "\n"))
	}
	b.WriteString("\n")

	// Group labels — the correlation Alertmanager applied.
	if len(inc.GroupLabels) > 0 {
		b.WriteString(divider("Grouped By", width))
		for _, k := range sortedKeys(inc.GroupLabels) {
			b.WriteString(fmt.Sprintf("  %-22s %s\n", k, truncate(inc.GroupLabels[k], contentW-24)))
		}
		b.WriteString("\n")
	}

	// Member alerts
	b.WriteString(divider(fmt.Sprintf("Alerts (%d)", len(inc.Alerts)), width))
	if len(inc.Alerts) == 0 {
		b.WriteString(styleMuted.Render("  No alerts.\n"))
	} else {
		for _, a := range inc.Alerts {
			marker := styleFiring.Render("●")
			if a.Status != "firing" {
				marker = styleResolved.Render("✓")
			}
			instance := a.Labels["instance"]
			if instance == "" {
				instance = a.Fingerprint
			}
			b.WriteString(fmt.Sprintf("  %s %-28s %-26s %s\n",
				marker, truncate(a.Name, 28), truncate(instance, 26),
				styleMuted.Render("last seen "+humanAgo(now, a.ReceivedAt))))
		}
	}
	b.WriteString("\n")

	// Timeline — the only history the server keeps.
	notes := noteEvents(timeline)
	b.WriteString(divider(fmt.Sprintf("Timeline (%d events, %d notes)", len(timeline), len(notes)), width))
	if len(timeline) == 0 {
		b.WriteString(styleMuted.Render("  Nothing recorded yet.\n"))
	} else {
		noteIndex := 0
		for _, e := range timeline {
			when := styleMuted.Render(humanAgo(now, e.CreatedAt))
			if e.Type != api.EventNote {
				b.WriteString(fmt.Sprintf("  %-52s %s\n", eventLabel(e), when))
				continue
			}
			marker := "  "
			author := styleBold.Render(e.Username)
			if noteIndex == cursor {
				marker = styleSelected.Render("> ")
				author = styleSelected.Render(e.Username)
			}
			b.WriteString(fmt.Sprintf("%s%-50s %s\n", marker, author+" wrote", when))
			b.WriteString("    " + e.Detail + "\n")
			noteIndex++
		}
	}

	return b.String()
}

// incidentStatusLabel is the headline badge for an incident.
func incidentStatusLabel(inc api.Incident) string {
	switch inc.Status {
	case api.StatusTriggered:
		if inc.IsSnoozed() {
			return "● TRIGGERED (snoozed)"
		}
		return "● TRIGGERED"
	case api.StatusAcknowledged:
		if inc.IsSnoozed() {
			return "◐ ACKNOWLEDGED (snoozed)"
		}
		return "◐ ACKNOWLEDGED"
	case api.StatusResolved:
		return "✓ RESOLVED"
	default:
		return strings.ToUpper(inc.Status)
	}
}

// eventLabel renders one timeline entry as a sentence. Unrecognised types fall
// back to their raw name rather than vanishing — the server may add more.
func eventLabel(e api.IncidentEvent) string {
	who := e.Username
	switch e.Type {
	case api.EventTriggered:
		return "  Incident opened"
	case api.EventAlertAdded:
		if e.AlertID != nil {
			return fmt.Sprintf("  Alert #%d joined", *e.AlertID)
		}
		return "  Alert joined"
	case api.EventAlertResolved:
		if e.AlertID != nil {
			return fmt.Sprintf("  Alert #%d resolved", *e.AlertID)
		}
		return "  Alert resolved"
	case api.EventAcknowledged:
		return "  Acknowledged by " + who
	case api.EventUnacknowledged:
		return "  Acknowledgement cleared by " + who
	case api.EventAssigned:
		// On an assigned event the user is the assignee, not the actor.
		return "  Assigned to " + who
	case api.EventSnoozed:
		if e.Detail != "" {
			return "  Snoozed until " + e.Detail
		}
		return "  Snoozed"
	case api.EventUnsnoozed:
		return "  Snooze cleared by " + who
	case api.EventResolved:
		if who != "" {
			return "  Resolved by " + who
		}
		return "  Resolved (all alerts stopped firing)"
	default:
		label := "  " + e.Type
		if e.Detail != "" {
			label += " · " + e.Detail
		}
		return label
	}
}

func buildAlertDetailContent(alert api.Alert, width int) string {
	now := time.Now()
	var b strings.Builder
	contentW := width - 4

	name := styleAlertName.Render(alert.Name)
	var statusStr string
	if alert.Status == "firing" {
		statusStr = styleFiring.Render("● FIRING")
	} else {
		label := "✓ RESOLVED"
		if alert.ResolutionSource != nil {
			label += " · " + *alert.ResolutionSource
		}
		statusStr = styleResolved.Render(label)
	}
	gap := contentW - lipgloss.Width(name) - lipgloss.Width(statusStr)
	if gap < 1 {
		gap = 1
	}
	b.WriteString("\n  " + name + strings.Repeat(" ", gap) + statusStr + "\n\n")

	b.WriteString(fmt.Sprintf("  Started:    %s  (%s)\n",
		alert.StartsAt.UTC().Format("2006-01-02 15:04 UTC"), humanAgo(now, alert.StartsAt)))
	b.WriteString(fmt.Sprintf("  Last Seen:  %s  (%s)\n",
		alert.ReceivedAt.UTC().Format("2006-01-02 15:04 UTC"), humanAgo(now, alert.ReceivedAt)))
	if alert.EndsAt != nil {
		b.WriteString(fmt.Sprintf("  Ended:      %s\n", alert.EndsAt.UTC().Format("2006-01-02 15:04 UTC")))
	}
	if alert.GeneratorURL != "" {
		b.WriteString(fmt.Sprintf("  Source:     %s\n", truncate(alert.GeneratorURL, contentW-12)))
	}
	if alert.IncidentID != nil {
		b.WriteString(fmt.Sprintf("  Incident:   %s   %s\n",
			styleBold.Render(fmt.Sprintf("#%d", *alert.IncidentID)),
			styleMuted.Render("press i to open it")))
	} else {
		b.WriteString(styleMuted.Render("  Incident:   none\n"))
	}
	b.WriteString("\n")

	if len(alert.Labels) > 0 {
		b.WriteString(divider("Labels", width))
		for _, k := range sortedKeys(alert.Labels) {
			b.WriteString(fmt.Sprintf("  %-22s %s\n", k, truncate(alert.Labels[k], contentW-24)))
		}
		b.WriteString("\n")
	}

	if len(alert.Annotations) > 0 {
		b.WriteString(divider("Annotations", width))
		for _, k := range sortedKeys(alert.Annotations) {
			b.WriteString(fmt.Sprintf("  %-22s %s\n", k, truncate(alert.Annotations[k], contentW-24)))
		}
		b.WriteString("\n")
	}

	// Alerts carry no workflow state: it all lives on the incident.
	b.WriteString(divider("", width))
	b.WriteString(styleMuted.Render(
		"  Alerts are read-only — acknowledge, assign, note and resolve on the incident.\n"))

	return b.String()
}

func buildStatsContent(incidents *api.IncidentStats, top []api.TopAlert, byHour []api.HourStat, byDay []api.DayStat, width int) string {
	barWidth := width/2 - 10
	if barWidth < 8 {
		barWidth = 8
	}
	if barWidth > 40 {
		barWidth = 40
	}

	var b strings.Builder
	b.WriteString("\n")

	// Response times first: they are what a rota is actually judged on.
	b.WriteString(divider("Incident Response", width))
	if incidents == nil {
		b.WriteString(styleMuted.Render("  No data.\n"))
	} else {
		b.WriteString(fmt.Sprintf("  %-28s %s\n", "Incidents total",
			styleBold.Render(fmt.Sprintf("%d", incidents.Total))))
		b.WriteString(fmt.Sprintf("  %-28s %s\n", "Triggered",
			styleTriggered.Render(fmt.Sprintf("%d", incidents.Triggered))))
		b.WriteString(fmt.Sprintf("  %-28s %s\n", "Acknowledged",
			styleAcknowledged.Render(fmt.Sprintf("%d", incidents.Acknowledged))))
		b.WriteString(fmt.Sprintf("  %-28s %s\n", "Resolved",
			styleResolved.Render(fmt.Sprintf("%d", incidents.Resolved))))
		b.WriteString(fmt.Sprintf("  %-28s %s\n", "Mean time to acknowledge",
			styleBold.Render(humanSeconds(incidents.MTTASeconds))))
		b.WriteString(fmt.Sprintf("  %-28s %s\n", "Mean time to resolve",
			styleBold.Render(humanSeconds(incidents.MTTRSeconds))))
		if incidents.MTTASeconds == nil || incidents.MTTRSeconds == nil {
			b.WriteString(styleMuted.Render("  (— means nothing has been acknowledged or resolved yet)\n"))
		}
	}
	b.WriteString("\n")

	b.WriteString(divider("Top Alerts", width))
	if len(top) == 0 {
		b.WriteString(styleMuted.Render("  No data.\n"))
	} else {
		maxCount := top[0].Count
		for i, a := range top {
			bar := styleResolved.Render(strings.Repeat("█", renderBarWidth(a.Count, maxCount, barWidth)))
			b.WriteString(fmt.Sprintf("  %2d. %-30s %s %d\n", i+1, truncate(a.Name, 30), bar, a.Count))
		}
	}
	b.WriteString("\n")

	b.WriteString(divider("Alerts by Hour (UTC)", width))
	if len(byHour) > 0 {
		maxCount := 0
		for _, h := range byHour {
			if h.Count > maxCount {
				maxCount = h.Count
			}
		}
		for _, h := range byHour {
			bar := styleFiring.Render(strings.Repeat("█", renderBarWidth(h.Count, maxCount, barWidth)))
			b.WriteString(fmt.Sprintf("  %2dh  %-*s %d\n", h.Hour, barWidth, bar, h.Count))
		}
	} else {
		b.WriteString(styleMuted.Render("  No data.\n"))
	}
	b.WriteString("\n")

	b.WriteString(divider("Alerts by Day", width))
	if len(byDay) > 0 {
		maxCount := 0
		for _, d := range byDay {
			if d.Count > maxCount {
				maxCount = d.Count
			}
		}
		for _, d := range byDay {
			bar := styleAccent.Render(strings.Repeat("█", renderBarWidth(d.Count, maxCount, barWidth)))
			b.WriteString(fmt.Sprintf("  %-4s  %-*s %d\n", d.DayName[:3], barWidth, bar, d.Count))
		}
	} else {
		b.WriteString(styleMuted.Render("  No data.\n"))
	}

	return b.String()
}

// ── Users ──────────────────────────────────────────────────────────────────

func (m Model) renderUsers() string {
	if m.usersLoading {
		return "\n" + styleMuted.Render("  Loading users…")
	}
	if len(m.users) == 0 {
		return "\n" + styleMuted.Render("  No users found. Press n to create one.")
	}
	return "\n" + m.userManageTable.View()
}

func (m Model) renderUserCreate() string {
	header := "\n  " + styleBold.Render("Create new user") + "\n\n"
	usernameLabel := "  Username:  "
	emailLabel := "  Email:     "
	if m.userFormFocus == 0 {
		usernameLabel = styleSelected.Render("  Username:  ")
	} else {
		emailLabel = styleSelected.Render("  Email:     ")
	}
	return header +
		usernameLabel + m.userFormInputs[0].View() + "\n" +
		emailLabel + m.userFormInputs[1].View() + "\n"
}

func (m Model) renderAPIKeyMenu() string {
	header := fmt.Sprintf("\n  API keys for %s\n", styleBold.Render(m.selectedUser.Username))
	warning := styleMuted.Render("  Keys cannot be listed — only new keys can be created,\n  or existing ones revoked by their integer ID.\n")
	options := "\n" +
		styleAccent.Render("  n") + "  · create a new API key\n" +
		styleAccent.Render("  r") + "  · revoke a key by ID\n"
	return header + "\n" + warning + options
}

func (m Model) renderAPIKeyCreate() string {
	header := fmt.Sprintf("\n  New API key for %s\n\n", styleBold.Render(m.selectedUser.Username))
	label := styleSelected.Render("  Key name:  ")
	return header + label + m.apiKeyNameInput.View() + "\n"
}

func (m Model) renderAPIKeyReveal() string {
	sep := styleMuted.Render(strings.Repeat("─", m.width))
	warn := styleError.Render("  !! COPY NOW — this key will NEVER be shown again !!")
	nameLine := fmt.Sprintf("  Key name:  %s", styleBold.Render(m.revealedAPIKey.Name))
	idLine := fmt.Sprintf("  Key ID:    %s   %s",
		styleBold.Render(fmt.Sprintf("%d", m.revealedAPIKey.ID)),
		styleMuted.Render("(save this — needed for future revocation)"))

	keyLine := styleResolved.Render("  " + m.revealedAPIKey.Key)

	return "\n" + sep + "\n\n" +
		warn + "\n\n" +
		nameLine + "\n" +
		idLine + "\n\n" +
		styleMuted.Render("  Key value:") + "\n" +
		keyLine + "\n\n" +
		sep + "\n"
}

func (m Model) renderAPIKeyRevokeByID() string {
	header := fmt.Sprintf("\n  Revoke API key for %s\n", styleBold.Render(m.selectedUser.Username))
	hint := styleMuted.Render("  Enter the integer key ID (shown when the key was created).\n")
	label := styleSelected.Render("  Key ID:  ")
	return header + "\n" + hint + "\n" + label + m.apiKeyRevokeInput.View() + "\n"
}

// ── Helpers ────────────────────────────────────────────────────────────────

func divider(title string, width int) string {
	prefix := "── "
	if title != "" {
		prefix += title + " "
	}
	remaining := width - len(prefix) - 2
	if remaining > 0 {
		prefix += strings.Repeat("─", remaining)
	}
	return styleMuted.Render(prefix) + "\n"
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func renderBarWidth(count, maxCount, maxWidth int) int {
	if maxCount == 0 {
		return 0
	}
	w := count * maxWidth / maxCount
	if w == 0 && count > 0 {
		w = 1
	}
	return w
}

func truncate(s string, max int) string {
	if max < 1 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
