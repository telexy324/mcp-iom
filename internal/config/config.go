package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Addr, Port, Path, Token, Mode, MockDir, BaseURL, HeaderName, HeaderValue, CMDBPath, ChangesPath, AlertsPath, GinMode, LogLevel string
	Days, MaxResults, Timeout, MaxBody, TextLimit                                                                                  int
	Allowed                                                                                                                        []string
}

func env(k, d string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return d
}
func Load() (c Config, err error) {
	c = Config{Addr: env("MCP_LISTEN_ADDR", "127.0.0.1"), Port: env("PORT", "8080"), Path: env("MCP_PATH", "/mcp"), Token: env("MCP_AUTH_TOKEN", ""), Mode: env("DATA_MODE", "mock"), MockDir: env("MOCK_DATA_DIR", "testdata/mock"), BaseURL: env("OPS_API_BASE_URL", ""), HeaderName: env("OPS_API_HEADER_NAME", "api_key"), HeaderValue: env("OPS_API_HEADER_VALUE", ""), CMDBPath: env("OPS_CMDB_PATH", "/api/cmdb/systems/{system_id}"), ChangesPath: env("OPS_CHANGES_PATH", "/api/changes"), AlertsPath: env("OPS_ALERTS_PATH", "/api/alerts"), GinMode: env("GIN_MODE", "debug"), LogLevel: env("LOG_LEVEL", "info")}
	for _, p := range []struct {
		k string
		d int
		v *int
	}{{"MAX_QUERY_DAYS", 7, &c.Days}, {"MAX_RESULTS", 100, &c.MaxResults}, {"HTTP_TIMEOUT_SECONDS", 10, &c.Timeout}, {"MAX_RESPONSE_BYTES", 1048576, &c.MaxBody}, {"MAX_TEXT_LENGTH", 1024, &c.TextLimit}} {
		*p.v, err = strconv.Atoi(env(p.k, strconv.Itoa(p.d)))
		if err != nil || *p.v < 1 || *p.v > 10000000 {
			return c, fmt.Errorf("invalid %s", p.k)
		}
	}
	if c.MaxResults > 100 || c.Days > 365 || c.Timeout > 300 {
		return c, fmt.Errorf("configured query limits exceed supported bounds")
	}
	if c.GinMode != "debug" && c.GinMode != "release" && c.GinMode != "test" {
		return c, fmt.Errorf("invalid GIN_MODE")
	}
	port, e := strconv.Atoi(c.Port)
	if e != nil || port < 1 || port > 65535 {
		return c, fmt.Errorf("invalid PORT")
	}
	if !strings.HasPrefix(c.Path, "/") || c.Path == "/healthz" || strings.ContainsAny(c.Path, "*:? #") {
		return c, fmt.Errorf("invalid MCP_PATH")
	}
	ip := net.ParseIP(c.Addr)
	if ip == nil {
		return c, fmt.Errorf("MCP_LISTEN_ADDR must be an IP address")
	}
	if !ip.IsLoopback() && c.Token == "" {
		return c, fmt.Errorf("MCP_AUTH_TOKEN required for non-loopback listening")
	}
	if s := env("ALLOWED_SYSTEM_IDS", ""); s != "" {
		for _, id := range strings.Split(s, ",") {
			c.Allowed = append(c.Allowed, strings.TrimSpace(id))
		}
	}
	if c.Mode != "mock" && c.Mode != "http" {
		return c, fmt.Errorf("invalid DATA_MODE")
	}
	if c.Mode == "http" {
		u, e := url.Parse(c.BaseURL)
		if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return c, fmt.Errorf("valid OPS_API_BASE_URL required")
		}
		if len(c.Allowed) == 0 {
			return c, fmt.Errorf("ALLOWED_SYSTEM_IDS required in http mode")
		}
		if (c.HeaderName == "") != (c.HeaderValue == "") {
			return c, fmt.Errorf("configure upstream header name and value together")
		}
	}
	return c, nil
}
