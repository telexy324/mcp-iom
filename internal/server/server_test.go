package server

import (
	"bank-ops-mcp/internal/config"
	"bank-ops-mcp/internal/model"
	"bank-ops-mcp/internal/provider"
	"context"
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type authTransport struct{}

func (authTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer example-test-token")
	return http.DefaultTransport.RoundTrip(r)
}
func TestMCP(t *testing.T) {
	c, e := config.Load()
	if e != nil {
		t.Fatal(e)
	}
	c.Token = "example-test-token"
	c.GinMode = "test"
	p, e := provider.NewMock("../../testdata/mock")
	if e != nil {
		t.Fatal(e)
	}
	srv := httptest.NewServer(New(c, p, slog.New(slog.NewTextHandler(io.Discard, nil))))
	defer srv.Close()
	for _, method := range []string{"GET", "POST", "DELETE"} {
		r, _ := http.NewRequest(method, srv.URL+c.Path, nil)
		res, e := http.DefaultClient.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatalf("%s: %d", method, res.StatusCode)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, e := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL + c.Path, HTTPClient: &http.Client{Transport: authTransport{}}}, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer session.Close()
	list, e := session.ListTools(ctx, nil)
	if e != nil || len(list.Tools) != 3 {
		t.Fatalf("list: %v %v", list, e)
	}
	for _, name := range []string{"get_system_info", "search_changes", "get_alerts"} {
		args := map[string]any{"system_id": "SYS001"}
		if name != "get_system_info" {
			args["start_time"] = "2026-11-13T13:00:00+08:00"
			args["end_time"] = "2026-11-13T15:00:00+08:00"
			args["ci_ids"] = []string{"CI001"}
		}
		r, e := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if e != nil || r.IsError {
			t.Fatalf("%s: %v %v", name, r, e)
		}
		b, _ := json.Marshal(r.StructuredContent)
		var payload map[string]any
		if e = json.Unmarshal(b, &payload); e != nil || payload["system_id"] != "SYS001" {
			t.Fatalf("structured output: %s", b)
		}
	}
	r, e := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_alerts", Arguments: map[string]any{"system_id": "SYS001", "start_time": "bad", "end_time": "bad"}})
	if e == nil && !r.IsError {
		t.Fatal("invalid times succeeded")
	}
}
func TestValidation(t *testing.T) {
	c, _ := config.Load()
	for _, q := range []model.Query{{SystemID: "../x"}, {SystemID: "SYS001", Start: "2026-11-01T00:00:00Z", End: "2026-11-15T00:00:00Z"}, {SystemID: "SYS001", Start: "2026-11-01T00:00:00Z", End: "2026-11-02T00:00:00Z", Limit: 101}} {
		if validateQuery(c, &q) == nil {
			t.Fatal("invalid query accepted")
		}
	}
	c.Allowed = []string{"SYS002"}
	if validateID(c, "SYS001") == nil {
		t.Fatal("scope bypass")
	}
	if clean(c, "password=abcdef") != "[REDACTED]" {
		t.Fatal("redaction")
	}
}
