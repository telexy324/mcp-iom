package provider

import (
	"bank-ops-mcp/internal/model"
	"fmt"
)

// No upstream contract has been approved. Never interpret unknown data as empty success.
func adaptSystem([]byte) (model.SystemInfo, error) {
	return model.SystemInfo{}, fmt.Errorf("upstream_adapter_not_configured")
}
func adaptChanges([]byte) (model.ChangeResult, error) {
	return model.ChangeResult{}, fmt.Errorf("upstream_adapter_not_configured")
}
func adaptAlerts([]byte) (model.AlertResult, error) {
	return model.AlertResult{}, fmt.Errorf("upstream_adapter_not_configured")
}
