package api

import "time"

type Alert struct {
	ID               int64             `json:"id"`
	Fingerprint      string            `json:"fingerprint"`
	Name             string            `json:"name"`
	Status           string            `json:"status"`
	Labels           map[string]string `json:"labels"`
	Annotations      map[string]string `json:"annotations"`
	StartsAt         time.Time         `json:"starts_at"`
	EndsAt           *time.Time        `json:"ends_at"`
	GeneratorURL     string            `json:"generator_url"`
	ReceivedAt       time.Time         `json:"received_at"`
	AcknowledgedByID *int64            `json:"acknowledged_by_id"`
	AcknowledgedBy   string            `json:"acknowledged_by"`
	AcknowledgedAt   *time.Time        `json:"acknowledged_at"`
}

type AlertStats struct {
	Total    int `json:"total"`
	Firing   int `json:"firing"`
	Resolved int `json:"resolved"`
}
