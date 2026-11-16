package provider

import (
	"bank-ops-mcp/internal/config"
	"bank-ops-mcp/internal/model"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type HTTP struct {
	c      config.Config
	client *http.Client
}

func NewHTTP(c config.Config) *HTTP {
	return &HTTP{c, &http.Client{Timeout: time.Duration(c.Timeout) * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (h *HTTP) fetch(ctx context.Context, path string, q *model.Query) ([]byte, error) {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, fmt.Errorf("upstream_configuration_error")
	}
	u, e := url.Parse(strings.TrimRight(h.c.BaseURL, "/") + path)
	if e != nil {
		return nil, fmt.Errorf("upstream_configuration_error")
	}
	if q != nil {
		v := u.Query()
		v.Set("system_id", q.SystemID)
		v.Set("start_time", q.Start)
		v.Set("end_time", q.End)
		v.Set("limit", strconv.Itoa(q.Limit))
		for _, id := range q.CIIDs {
			v.Add("ci_ids", id)
		}
		u.RawQuery = v.Encode()
	}
	req, e := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if e != nil {
		return nil, fmt.Errorf("upstream_configuration_error")
	}
	if h.c.HeaderName != "" {
		req.Header.Set(h.c.HeaderName, h.c.HeaderValue)
	}
	res, e := h.client.Do(req)
	if e != nil {
		if errors.Is(e, context.DeadlineExceeded) {
			return nil, fmt.Errorf("upstream_timeout")
		}
		return nil, fmt.Errorf("upstream_connection_error")
	}
	defer res.Body.Close()
	if res.StatusCode == 401 || res.StatusCode == 403 {
		return nil, fmt.Errorf("upstream_permission_denied")
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("upstream_status_%d", res.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, int64(h.c.MaxBody)+1))
	if e != nil {
		return nil, fmt.Errorf("upstream_read_error")
	}
	if len(b) > h.c.MaxBody {
		return nil, fmt.Errorf("upstream_response_too_large")
	}
	return b, nil
}
func (h *HTTP) GetSystemInfo(ctx context.Context, id string) (model.SystemInfo, error) {
	b, e := h.fetch(ctx, strings.ReplaceAll(h.c.CMDBPath, "{system_id}", url.PathEscape(id)), nil)
	if e != nil {
		return model.SystemInfo{}, e
	}
	return adaptSystem(b)
}
func (h *HTTP) SearchChanges(ctx context.Context, q model.Query) (model.ChangeResult, error) {
	b, e := h.fetch(ctx, h.c.ChangesPath, &q)
	if e != nil {
		return model.ChangeResult{}, e
	}
	return adaptChanges(b)
}
func (h *HTTP) GetAlerts(ctx context.Context, q model.Query) (model.AlertResult, error) {
	b, e := h.fetch(ctx, h.c.AlertsPath, &q)
	if e != nil {
		return model.AlertResult{}, e
	}
	return adaptAlerts(b)
}
