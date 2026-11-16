package server

import (
	"bank-ops-mcp/internal/config"
	"bank-ops-mcp/internal/model"
	"bank-ops-mcp/internal/provider"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func validateID(c config.Config, id string) error {
	if !identifier.MatchString(id) {
		return fmt.Errorf("invalid_argument: system_id must match [A-Za-z0-9_-]{1,64}")
	}
	if len(c.Allowed) > 0 {
		for _, s := range c.Allowed {
			if s == id {
				return nil
			}
		}
		return fmt.Errorf("permission_denied: system outside allowed scope")
	}
	return nil
}
func validateQuery(c config.Config, q *model.Query) error {
	if e := validateID(c, q.SystemID); e != nil {
		return e
	}
	if len(q.CIIDs) > 100 {
		return fmt.Errorf("invalid_argument: too many ci_ids")
	}
	for _, id := range q.CIIDs {
		if !identifier.MatchString(id) {
			return fmt.Errorf("invalid_argument: invalid ci_id")
		}
	}
	a, e := time.Parse(time.RFC3339, q.Start)
	b, f := time.Parse(time.RFC3339, q.End)
	if e != nil || f != nil || !a.Before(b) || b.Sub(a) > time.Duration(c.Days)*24*time.Hour {
		return fmt.Errorf("invalid_argument: require RFC3339 time range within %d days", c.Days)
	}
	if q.Limit == 0 {
		q.Limit = 50
		if q.Limit > c.MaxResults {
			q.Limit = c.MaxResults
		}
	}
	if q.Limit < 1 || q.Limit > c.MaxResults {
		return fmt.Errorf("invalid_argument: limit out of range")
	}
	return nil
}

var sensitive = regexp.MustCompile(`(?i)(password|secret|token|api[_-]?key)\s*[:=]\s*[^\s,;]+|[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}|\b1[3-9][0-9]{9}\b`)

func clean(c config.Config, s string) string {
	for _, v := range []string{c.Token, c.HeaderValue} {
		if v != "" {
			s = strings.ReplaceAll(s, v, "[REDACTED]")
		}
	}
	s = sensitive.ReplaceAllString(s, "[REDACTED]")
	r := []rune(s)
	if len(r) > c.TextLimit {
		s = string(r[:c.TextLimit]) + "…"
	}
	return s
}
func audit(log *slog.Logger, name string, q model.Query, start time.Time, n int, err error) {
	if !identifier.MatchString(q.SystemID) {
		q.SystemID = "[INVALID]"
	}
	if len(q.CIIDs) > 100 {
		q.CIIDs = nil
	} else {
		ids := make([]string, len(q.CIIDs))
		for i, id := range q.CIIDs {
			if identifier.MatchString(id) {
				ids[i] = id
			} else {
				ids[i] = "[INVALID]"
			}
		}
		q.CIIDs = ids
	}
	if _, e := time.Parse(time.RFC3339, q.Start); e != nil {
		q.Start = ""
	}
	if _, e := time.Parse(time.RFC3339, q.End); e != nil {
		q.End = ""
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	log.Info("tool_call", "request_id", hex.EncodeToString(b), "tool", name, "system_id", q.SystemID, "ci_ids", q.CIIDs, "start_time", q.Start, "end_time", q.End, "limit", q.Limit, "duration_ms", time.Since(start).Milliseconds(), "returned_count", n, "success", err == nil)
}
func New(c config.Config, p provider.OpsProvider, log *slog.Logger) http.Handler {
	s := mcp.NewServer(&mcp.Implementation{Name: "bank-ops-mcp", Version: "0.1.0"}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "get_system_info", Description: "Read-only CMDB and registered dependencies from configured mock/platform provider; never infer dependencies.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(ctx context.Context, r *mcp.CallToolRequest, in model.SystemInput) (result *mcp.CallToolResult, out model.SystemInfo, err error) {
		start := time.Now()
		defer func() {
			audit(log, "get_system_info", model.Query{SystemID: in.SystemID}, start, len(out.Components), err)
		}()
		if err = validateID(c, in.SystemID); err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(ctx, time.Duration(c.Timeout)*time.Second)
		defer cancel()
		out, err = p.GetSystemInfo(ctx, in.SystemID)
		out.SystemName = clean(c, out.SystemName)
		for i := range out.Components {
			out.Components[i].Name = clean(c, out.Components[i].Name)
			out.Components[i].Host = clean(c, out.Components[i].Host)
		}
		return
	})
	mcp.AddTool(s, &mcp.Tool{Name: "search_changes", Description: "Read-only actual change/start-stop records from configured mock/platform provider; planned-only records excluded. Window overlaps [start_time,end_time).", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(ctx context.Context, r *mcp.CallToolRequest, q model.Query) (result *mcp.CallToolResult, out model.ChangeResult, err error) {
		start := time.Now()
		defer func() { audit(log, "search_changes", q, start, len(out.Changes), err) }()
		if err = validateQuery(c, &q); err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(ctx, time.Duration(c.Timeout)*time.Second)
		defer cancel()
		out, err = p.SearchChanges(ctx, q)
		for i := range out.Changes {
			out.Changes[i].Title = clean(c, out.Changes[i].Title)
			out.Changes[i].Description = clean(c, out.Changes[i].Description)
		}
		return
	})
	mcp.AddTool(s, &mcp.Tool{Name: "get_alerts", Description: "Read-only monitoring events from configured mock/platform provider; first_seen in [start_time,end_time), newest first. Counts are recorded event counts.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(ctx context.Context, r *mcp.CallToolRequest, q model.Query) (result *mcp.CallToolResult, out model.AlertResult, err error) {
		start := time.Now()
		defer func() { audit(log, "get_alerts", q, start, len(out.Alerts), err) }()
		if err = validateQuery(c, &q); err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(ctx, time.Duration(c.Timeout)*time.Second)
		defer cancel()
		out, err = p.GetAlerts(ctx, q)
		for i := range out.Alerts {
			out.Alerts[i].Name = clean(c, out.Alerts[i].Name)
			out.Alerts[i].Summary = clean(c, out.Alerts[i].Summary)
		}
		return
	})
	gin.SetMode(c.GinMode)
	g := gin.New()
	g.Use(gin.Recovery())
	g.GET("/healthz", func(ctx *gin.Context) { ctx.JSON(200, gin.H{"status": "ok"}) })
	g.Any(c.Path, func(ctx *gin.Context) {
		if c.Token != "" && subtle.ConstantTimeCompare([]byte(ctx.GetHeader("Authorization")), []byte("Bearer "+c.Token)) != 1 {
			ctx.AbortWithStatus(401)
			return
		}
		if ctx.GetHeader("Origin") != "" {
			ctx.AbortWithStatus(403)
			return
		}
		ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 65536)
		ctx.Next()
	}, gin.WrapH(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, &mcp.StreamableHTTPOptions{JSONResponse: true})))
	return g
}
