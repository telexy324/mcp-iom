package provider

import (
	"bank-ops-mcp/internal/model"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Mock struct {
	Systems []model.SystemInfo
	Changes []model.Change
	Alerts  []model.Alert
}

func NewMock(dir string) (*Mock, error) {
	m := &Mock{}
	for _, f := range []struct {
		name string
		dst  any
	}{{"cmdb.json", &m.Systems}, {"changes.json", &m.Changes}, {"alerts.json", &m.Alerts}} {
		b, e := os.ReadFile(filepath.Join(dir, f.name))
		if e != nil {
			return nil, e
		}
		if e = json.Unmarshal(b, f.dst); e != nil {
			return nil, e
		}
	}
	return m, nil
}
func meta(source string, n, limit int) model.Meta {
	total := n
	returned := n
	if returned > limit {
		returned = limit
	}
	return model.Meta{Source: source, RetrievedAt: time.Now(), ReturnedCount: returned, TotalCount: &total, Truncated: n > limit}
}
func (m *Mock) GetSystemInfo(ctx context.Context, id string) (model.SystemInfo, error) {
	if e := ctx.Err(); e != nil {
		return model.SystemInfo{}, e
	}
	r := model.SystemInfo{SystemID: id, Components: []model.Component{}, Dependencies: []model.Dependency{}}
	for _, s := range m.Systems {
		if s.SystemID == id {
			r = s
			r.Components = append([]model.Component{}, s.Components...)
			r.Dependencies = append([]model.Dependency{}, s.Dependencies...)
			break
		}
	}
	r.Meta = meta("mock_cmdb", len(r.Components), len(r.Components))
	return r, nil
}
func matches(ids []string, id string) bool {
	if len(ids) == 0 {
		return true
	}
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
func overlap(q model.Query, a, b time.Time) bool {
	s, _ := time.Parse(time.RFC3339, q.Start)
	e, _ := time.Parse(time.RFC3339, q.End)
	return a.Before(e) && !b.Before(s)
}
func (m *Mock) SearchChanges(ctx context.Context, q model.Query) (model.ChangeResult, error) {
	r := model.ChangeResult{SystemID: q.SystemID, Changes: []model.Change{}}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	for _, v := range m.Changes {
		ok := len(q.CIIDs) == 0
		for _, id := range v.CIIDs {
			ok = ok || matches(q.CIIDs, id)
		}
		if v.SystemID == q.SystemID && ok && v.TimeType == "actual" && overlap(q, v.Start, v.End) {
			r.Changes = append(r.Changes, v)
		}
	}
	sort.Slice(r.Changes, func(i, j int) bool { return r.Changes[i].Start.After(r.Changes[j].Start) })
	r.Meta = meta("mock_changes", len(r.Changes), q.Limit)
	if len(r.Changes) > q.Limit {
		r.Changes = r.Changes[:q.Limit]
	}
	return r, nil
}
func (m *Mock) GetAlerts(ctx context.Context, q model.Query) (model.AlertResult, error) {
	r := model.AlertResult{SystemID: q.SystemID, Alerts: []model.Alert{}}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	s, _ := time.Parse(time.RFC3339, q.Start)
	e, _ := time.Parse(time.RFC3339, q.End)
	for _, v := range m.Alerts {
		if v.SystemID == q.SystemID && matches(q.CIIDs, v.CIID) && !v.First.Before(s) && v.First.Before(e) {
			r.Alerts = append(r.Alerts, v)
		}
	}
	sort.Slice(r.Alerts, func(i, j int) bool { return r.Alerts[i].First.After(r.Alerts[j].First) })
	r.Meta = meta("mock_alerts", len(r.Alerts), q.Limit)
	if len(r.Alerts) > q.Limit {
		r.Alerts = r.Alerts[:q.Limit]
	}
	return r, nil
}
