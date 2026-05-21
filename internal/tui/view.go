package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/yeniklas/terdut-tui/internal/api"
)

var sectionNames = []string{"Alerts", "Schedule", "Users"}

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
	gap := m.width - lipgloss.Width(title) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return title + strings.Repeat(" ", gap) + right
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
	case modeDetail:
		return m.renderDetail()
	case modeComment:
		return m.renderCommentCompose()
	case modeConfirmDelete:
		if m.confirmTarget == confirmDeleteComment {
			return m.renderDetail()
		}
		return m.renderSchedule()
	case modeStats:
		return m.renderStats()
	case modeScheduleUserPicker:
		return m.renderUserPicker()
	default:
		return m.renderDashboard()
	}
}

func (m Model) renderFooter() string {
	switch m.mode {
	case modeDetail:
		actions := styleFooter.Render("  a·ack  A·unack  c·comment  [/]·select  d·del  s·assign  S·stats  esc·back")
		if m.statusMsg != "" {
			return styleStatus.Render("  "+m.statusMsg) + "\n" + actions
		}
		return "\n" + actions

	case modeComment:
		return "\n" + styleFooter.Render("  enter·submit  esc·cancel")

	case modeConfirmDelete:
		var deleteDesc string
		switch m.confirmTarget {
		case confirmDeleteComment:
			if m.commentCursor >= 0 && m.commentCursor < len(m.comments) {
				deleteDesc = fmt.Sprintf("comment by %s", m.comments[m.commentCursor].Username)
			} else {
				deleteDesc = "comment"
			}
		case confirmDeleteScheduleEntry:
			if m.pendingDeleteEntry != nil {
				deleteDesc = fmt.Sprintf("on-call for %s (%s)", m.pendingDeleteEntry.Date, m.pendingDeleteEntry.Username)
			} else {
				deleteDesc = "schedule entry"
			}
		}
		return "\n" + styleError.Render(fmt.Sprintf("  Delete %s? [y/N]", deleteDesc))

	case modeStats:
		if m.statusMsg != "" {
			return styleStatus.Render("  "+m.statusMsg) + "\n" + styleFooter.Render("  esc·back")
		}
		return "\n" + styleFooter.Render("  esc·back")

	case modeScheduleUserPicker:
		return "\n" + styleFooter.Render("  j/k·navigate  enter·select  esc·cancel")

	default:
		if m.activeSection == sectionSchedule {
			actions := styleFooter.Render("  +·assign  d·del  ←/→·shift week  tab·section  r·refresh  q·quit")
			if m.statusMsg != "" {
				return styleStatus.Render("  "+m.statusMsg) + "\n" + actions
			}
			return "\n" + actions
		}
		helpView := styleFooter.Render(m.help.ShortHelpView(m.keys.ShortHelp()))
		if m.statusMsg != "" {
			status := styleStatus.Render(m.statusMsg)
			gap := m.width - lipgloss.Width(status) - lipgloss.Width(helpView)
			if gap < 0 {
				gap = 0
			}
			return "\n" + status + strings.Repeat(" ", gap) + helpView
		}
		return "\n" + helpView
	}
}

// ── Dashboard ──────────────────────────────────────────────────────────────

func (m Model) renderDashboard() string {
	switch m.activeSection {
	case sectionAlerts:
		return m.renderAlerts()
	case sectionSchedule:
		return m.renderSchedule()
	case sectionUsers:
		return "\n" + styleMuted.Render("  User management — coming in Stage 5")
	}
	return ""
}

// ── Schedule ───────────────────────────────────────────────────────────────

func (m Model) renderSchedule() string {
	if m.scheduleLoading {
		return "\n" + styleMuted.Render("  Loading schedule…")
	}

	// On-call header
	var onCallLine string
	if m.currentOnCall != nil {
		onCallLine = fmt.Sprintf("  On-call today: %s",
			styleAlertName.Render(m.currentOnCall.Username))
	} else {
		onCallLine = styleMuted.Render("  On-call today: nobody scheduled")
	}

	// Window label
	from := m.scheduleWindow
	to := m.scheduleWindow.AddDate(0, 0, 13)
	windowLabel := styleMuted.Render(fmt.Sprintf("  %s — %s",
		from.Format("Jan 02"), to.Format("Jan 02, 2006")))

	gap := m.width - lipgloss.Width(onCallLine) - lipgloss.Width(windowLabel)
	if gap < 0 {
		gap = 0
	}
	header := "\n" + onCallLine + strings.Repeat(" ", gap) + windowLabel + "\n"
	return header + m.scheduleTable.View()
}

func (m Model) renderUserPicker() string {
	if m.usersLoading {
		return "\n" + styleMuted.Render("  Loading users…")
	}

	selectedDate := ""
	cursor := m.scheduleTable.Cursor()
	if cursor < len(m.scheduleDays) {
		d := m.scheduleDays[cursor]
		if d.date.Format("2006-01-02") == time.Now().UTC().Format("2006-01-02") {
			selectedDate = "Today (" + d.date.Format("Mon") + ")"
		} else {
			selectedDate = d.date.Format("Jan 02 (Mon)")
		}
	}

	header := fmt.Sprintf("\n  Assign on-call for %s — select a user:\n\n",
		styleBold.Render(selectedDate))
	return header + m.userPickerTable.View()
}

func (m Model) renderAlerts() string {
	statsBar := m.renderStatsBar()
	var content string
	if m.loading && len(m.alerts) == 0 {
		content = styleMuted.Render("  Loading alerts…")
	} else if len(m.alerts) == 0 {
		label := m.filterStatus
		if label == "" {
			label = "all"
		}
		content = styleMuted.Render(fmt.Sprintf("  No %s alerts.", label))
	} else {
		content = m.alertTable.View()
	}
	return lipgloss.JoinVertical(lipgloss.Left, statsBar, content)
}

func (m Model) renderStatsBar() string {
	total, firing, resolved := 0, 0, 0
	if m.stats != nil {
		total = m.stats.Total
		firing = m.stats.Firing
		resolved = m.stats.Resolved
	}
	filterLabel := m.filterStatus
	if filterLabel == "" {
		filterLabel = "all"
	}
	left := fmt.Sprintf("  Total: %d  %s  %s",
		total,
		styleFiring.Render(fmt.Sprintf("Firing: %d", firing)),
		styleResolved.Render(fmt.Sprintf("Resolved: %d", resolved)),
	)
	right := styleMuted.Render(fmt.Sprintf("filter: %s  [f]  ", filterLabel))
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return left + strings.Repeat(" ", gap) + right
}

// ── Detail ─────────────────────────────────────────────────────────────────

func (m Model) renderDetail() string {
	if m.detailLoading {
		return "\n" + styleMuted.Render("  Loading alert details…")
	}
	return m.detailViewport.View()
}

func (m Model) renderCommentCompose() string {
	vp := m.detailViewport.View()
	sep := styleMuted.Render(strings.Repeat("─", m.width))
	prompt := styleHeader.Render("Comment: ") + m.commentInput.View()
	return vp + "\n" + sep + "\n" + prompt
}

// ── Stats ──────────────────────────────────────────────────────────────────

func (m Model) renderStats() string {
	if m.statsLoading {
		return "\n" + styleMuted.Render("  Loading statistics…")
	}
	return m.statsViewport.View()
}

// ── Content builders ───────────────────────────────────────────────────────

func buildDetailContent(alert api.Alert, comments []api.Comment, cursor, width int) string {
	now := time.Now()
	var b strings.Builder
	contentW := width - 4

	// Name + status header
	name := styleAlertName.Render(alert.Name)
	var statusStr string
	if alert.Status == "firing" {
		statusStr = styleFiring.Render("● FIRING")
	} else {
		statusStr = styleResolved.Render("✓ RESOLVED")
	}
	nameGap := contentW - lipgloss.Width(name) - lipgloss.Width(statusStr)
	if nameGap < 1 {
		nameGap = 1
	}
	b.WriteString("\n  " + name + strings.Repeat(" ", nameGap) + statusStr + "\n\n")

	// Timeline
	b.WriteString(fmt.Sprintf("  Started:  %s  (%s)\n",
		alert.StartsAt.UTC().Format("2006-01-02 15:04 UTC"), humanAgo(now, alert.StartsAt)))
	if alert.EndsAt != nil {
		b.WriteString(fmt.Sprintf("  Ended:    %s\n", alert.EndsAt.UTC().Format("2006-01-02 15:04 UTC")))
	}
	if alert.GeneratorURL != "" {
		url := alert.GeneratorURL
		if len(url) > contentW-12 {
			url = url[:contentW-15] + "…"
		}
		b.WriteString(fmt.Sprintf("  Source:   %s\n", url))
	}
	b.WriteString("\n")

	// Labels
	if len(alert.Labels) > 0 {
		b.WriteString(divider("Labels", width))
		for _, k := range sortedKeys(alert.Labels) {
			v := alert.Labels[k]
			if len(v) > contentW-24 {
				v = v[:contentW-27] + "…"
			}
			b.WriteString(fmt.Sprintf("  %-22s %s\n", k, v))
		}
		b.WriteString("\n")
	}

	// Annotations
	if len(alert.Annotations) > 0 {
		b.WriteString(divider("Annotations", width))
		for _, k := range sortedKeys(alert.Annotations) {
			v := alert.Annotations[k]
			if len(v) > contentW-24 {
				v = v[:contentW-27] + "…"
			}
			b.WriteString(fmt.Sprintf("  %-22s %s\n", k, v))
		}
		b.WriteString("\n")
	}

	// Acknowledgement
	b.WriteString(divider("Acknowledgement", width))
	if alert.AcknowledgedByID != nil {
		ackAt := ""
		if alert.AcknowledgedAt != nil {
			ackAt = "  at " + alert.AcknowledgedAt.UTC().Format("2006-01-02 15:04 UTC")
		}
		b.WriteString(styleResolved.Render(fmt.Sprintf("  ✓ Acknowledged by %s%s\n", alert.AcknowledgedBy, ackAt)))
	} else {
		b.WriteString(styleMuted.Render("  Not acknowledged\n"))
	}
	b.WriteString("\n")

	// Comments
	b.WriteString(divider(fmt.Sprintf("Comments (%d)", len(comments)), width))
	if len(comments) == 0 {
		b.WriteString(styleMuted.Render("  No comments yet.\n"))
	} else {
		for i, c := range comments {
			prefix := "  "
			authorLine := fmt.Sprintf("%s%s  •  %s", prefix, styleBold.Render(c.Username), humanAgo(now, c.CreatedAt))
			if i == cursor {
				authorLine = styleSelected.Render("> ") + styleBold.Render(c.Username) +
					styleMuted.Render(fmt.Sprintf("  •  %s", humanAgo(now, c.CreatedAt)))
			}
			b.WriteString(authorLine + "\n")
			b.WriteString("  " + c.Content + "\n\n")
		}
	}

	return b.String()
}

func buildStatsContent(top []api.TopAlert, byHour []api.HourStat, byDay []api.DayStat, width int) string {
	barWidth := width/2 - 10
	if barWidth < 8 {
		barWidth = 8
	}
	if barWidth > 40 {
		barWidth = 40
	}

	var b strings.Builder
	b.WriteString("\n")

	// Top alerts
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

	// By hour
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

	// By day
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

// ── Helpers ────────────────────────────────────────────────────────────────

func divider(title string, width int) string {
	prefix := "── " + title + " "
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
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
