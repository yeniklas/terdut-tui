package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) newRequest(method, path string) (*http.Request, error) {
	req, err := http.NewRequest(method, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error != "" {
			return fmt.Errorf("server returned %d: %s", resp.StatusCode, e.Error)
		}
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// ListAlerts fetches alerts. status may be "firing", "resolved", or "" for all.
// Set archived=true to fetch only archived alerts; false returns only non-archived.
//
// Alerts are read-only on the server — there is nothing to acknowledge or
// archive here. This is the raw feed, useful for checking what Alertmanager is
// actually sending; the work queue is ListIncidents.
func (c *Client) ListAlerts(status string, archived bool, limit int) ([]Alert, error) {
	q := url.Values{}
	if status != "" {
		q.Set("status", status)
	}
	if archived {
		q.Set("archived", "true")
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	path := "/api/alerts"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	req, err := c.newRequest(http.MethodGet, path)
	if err != nil {
		return nil, err
	}
	var alerts []Alert
	return alerts, c.do(req, &alerts)
}

// GetAlertStats fetches aggregate alert counts.
func (c *Client) GetAlertStats() (*AlertStats, error) {
	req, err := c.newRequest(http.MethodGet, "/api/stats/alerts")
	if err != nil {
		return nil, err
	}
	var stats AlertStats
	return &stats, c.do(req, &stats)
}

func (c *Client) newRequestWithBody(method, path string, body any) (*http.Request, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func (c *Client) GetAlert(id int64) (*Alert, error) {
	req, err := c.newRequest(http.MethodGet, fmt.Sprintf("/api/alerts/%d", id))
	if err != nil {
		return nil, err
	}
	var alert Alert
	return &alert, c.do(req, &alert)
}

// ── Incidents ──────────────────────────────────────────────────────────────

// ListIncidents fetches the work queue. status may be "triggered",
// "acknowledged", "resolved", or "" for the server default of open incidents
// only. archived and snoozed each switch the list to that set rather than
// adding to it, matching the server's filters.
func (c *Client) ListIncidents(status string, archived, snoozed bool, limit int) ([]Incident, error) {
	q := url.Values{}
	if status != "" {
		q.Set("status", status)
	}
	if archived {
		q.Set("archived", "true")
	}
	if snoozed {
		q.Set("snoozed", "true")
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	path := "/api/incidents"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	req, err := c.newRequest(http.MethodGet, path)
	if err != nil {
		return nil, err
	}
	var incidents []Incident
	return incidents, c.do(req, &incidents)
}

// GetIncident returns one incident with its member alerts inline.
func (c *Client) GetIncident(id int64) (*Incident, error) {
	req, err := c.newRequest(http.MethodGet, fmt.Sprintf("/api/incidents/%d", id))
	if err != nil {
		return nil, err
	}
	var incident Incident
	return &incident, c.do(req, &incident)
}

func (c *Client) GetIncidentTimeline(id int64) ([]IncidentEvent, error) {
	req, err := c.newRequest(http.MethodGet, fmt.Sprintf("/api/incidents/%d/timeline", id))
	if err != nil {
		return nil, err
	}
	var events []IncidentEvent
	return events, c.do(req, &events)
}

func (c *Client) AcknowledgeIncident(id int64) (*Incident, error) {
	req, err := c.newRequest(http.MethodPost, fmt.Sprintf("/api/incidents/%d/acknowledge", id))
	if err != nil {
		return nil, err
	}
	var incident Incident
	return &incident, c.do(req, &incident)
}

func (c *Client) UnacknowledgeIncident(id int64) error {
	req, err := c.newRequest(http.MethodDelete, fmt.Sprintf("/api/incidents/%d/acknowledge", id))
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

// ResolveIncident closes an incident by hand. This is terminal on the server: a
// later occurrence in the same group opens a new incident rather than reopening
// this one, and if the alert underneath never stops firing the incident stays
// closed. Use SnoozeIncident for "not now".
func (c *Client) ResolveIncident(id int64) (*Incident, error) {
	req, err := c.newRequest(http.MethodPost, fmt.Sprintf("/api/incidents/%d/resolve", id))
	if err != nil {
		return nil, err
	}
	var incident Incident
	return &incident, c.do(req, &incident)
}

func (c *Client) AssignIncident(id, userID int64) (*Incident, error) {
	body := struct {
		UserID int64 `json:"user_id"`
	}{UserID: userID}
	req, err := c.newRequestWithBody(http.MethodPost, fmt.Sprintf("/api/incidents/%d/assign", id), body)
	if err != nil {
		return nil, err
	}
	var incident Incident
	return &incident, c.do(req, &incident)
}

// SnoozeIncident hides an incident from the default queue for a duration,
// without closing it.
func (c *Client) SnoozeIncident(id int64, duration string) (*Incident, error) {
	body := struct {
		Duration string `json:"duration"`
	}{Duration: duration}
	req, err := c.newRequestWithBody(http.MethodPost, fmt.Sprintf("/api/incidents/%d/snooze", id), body)
	if err != nil {
		return nil, err
	}
	var incident Incident
	return &incident, c.do(req, &incident)
}

func (c *Client) UnsnoozeIncident(id int64) error {
	req, err := c.newRequest(http.MethodDelete, fmt.Sprintf("/api/incidents/%d/snooze", id))
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *Client) ArchiveIncident(id int64) (*Incident, error) {
	req, err := c.newRequest(http.MethodPost, fmt.Sprintf("/api/incidents/%d/archive", id))
	if err != nil {
		return nil, err
	}
	var incident Incident
	return &incident, c.do(req, &incident)
}

func (c *Client) UnarchiveIncident(id int64) error {
	req, err := c.newRequest(http.MethodDelete, fmt.Sprintf("/api/incidents/%d/archive", id))
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

// AddNote appends a note to the incident's timeline.
func (c *Client) AddNote(incidentID int64, content string) (*IncidentEvent, error) {
	req, err := c.newRequestWithBody(http.MethodPost,
		fmt.Sprintf("/api/incidents/%d/notes", incidentID), map[string]string{"content": content})
	if err != nil {
		return nil, err
	}
	var event IncidentEvent
	return &event, c.do(req, &event)
}

// DeleteNote removes one of your own notes. Only notes are deletable — the rest
// of the timeline is a record of what happened.
func (c *Client) DeleteNote(incidentID, eventID int64) error {
	req, err := c.newRequest(http.MethodDelete,
		fmt.Sprintf("/api/incidents/%d/notes/%d", incidentID, eventID))
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *Client) GetIncidentStats() (*IncidentStats, error) {
	req, err := c.newRequest(http.MethodGet, "/api/stats/incidents")
	if err != nil {
		return nil, err
	}
	var stats IncidentStats
	return &stats, c.do(req, &stats)
}

// ── Statistics ─────────────────────────────────────────────────────────────

func (c *Client) GetTopAlerts(limit int) ([]TopAlert, error) {
	req, err := c.newRequest(http.MethodGet, fmt.Sprintf("/api/stats/alerts/top?limit=%d", limit))
	if err != nil {
		return nil, err
	}
	var result []TopAlert
	return result, c.do(req, &result)
}

func (c *Client) GetStatsByHour() ([]HourStat, error) {
	req, err := c.newRequest(http.MethodGet, "/api/stats/alerts/by-hour")
	if err != nil {
		return nil, err
	}
	var result []HourStat
	return result, c.do(req, &result)
}

func (c *Client) GetStatsByDay() ([]DayStat, error) {
	req, err := c.newRequest(http.MethodGet, "/api/stats/alerts/by-day")
	if err != nil {
		return nil, err
	}
	var result []DayStat
	return result, c.do(req, &result)
}

func (c *Client) GetSchedule(from, to string) ([]ScheduleEntry, error) {
	req, err := c.newRequest(http.MethodGet, fmt.Sprintf("/api/schedule?from=%s&to=%s", from, to))
	if err != nil {
		return nil, err
	}
	var entries []ScheduleEntry
	return entries, c.do(req, &entries)
}

// GetCurrentOnCall returns today's on-call entry, or nil if nobody is scheduled.
func (c *Client) GetCurrentOnCall() (*ScheduleEntry, error) {
	req, err := c.newRequest(http.MethodGet, "/api/schedule/current")
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error != "" {
			return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, e.Error)
		}
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}
	var entry ScheduleEntry
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// AssignSchedule puts one user on call for the given dates.
//
// The server holds one person per day and refuses a date somebody already has,
// so replace is what takes a shift off its current holder. It is all-or-nothing
// either way: a week of free and taken days moves as a unit, or not at all.
func (c *Client) AssignSchedule(userID int64, dates []string, replace bool) ([]ScheduleEntry, error) {
	body := struct {
		UserID  int64    `json:"user_id"`
		Dates   []string `json:"dates"`
		Replace bool     `json:"replace,omitempty"`
	}{UserID: userID, Dates: dates, Replace: replace}
	req, err := c.newRequestWithBody(http.MethodPost, "/api/schedule", body)
	if err != nil {
		return nil, err
	}
	var entries []ScheduleEntry
	return entries, c.do(req, &entries)
}

func (c *Client) DeleteScheduleEntry(id int64) error {
	req, err := c.newRequest(http.MethodDelete, fmt.Sprintf("/api/schedule/%d", id))
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *Client) ListUsers() ([]User, error) {
	req, err := c.newRequest(http.MethodGet, "/api/users")
	if err != nil {
		return nil, err
	}
	var users []User
	return users, c.do(req, &users)
}

func (c *Client) CreateUser(username, email string) (*User, error) {
	body := struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	}{Username: username, Email: email}
	req, err := c.newRequestWithBody(http.MethodPost, "/api/users", body)
	if err != nil {
		return nil, err
	}
	var user User
	return &user, c.do(req, &user)
}

// SetUserNotifyTarget points a user's push notifications at an ntfy topic.
//
// An empty topic clears it: the server stores NULL, and that user's incidents
// page the shared fallback topic instead — which carries no Acknowledge button,
// because anyone subscribed to it could otherwise acknowledge as somebody else.
func (c *Client) SetUserNotifyTarget(userID int64, topic string) (*User, error) {
	body := struct {
		NtfyTopic string `json:"ntfy_topic"`
	}{NtfyTopic: topic}
	req, err := c.newRequestWithBody(http.MethodPut, fmt.Sprintf("/api/users/%d/notify", userID), body)
	if err != nil {
		return nil, err
	}
	var user User
	return &user, c.do(req, &user)
}

func (c *Client) DeleteUser(id int64) error {
	req, err := c.newRequest(http.MethodDelete, fmt.Sprintf("/api/users/%d", id))
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *Client) CreateAPIKey(userID int64, name string) (*APIKey, error) {
	body := struct {
		Name string `json:"name"`
	}{Name: name}
	req, err := c.newRequestWithBody(http.MethodPost, fmt.Sprintf("/api/users/%d/api-keys", userID), body)
	if err != nil {
		return nil, err
	}
	var key APIKey
	return &key, c.do(req, &key)
}

func (c *Client) DeleteAPIKey(userID, keyID int64) error {
	req, err := c.newRequest(http.MethodDelete, fmt.Sprintf("/api/users/%d/api-keys/%d", userID, keyID))
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

// HealthCheck calls GET /healthz (unauthenticated path, no auth needed but we send it anyway).
func (c *Client) HealthCheck() error {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: %s", resp.Status)
	}
	return nil
}
