package provider

import (
	"bank-ops-mcp/internal/model"
	"context"
)

type OpsProvider interface {
	GetSystemInfo(context.Context, string) (model.SystemInfo, error)
	SearchChanges(context.Context, model.Query) (model.ChangeResult, error)
	GetAlerts(context.Context, model.Query) (model.AlertResult, error)
}
