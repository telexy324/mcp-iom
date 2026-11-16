package model

import "time"

type Meta struct {
	Source        string    `json:"source"`
	RetrievedAt   time.Time `json:"retrieved_at"`
	Truncated     bool      `json:"truncated"`
	ReturnedCount int       `json:"returned_count"`
	TotalCount    *int      `json:"total_count"`
}
type Component struct {
	CIID string `json:"ci_id"`
	Type string `json:"type"`
	Name string `json:"name"`
	Host string `json:"host"`
}
type Dependency struct {
	Source   string `json:"source_ci_id"`
	Target   string `json:"target_ci_id"`
	Relation string `json:"relation"`
}
type SystemInfo struct {
	SystemID     string       `json:"system_id"`
	SystemName   string       `json:"system_name"`
	Components   []Component  `json:"components"`
	Dependencies []Dependency `json:"dependencies"`
	Meta         Meta         `json:"meta"`
}
type Change struct {
	ID          string    `json:"change_id"`
	SystemID    string    `json:"system_id"`
	RecordType  string    `json:"record_type"`
	CIIDs       []string  `json:"ci_ids"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Start       time.Time `json:"start_time"`
	End         time.Time `json:"end_time"`
	TimeType    string    `json:"time_type"`
	Status      string    `json:"status"`
}
type Alert struct {
	ID       string    `json:"alert_id"`
	SystemID string    `json:"system_id"`
	CIID     string    `json:"ci_id"`
	Name     string    `json:"name"`
	Severity string    `json:"severity"`
	First    time.Time `json:"first_seen"`
	Last     time.Time `json:"last_seen"`
	Count    int       `json:"count"`
	Summary  string    `json:"summary"`
}
type ChangeResult struct {
	SystemID string   `json:"system_id"`
	Changes  []Change `json:"changes"`
	Meta     Meta     `json:"meta"`
}
type AlertResult struct {
	SystemID string  `json:"system_id"`
	Alerts   []Alert `json:"alerts"`
	Meta     Meta    `json:"meta"`
}
type SystemInput struct {
	SystemID string `json:"system_id" jsonschema:"Business system identifier"`
}
type Query struct {
	SystemID string   `json:"system_id"`
	CIIDs    []string `json:"ci_ids,omitempty"`
	Start    string   `json:"start_time"`
	End      string   `json:"end_time"`
	Limit    int      `json:"limit,omitempty"`
}
