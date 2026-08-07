package api

import "time"

// Alert is the server's record of what Alertmanager said. It is read-only:
// acknowledging, assigning, noting and resolving all happen on the Incident an
// alert belongs to.
type Alert struct {
	ID           int64             `json:"id"`
	Fingerprint  string            `json:"fingerprint"`
	Name         string            `json:"name"`
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"starts_at"`
	EndsAt       *time.Time        `json:"ends_at"`
	GeneratorURL string            `json:"generator_url"`
	ReceivedAt   time.Time         `json:"received_at"`
	ArchivedAt   *time.Time        `json:"archived_at,omitempty"`

	// IncidentID is the most recent incident this alert belongs to. An alert row
	// is reused across occurrences of the same fingerprint, so it belongs to a
	// series of incidents over its life and this is only the newest.
	IncidentID *int64 `json:"incident_id,omitempty"`

	// ResolutionSource records why a resolved alert left the firing state:
	// "alertmanager" for a real resolved webhook, "expiry" when the server
	// inferred it after the alert stopped being refreshed.
	ResolutionSource *string `json:"resolution_source,omitempty"`
}

// Incident statuses.
const (
	StatusTriggered    = "triggered"
	StatusAcknowledged = "acknowledged"
	StatusResolved     = "resolved"
)

// Incident is the work item: what a person acknowledges, assigns, snoozes,
// discusses and resolves. Many alerts map to one incident, correlated by the
// groupKey Alertmanager computed from the operator's group_by configuration.
type Incident struct {
	ID          int64             `json:"id"`
	GroupKey    string            `json:"group_key"`
	Title       string            `json:"title"`
	GroupLabels map[string]string `json:"group_labels"`
	Status      string            `json:"status"`

	// Severity is a high-water mark across the incident's alerts, never lowered,
	// so a resolved incident still says how bad it got.
	Severity string `json:"severity,omitempty"`

	TriggeredAt time.Time `json:"triggered_at"`

	AcknowledgedByID *int64     `json:"acknowledged_by_id,omitempty"`
	AcknowledgedBy   string     `json:"acknowledged_by,omitempty"`
	AcknowledgedAt   *time.Time `json:"acknowledged_at,omitempty"`

	AssignedToID *int64 `json:"assigned_to_id,omitempty"`
	AssignedTo   string `json:"assigned_to,omitempty"`

	SnoozedUntil *time.Time `json:"snoozed_until,omitempty"`

	ResolvedAt *time.Time `json:"resolved_at,omitempty"`

	// ResolutionSource is "alerts" when every alert stopped firing, or "manual"
	// when a person closed it. Treat the value set as open.
	ResolutionSource *string `json:"resolution_source,omitempty"`

	ArchivedAt *time.Time `json:"archived_at,omitempty"`

	// Alerts is populated by GET /api/incidents/{id} only.
	Alerts []Alert `json:"alerts,omitempty"`
}

// IsSnoozed reports whether the incident is currently quietened. A snooze
// expires by falling into the past; nothing on the server sweeps it.
func (i Incident) IsSnoozed() bool {
	return i.SnoozedUntil != nil && i.SnoozedUntil.After(time.Now())
}

// IsOpen reports whether the incident is still work in progress.
func (i Incident) IsOpen() bool { return i.ResolvedAt == nil }

// Incident timeline event types written by the server. New ones may be added,
// so render unrecognised types generically rather than dropping them.
const (
	EventTriggered      = "triggered"
	EventAlertAdded     = "alert_added"
	EventAlertResolved  = "alert_resolved"
	EventAcknowledged   = "acknowledged"
	EventUnacknowledged = "unacknowledged"
	EventAssigned       = "assigned"
	EventSnoozed        = "snoozed"
	EventUnsnoozed      = "unsnoozed"
	EventResolved       = "resolved"
	EventNote           = "note"

	// Written by the server's notifier from the delivery result, not at enqueue.
	// Detail carries the notification kind ("triggered", "reminder", "resolved"),
	// and on a failure the reason after it. An absent user means the page went to
	// the shared fallback topic rather than to a person.
	EventNotified     = "notified"
	EventNotifyFailed = "notify_failed"
)

// IncidentEvent is one entry in an incident's timeline. An empty Username means
// the server acted rather than a person. On an "assigned" event the user is the
// assignee, not the actor.
type IncidentEvent struct {
	ID         int64     `json:"id"`
	IncidentID int64     `json:"incident_id"`
	Type       string    `json:"type"`
	UserID     *int64    `json:"user_id,omitempty"`
	Username   string    `json:"username,omitempty"`
	AlertID    *int64    `json:"alert_id,omitempty"`
	Detail     string    `json:"detail,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type AlertStats struct {
	Total    int `json:"total"`
	Firing   int `json:"firing"`
	Resolved int `json:"resolved"`
}

// IncidentStats carries the queue counts plus mean time to acknowledge and to
// resolve. Both averages are nil until something has actually been acknowledged
// or resolved — that is "no data", not zero.
type IncidentStats struct {
	Total        int      `json:"total"`
	Triggered    int      `json:"triggered"`
	Acknowledged int      `json:"acknowledged"`
	Resolved     int      `json:"resolved"`
	MTTASeconds  *float64 `json:"mtta_seconds"`
	MTTRSeconds  *float64 `json:"mttr_seconds"`
}

type TopAlert struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type HourStat struct {
	Hour  int `json:"hour"`
	Count int `json:"count"`
}

type DayStat struct {
	Day     int    `json:"day"`
	DayName string `json:"day_name"`
	Count   int    `json:"count"`
}

type ScheduleEntry struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Username  string    `json:"username"`
	Date      string    `json:"date"` // YYYY-MM-DD
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`

	// NtfyTopic is where this user's push notifications go. Nil and empty mean
	// the same thing — no topic of their own — because the server stores a blank
	// string as NULL. Their incidents fall back to the server's shared fallback
	// topic, which carries no Acknowledge button.
	NtfyTopic *string `json:"ntfy_topic,omitempty"`
}

// Topic reads the user's ntfy topic, flattening the nil and empty cases the
// server treats alike.
func (u User) Topic() string {
	if u.NtfyTopic == nil {
		return ""
	}
	return *u.NtfyTopic
}

type APIKey struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	Name       string     `json:"name"`
	Key        string     `json:"key,omitempty"` // only present in create response
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}
