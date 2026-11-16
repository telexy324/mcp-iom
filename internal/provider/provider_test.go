package provider

import (
	"bank-ops-mcp/internal/config"
	"bank-ops-mcp/internal/model"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMock(t *testing.T) {
	m, e := NewMock("../../testdata/mock")
	if e != nil {
		t.Fatal(e)
	}
	q := model.Query{SystemID: "SYS001", Start: "2026-11-13T13:00:00+08:00", End: "2026-11-13T15:00:00+08:00", Limit: 1}
	r, e := m.SearchChanges(context.Background(), q)
	if e != nil || len(r.Changes) != 1 || r.Changes[0].ID != "CHG001" {
		t.Fatalf("%+v %v", r, e)
	}
	m.Changes = append(m.Changes, m.Changes[0])
	r, _ = m.SearchChanges(context.Background(), q)
	if !r.Meta.Truncated || *r.Meta.TotalCount != 2 {
		t.Fatal("truncation")
	}
	q.CIIDs = []string{"CI002"}
	a, _ := m.GetAlerts(context.Background(), q)
	if len(a.Alerts) != 0 {
		t.Fatal("CI filtering")
	}
	q.CIIDs = nil
	q.SystemID = "MISSING"
	r, _ = m.SearchChanges(context.Background(), q)
	if len(r.Changes) != 0 {
		t.Fatal("system filtering")
	}
	q.SystemID = "SYS001"
	q.Start = "2026-11-14T00:00:00Z"
	q.End = "2026-11-15T00:00:00Z"
	r, _ = m.SearchChanges(context.Background(), q)
	if len(r.Changes) != 0 {
		t.Fatal("time filtering")
	}
}
func TestHTTPFailures(t *testing.T) {
	for _, tc := range []struct {
		status     int
		body, want string
	}{{401, "", "permission_denied"}, {500, "", "status_500"}, {200, "unknown", "adapter_not_configured"}, {200, strings.Repeat("x", 200), "too_large"}} {
		t.Run(tc.want, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
			defer s.Close()
			c, _ := config.Load()
			c.BaseURL = s.URL
			c.MaxBody = 100
			_, e := NewHTTP(c).GetSystemInfo(context.Background(), "SYS001")
			if e == nil || !strings.Contains(e.Error(), tc.want) {
				t.Fatalf("%v", e)
			}
		})
	}
}
