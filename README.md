# 银行一体化运维 MCP Server

Go 1.23.3 + Gin v1.10.1 + 官方 MCP Go SDK v1.0.0，提供三个只读工具。当前支持完整 Mock 演示；真实平台 HTTP 传输层已实现，字段映射等待实际接口契约。

## 本地运行

在项目根目录执行：

```sh
cp .env.example .env
set -a
. ./.env
set +a
go run ./cmd/server
```

环境变量由 shell 加载，程序不会自行读取 `.env`。默认监听 `127.0.0.1:8080`。`GET /healthz` 返回 `{"status":"ok"}`，MCP 地址为 `http://127.0.0.1:8080/mcp`。这不是普通 REST 查询接口，请使用支持 Streamable HTTP 的 MCP Client。

```sh
go build ./...
go test ./...
```

协议集成测试使用官方 MCP Client，真实验证 initialize、tools/list、三个工具调用、结构化输出、参数错误及 GET/POST/DELETE 鉴权。

## 工具与查询规则

- `get_system_info`：输入 `system_id`，返回配置项与已登记依赖。未知系统返回空配置项和空依赖。
- `search_changes`：必填 `system_id`、`start_time`、`end_time`；可选 `ci_ids`、`limit`。返回实际执行时间与窗口重叠的变更/启停记录，按开始时间倒序；计划记录不纳入实际执行查询。
- `get_alerts`：参数同上，按首次发生时间落在 `[start_time,end_time)` 内过滤，按首次发生时间倒序。

标识符为 1～64 个 ASCII 字母、数字、下划线或短横线；CI 筛选最多 100 个。时间必须为带时区 RFC3339，开始早于结束，默认最大 7 天。limit 省略或为 0 时默认 50（受 MAX_RESULTS 限制）；负数或超上限返回错误。默认上限 100，配置不得大于 100。变更区间末端等于窗口开始时视为重叠，开始等于窗口结束时排除。

所有列表均返回来源、查询时间、实际返回条数、过滤后总条数及截断标记。告警 count 是模拟数据提供的事件次数，total_count 是告警记录数量。HTTP 后续若无法获知总量应使用 null。

示例调用参数：

```json
{"system_id":"SYS001","ci_ids":["CI001"],"start_time":"2026-11-13T13:00:00+08:00","end_time":"2026-11-13T15:00:00+08:00","limit":50}
```

演示中 CHG001 于 13:30～13:45 调整 CI001，ALT001 于 14:01 首次出现。SYS002 用于验证隔离。数据均为虚构；时间接近不能证明因果关系。

## 配置和访问范围

完整配置见 `.env.example`。跨主机监听必须设置 `MCP_AUTH_TOKEN`，客户端发送 `Authorization: Bearer <token>`。各 MCP HTTP 方法统一鉴权。拒绝携带 Origin 的浏览器请求；当前面向后端 MCP Client。请求体限制 64 KiB，上游响应默认限制 1 MiB，超时默认 10 秒，不自动重试或跟随重定向。

`ALLOWED_SYSTEM_IDS=SYS001,SYS002` 可限制工具查询范围；HTTP 模式必填。该范围需由部署方批准，不等同于逐用户授权。静态令牌无法识别个人调用者，审计不伪造个人身份。

`MAX_TEXT_LENGTH` 控制展示文本长度；描述中的常见凭据、邮箱、手机号和配置的令牌会被替换。真实接口接入时应根据平台字段继续完善脱敏；当前映射未开放。审计为 JSON 日志，记录工具调用 ID、查询范围、耗时、数量及成功状态，不记录完整结果或认证 Header。

## HTTP Provider 骨架

设置 DATA_MODE=http 时，必须有合法 OPS_API_BASE_URL、ALLOWED_SYSTEM_IDS，以及成对配置的认证 Header 名称和值；无认证的测试上游需将 Header 名称和值同时置空。缺配置启动失败，不回退 Mock。

`internal/provider/http.go` 负责受限 GET 请求、参数编码、超时、响应体大小和上游状态处理。三个路径可配置。`internal/provider/adapter.go` 目前明确返回 `upstream_adapter_not_configured`，即使 HTTP 200 也不会把未知格式输出为空集合。

获得真实 API 文档后，在 Adapter 实现字段字典、权限、分页、数据范围校验和统一模型映射，补充契约测试，再切换生产数据。工具名称及输入字段保持稳定。

## 百度千帆联调

1. 部署到千帆后端可达的批准网络，跨主机设置令牌，使用受控 HTTPS 网关并限制来源。
2. 千帆添加第三方 MCP 工具，选择 Streamable HTTP，填写 `https://<批准域名>/mcp`。
3. 配置 Authorization Header，初始化后应发现恰好三个工具。
4. 查询 SYS001 CMDB，再用上述时间窗口查变更和告警，结合人工上传的维护手册生成引用 CHG001/ALT001 的排查建议。
5. 验证错误 token、未知系统和非法时间窗口的行为。

网关应保留 MCP 会话及协议 Header，允许 GET/POST/DELETE，不缓存或缓冲 SSE。401 检查服务令牌；403 检查 Origin 或系统范围；超时检查网络和上游；adapter_not_configured 表示真实平台映射尚未完成。

阶段 A/B 已在本地实现，阶段 C 需在实际千帆环境验证。阶段 D 完成传输骨架，真实字段映射及平台验收待接口资料和测试环境就绪。
