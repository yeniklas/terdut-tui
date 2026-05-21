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

// ListAlerts fetches alerts from the server. status may be "firing", "resolved", or "" for all.
func (c *Client) ListAlerts(status string, limit int) ([]Alert, error) {
	q := url.Values{}
	if status != "" {
		q.Set("status", status)
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

func (c *Client) AcknowledgeAlert(id int64) (*Alert, error) {
	req, err := c.newRequest(http.MethodPost, fmt.Sprintf("/api/alerts/%d/acknowledge", id))
	if err != nil {
		return nil, err
	}
	var alert Alert
	return &alert, c.do(req, &alert)
}

func (c *Client) UnacknowledgeAlert(id int64) error {
	req, err := c.newRequest(http.MethodDelete, fmt.Sprintf("/api/alerts/%d/acknowledge", id))
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *Client) GetComments(alertID int64) ([]Comment, error) {
	req, err := c.newRequest(http.MethodGet, fmt.Sprintf("/api/alerts/%d/comments", alertID))
	if err != nil {
		return nil, err
	}
	var comments []Comment
	return comments, c.do(req, &comments)
}

func (c *Client) AddComment(alertID int64, content string) (*Comment, error) {
	req, err := c.newRequestWithBody(http.MethodPost, fmt.Sprintf("/api/alerts/%d/comments", alertID), map[string]string{"content": content})
	if err != nil {
		return nil, err
	}
	var comment Comment
	return &comment, c.do(req, &comment)
}

func (c *Client) DeleteComment(alertID, commentID int64) error {
	req, err := c.newRequest(http.MethodDelete, fmt.Sprintf("/api/alerts/%d/comments/%d", alertID, commentID))
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

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
		var e struct{ Error string `json:"error"` }
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

func (c *Client) AssignSchedule(userID int64, dates []string) ([]ScheduleEntry, error) {
	body := struct {
		UserID int64    `json:"user_id"`
		Dates  []string `json:"dates"`
	}{UserID: userID, Dates: dates}
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
