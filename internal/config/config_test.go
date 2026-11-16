package config

import "testing"

func TestFailClosed(t *testing.T) {
	t.Setenv("MCP_LISTEN_ADDR", "0.0.0.0")
	t.Setenv("MCP_AUTH_TOKEN", "")
	if _, e := Load(); e == nil {
		t.Fatal("unauthenticated external binding")
	}
	t.Setenv("MCP_LISTEN_ADDR", "127.0.0.1")
	t.Setenv("DATA_MODE", "http")
	t.Setenv("OPS_API_BASE_URL", "")
	if _, e := Load(); e == nil {
		t.Fatal("missing upstream configuration")
	}
}
