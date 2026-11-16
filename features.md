# 银行一体化运维平台 MCP Server — 功能需求（features.md）

> 版本：v0.2 / MVP（Gin HTTP 框架）  
> 开发语言：Go  
> MCP SDK：`github.com/modelcontextprotocol/go-sdk/mcp`  
> 接入方：银行内部百度千帆智能体（支持 Streamable HTTP）  
> 目标：以最小开发量实现三个只读运维工具，先 Mock 联调，后续替换为一体化运维平台真实 HTTP API。

## 1. 项目目标与边界

开发一个独立部署的 Go MCP Server，向千帆智能体提供以下能力：

1. 查询业务系统的 CMDB 信息与已登记的依赖关系；
2. 查询指定系统或配置项在时间段内的变更工单及服务器启停记录；
3. 查询指定系统或配置项在时间段内的监控告警。

在千帆中可通过自然语言触发这些工具，例如：

> 请查询 SYS001 系统的资产信息，以及 2026-11-13 13:00 至 15:00 期间的变更和告警，并说明可能需要优先排查的变更。

**职责边界：** MCP Server 只负责数据访问、校验、标准化、限制与审计；千帆智能体负责工具编排、结合上传的启停维护手册进行解释和生成辅助分析建议。MCP Server **不**直接调用 LLM，不实现复杂的多 Agent 流程，不负责自动认定故障根因。

**MVP 不做：** 独立 Web 前端、知识库管理、数据库、消息队列、自动执行重启/变更/回滚、任意 SQL 或 Shell 执行、动态工具创建、复杂权限管理平台。

## 2. 技术选型

- Go：采用项目开发环境中稳定且受官方 MCP Go SDK 支持的版本；在 `go.mod` 中固定实际使用版本。
- MCP：优先采用官方 Go SDK，并固定经联调验证的依赖版本。
- 协议：**Streamable HTTP**，`POST/GET /mcp` 等行为由 SDK 处理，不手写 MCP JSON-RPC 协议。
- **HTTP 框架：Gin**（`github.com/gin-gonic/gin`），负责路由、中间件、健康检查；MCP 协议仍由官方 SDK 处理。
- **MCP 与 Gin 集成：** 采用 `gin.WrapH(mcp.NewStreamableHTTPHandler(...))` 挂载到 `/mcp`，保留 MCP SDK 的完整 Streamable HTTP 处理能力；不要自己解析 JSON-RPC。
- 不额外引入 Hertz；首版只选 Gin，避免双框架增加复杂度。
- 配置：环境变量，提供 `.env.example` 说明；标准库不自动读取 `.env`，可由启动脚本加载环境变量。
- 日志：`log/slog`，以 JSON 输出。
- 数据模式：`mock` / `http`，通过配置切换，业务层不感知区别。
- 测试：Go `testing` + `httptest`；优先编写真实 MCP Client 的协议集成测试。

参考：
- [Gin](https://github.com/gin-gonic/gin)
- [官方 MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [SDK Quick Start](https://github.com/modelcontextprotocol/go-sdk/blob/main/docs/quick_start.md)
- [Streamable HTTP 使用说明](https://github.com/modelcontextprotocol/go-sdk/blob/main/docs/protocol.md)

### 2.1 Gin 集成约定（必须遵守）

Gin 只是 HTTP 服务容器，**MCP SDK 仍是协议实现者**。优先通过 `gin.WrapH` 适配 SDK 的 `http.Handler`，不要自己实现 MCP 方法路由或协议解析。

下面是**集成方式示意**（具体依赖版本以实际构建通过为准）：

```go
s := mcp.NewServer(&mcp.Implementation{Name: "bank-ops-mcp", Version: "0.1.0"}, nil)
// ... 使用 mcp.AddTool(s, ...) 注册三个工具 ...
mcpHandler := mcp.NewStreamableHTTPHandler(
    func(r *http.Request) *mcp.Server { return s },
    &mcp.StreamableHTTPOptions{JSONResponse: true},
)
router := gin.New()
router.Use(gin.Recovery())
router.GET("/healthz", func(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"status": "ok"})
})
// MCP 入口独立分组：对所有 MCP HTTP 方法统一鉴权
mcpGroup := router.Group("/mcp", MCPAuthMiddleware(/* config */))
mcpGroup.Any("", gin.WrapH(mcpHandler))
// Gin 的 Any 支持该版本列出的 HTTP 方法；需以实际 MCP Client
// 联调确认 initialize、tools/list、tools/call 和会话所需方法正常。
```

如使用 `router.Any("/mcp", ...)` 也可以，但必须确认 Gin 路由匹配、认证和 SDK 所需的 GET、POST、DELETE 等方法均正确生效。生产部署需确认反向代理不会缓存/缓冲 MCP 流式响应或错误处理 SSE/会话 Header。不要在 SDK Handler 外再包一层将响应整体缓存的 Gin 中间件。

在本地 PoC 中默认监听 `127.0.0.1`，跨主机网络连接时必须启用服务认证及受控网络访问；开放监听地址本身不等于安全授权。

## 3. 整体结构

```text
百度千帆智能体（MCP Client）
       |
       | Streamable HTTP + 配置的认证 Header
       v
Gin HTTP Server: /mcp (MCP SDK Handler)
       |
       +-- Tool: get_system_info
       +-- Tool: search_changes
       +-- Tool: get_alerts
       |
       v
输入校验 -> 授权检查 -> 查询限制 -> Provider/Adapter -> 标准化响应
                                      |
                            +---------+---------+
                            |                   |
                      Mock Provider         HTTP Provider
                            |                   |
                       JSON 测试数据       一体化运维平台 API
```

原则：**MCP 工具结构保持稳定，未来只替换 Adapter 和映射配置，不改变千帆侧工具名称及输入字段。**

## 4. MCP 工具定义

三个工具均为**只读工具**。所有工具均应在描述中标明数据来源和适用范围。无数据应返回空集合及查询元数据，不得编造结果。

### 4.1 `get_system_info`：CMDB 查询

**用途：** 根据 `system_id` 获取业务系统、配置项以及已登记的依赖关系。

输入（JSON）：

```json
{"system_id":"SYS001"}
```

输入约束：`system_id` 必填，长度 1～64，只接受安全的标识符字符，不允许自由拼接 SQL。

输出示例（全部为模拟数据）：

```json
{
  "system_id": "SYS001",
  "system_name": "示例支付系统",
  "components": [
    {"ci_id":"CI001", "type":"application_server", "name":"app01", "host":"app01"},
    {"ci_id":"CI002", "type":"database", "name":"db01", "host":"db01"}
  ],
  "dependencies": [
    {"source_ci_id":"CI001", "target_ci_id":"CI002", "relation":"depends_on"}
  ],
  "meta": {"source":"mock_cmdb", "retrieved_at":"2026-11-16T09:00:00+08:00", "truncated":false}
}
```

若源系统不提供依赖关系，则返回 `dependencies: []`，不可由 MCP Server 猜测依赖。

### 4.2 `search_changes`：变更/启停记录查询

**用途：** 查找故障发生前后的相关工单与启停操作，支持与 CMDB 的 `ci_id` 关联。

输入示例：

```json
{
  "system_id":"SYS001",
  "ci_ids":["CI001"],
  "start_time":"2026-11-13T13:00:00+08:00",
  "end_time":"2026-11-13T15:00:00+08:00",
  "limit":50
}
```

约束：
- `system_id` 必填；`ci_ids` 可选，为过滤条件；`start_time`、`end_time` 必填且为 RFC3339 带时区时间；开始时间必须早于结束时间。
- 单次最大时间跨度默认 7 天（可配置，不允许无限范围）。
- `limit` 默认 50，上限 100；超出上限拒绝或按明确规则限制，并在文档说明。
- 按实际执行时间过滤；只有计划时间的记录要明确标注，不可当成实际执行时间。

输出示例（模拟）：

```json
{
  "system_id":"SYS001",
  "changes":[
    {
      "change_id":"CHG001",
      "record_type":"change",
      "ci_ids":["CI001"],
      "title":"调整应用连接池参数",
      "description":"调整最大连接数",
      "start_time":"2026-11-13T13:30:00+08:00",
      "end_time":"2026-11-13T13:45:00+08:00",
      "time_type":"actual",
      "status":"completed"
    }
  ],
  "meta":{"source":"mock_changes", "retrieved_at":"2026-11-16T09:00:00+08:00", "returned_count":1, "total_count":1, "truncated":false}
}
```

`record_type` 可为 `change`、`start_stop`；不允许混淆审批状态、执行状态和操作实际结果。

### 4.3 `get_alerts`：监控告警查询

**用途：** 按系统、配置项和时间窗口获取告警。返回有界结果，避免将大量原始日志直接送入 LLM。

输入示例：

```json
{
  "system_id":"SYS001",
  "ci_ids":["CI001"],
  "start_time":"2026-11-13T13:00:00+08:00",
  "end_time":"2026-11-13T15:00:00+08:00",
  "limit":50
}
```

约束与变更查询相同。默认按 `first_seen` 从新到旧排序；若返回聚合计数，应由程序真实统计，不能假设源 API 的行数就是事件总数。

输出示例（模拟）：

```json
{
  "system_id":"SYS001",
  "alerts":[
    {
      "alert_id":"ALT001",
      "ci_id":"CI001",
      "name":"数据库连接超时",
      "severity":"critical",
      "first_seen":"2026-11-13T14:01:00+08:00",
      "last_seen":"2026-11-13T14:05:00+08:00",
      "count":12,
      "summary":"示例应用出现数据库连接超时"
    }
  ],
  "meta":{"source":"mock_alerts", "retrieved_at":"2026-11-16T09:00:00+08:00", "returned_count":1, "total_count":1, "truncated":false}
}
```

### 4.4 统一响应要求

- 使用 Go 结构体输出 JSON，优先返回 MCP SDK 支持的结构化结果；必要时同时提供简洁文本。
- 字段使用明确的业务命名，统一 `system_id`、`ci_id`、`change_id`、`alert_id`。
- 时间统一 RFC3339，保留时区信息；绝不混用计划时间、实际时间和查询时间。
- 返回 `meta.source`、`meta.retrieved_at`、`meta.truncated`；列表另有 `returned_count` 和 `total_count`（源平台未知总量时允许 `total_count` 为空，并明确说明）。
- 结果不得包含接口密钥、登录凭据、完整个人敏感信息；日志、文本字段按配置进行脱敏和长度限制。
- 区分“无结果”“参数不合法”“无权限”“源平台错误”“超时”，不可把接口调用失败伪装成空结果。
- 不要求 LLM 直接理解源平台内部代码值，如 `CI_TYP=02`；在 Adapter 中通过受控映射转换，未知代码保留原值并标识未识别。

## 5. 数据接口适配：先 Mock，后 HTTP

采用 Provider 接口隔离数据来源。以下是**开发约定**，不是已确认的一体化运维平台真实 API。

```go
type OpsProvider interface {
    GetSystemInfo(ctx context.Context, systemID string) (SystemInfo, error)
    SearchChanges(ctx context.Context, query QueryFilter) (ChangeSearchResult, error)
    GetAlerts(ctx context.Context, query QueryFilter) (AlertSearchResult, error)
}
```

其中 `QueryFilter` 包含 `SystemID`、`CIIDs`、`StartTime`、`EndTime`、`Limit`。

### 5.1 MockProvider（必须实现）

- 使用 `testdata/mock/cmdb.json`、`changes.json`、`alerts.json`，数据全部为虚构示例。
- 按 `system_id`、`ci_ids`、时间窗口真实过滤；按 `limit` 截取并正确设置 `truncated`。
- 提供 `SYS001` 的可关联案例：CI001 的变更在 13:30～13:45，CI001 的告警出现在 14:01；另提供无关系统数据以验证过滤。
- 保证演示数据不包含银行真实 IP、人员、工单及生产名称。

### 5.2 HTTPProvider（预留并实现基本骨架）

在平台 API 尚未确定时，只需要实现**可配置 HTTP Client + Adapter 结构**。在没有真实契约时，不要求完成具体字段映射或真实接口联调。

建议的**占位 API 路径**（未来可通过配置调整）：

| 功能 | 假设请求方式及路径 | 备注 |
|---|---|---|
| CMDB | `GET /api/cmdb/systems/{system_id}` | 占位示例 |
| 变更 | `GET /api/changes?system_id=...&start_time=...&end_time=...` | 占位示例 |
| 告警 | `GET /api/alerts?system_id=...&start_time=...&end_time=...` | 占位示例 |

规范要求：
- `OPS_API_BASE_URL` 控制服务地址，三个路径可分别配置；不得把接口域名硬编码。
- 自定义上游认证由环境变量配置，至少支持静态 Header Key/Value；支持后续扩展为 token 获取机制。
- API Key/Secret 仅在服务端保存，不写入 MCP 返回值和日志。
- HTTP 请求必须带 `context`，设置合理超时；限制重试，不对非幂等请求自动重试（本期只读）。
- API 返回解析放在独立 Adapter 文件；真实平台字段确定后再填充映射逻辑。未知响应格式须返回明确错误，不能默默输出空数组。
- `DATA_MODE=http` 但缺少必要配置时，应启动失败并提示配置缺失，而不是回退成 Mock 后让用户误以为是真实数据。

## 6. 配置规范

提供 `.env.example`（只含示例，不包含真实凭据）：

```dotenv
PORT=8080
GIN_MODE=debug
DATA_MODE=mock
MCP_PATH=/mcp

# MCP 入口鉴权：本地 PoC 默认仅监听 127.0.0.1；跨主机测试需启用鉴权
MCP_AUTH_TOKEN=
MCP_LISTEN_ADDR=127.0.0.1

# 后续真实平台接口
OPS_API_BASE_URL=
OPS_CMDB_PATH=/api/cmdb/systems/{system_id}
OPS_CHANGES_PATH=/api/changes
OPS_ALERTS_PATH=/api/alerts
OPS_API_HEADER_NAME=api_key
OPS_API_HEADER_VALUE=

HTTP_TIMEOUT_SECONDS=10
MAX_QUERY_DAYS=7
MAX_RESULTS=100
LOG_LEVEL=info
```

备注：若运维平台同时需要 `api_key` 和 `api_secret`，可将认证配置扩展为多个 Header（推荐使用独立环境变量或受控的映射配置），不要将实际密钥写进源码。

## 7. 安全与运维要求（MVP 必须）

1. **只读：** 仅注册 3 个查询工具，不提供任何重启、启停、变更执行、SQL/Shell 执行工具。
2. **入口访问控制：** 跨主机部署时必须校验 MCP 请求 Header 中的服务令牌，并限制网络访问来源；不能仅依赖千帆端提示词。PoC 可使用静态服务令牌，正式上线前明确用户身份传递与授权模型。
3. **权限边界：** 不允许借助 MCP 共用账号绕过原平台的业务系统数据权限。无法取得调用者身份及资源授权时，只允许使用批准的低敏测试数据或受控查询范围，不得直接开放全量生产查询。
4. **查询限制：** 必须限制时间跨度、单次结果条数、输入长度、上游响应体大小和请求超时时间；阻止无限制检索。
5. **审计：** 记录请求 ID、工具名、调用方标识（能获取时）、查询范围、耗时、结果数量、成功/失败；不记录密钥或完整敏感结果。
6. **网络：** 本地模式仅监听 `127.0.0.1`。内网部署应优先在受控 HTTPS 网关后提供 `/mcp`，并确认千帆后端到服务地址的网络连通性。
7. **错误处理：** 上游连接错误、401/403、非预期 JSON、超时均须可定位；对智能体只返回必要信息，不泄漏内部 Header、凭据、完整异常堆栈。
8. **不可信文本：** 变更描述、告警说明等均为外部数据；在千帆工作流中按证据处理，不能当作更高优先级的系统指令。

> 注意：静态服务令牌只解决初步的服务接入认证，**不能自动实现逐用户、逐业务系统的授权**。生产使用必须由银行内部安全/运维规范确定进一步措施。

## 8. 推荐目录结构

```text
bank-ops-mcp/
├── cmd/server/main.go                # 启动 Gin + MCP SDK Handler
├── internal/config/config.go         # 环境变量配置
├── internal/model/types.go           # 输入/输出结构体
├── internal/tools/register.go        # 三个 MCP 工具注册
├── internal/provider/provider.go     # OpsProvider 接口
├── internal/provider/mock.go         # Mock 数据查询
├── internal/provider/http.go         # HTTP Client 骨架
├── internal/auth/middleware.go       # Gin 中间件：MCP 入口鉴权
├── internal/audit/log.go             # 审计日志
├── testdata/mock/cmdb.json
├── testdata/mock/changes.json
├── testdata/mock/alerts.json
├── .env.example
├── go.mod
├── README.md
└── features.md
```

**保持轻量：** 不强制引入 DI 框架、ORM、Redis、PostgreSQL、Docker Compose 或复杂的分层目录；在代码保持清晰前提下可合并少量文件。

## 9. 开发实施顺序

### 阶段 A：MCP 协议打通

- 初始化 Go 项目，引入 Gin 和官方 MCP Go SDK，注册 `get_system_info`。
- 创建 `gin.Engine`，通过 `gin.WrapH` 挂载 SDK 的 Streamable HTTP Handler 至 `/mcp`；MCP 初始化、工具发现、工具调用由 SDK 处理。
- 使用 Gin 注册 `GET /healthz`；不要将健康检查或普通 REST API 与 MCP 请求逻辑混在一起。
- 用 Gin Middleware 对 `/mcp` 的 GET/POST/DELETE 请求统一做服务令牌校验；勿让 `/mcp` 某个方法绕过认证。加入 `MCP_AUTH_TOKEN` 可选配置（仅限本地测试为空），跨主机配置必须启用。
- 用独立 MCP Client 或 Inspector 进行本地验证。

### 阶段 B：三个 Mock 工具

- 定义 `OpsProvider` 接口与统一模型。
- 实现 `MockProvider` 和三个 Tool。
- 增加字段校验、按系统与时间过滤、分页/数量限制、异常响应、审计日志。
- 使用模拟故障完成三个工具的联动演示。

### 阶段 C：百度千帆联调

- 千帆中创建第三方 MCP 工具，传输方式选择 **Streamable HTTP**。
- URL 填写 `https://<经批准的内网域名>/mcp`（测试环境可使用可达的内网地址）。
- 在 Headers 中填写约定的服务端认证 Header（如 `Authorization: Bearer <token>`，与代码保持一致）。
- 验证 `initialize`、`tools/list`、`tools/call`，确认三个工具均可发现、可按参数调用。
- 千帆工作流示例：获取 CMDB -> 查询变更与告警 -> 结合人工上传的维护手册 -> 形成有来源的排查建议。

### 阶段 D：预留真实平台接口

- 根据一体化运维平台实际文档确认 API 地址、认证方式、字段字典、分页、数据权限、时间字段与关联标识。
- 补齐 `HTTPProvider` 对应 Adapter，使用测试环境验证。
- 在不更改 MCP Tool Schema 的前提下，切换 `DATA_MODE=http`。
- 若实际平台接口差异较大，应调整 Adapter 而不是让千帆理解平台原始数据。

## 10. 验收标准

### 技术验收

- [ ] `go build ./...` 通过，`go test ./...` 通过。
- [ ] `DATA_MODE=mock` 可以独立启动；Gin 的 `GET /healthz` 返回健康状态（不暴露内部环境信息）。
- [ ] `/mcp` 通过 Gin 挂载的 SDK Handler 处理；支持 SDK 所需的 HTTP 方法、状态码、响应头和流式行为，不被自定义中间件破坏。
- [ ] 千帆或标准 MCP Client 能连接 `/mcp`，完成初始化及工具发现。
- [ ] `tools/list` 能发现恰好三个业务工具：`get_system_info`、`search_changes`、`get_alerts`。
- [ ] 三个工具能按入参查询 Mock 数据，输出符合本文约定的结构化 JSON。
- [ ] 无效时间、缺少系统 ID、超出限制的查询会收到明确错误。
- [ ] 无数据与上游失败被正确区分。
- [ ] 入口认证生效；无效 token 无法调用工具；日志中没有密钥。
- [ ] `DATA_MODE=http` 缺失关键配置不会静默回退 Mock。
- [ ] README 提供本地运行、配置说明、千帆接入步骤及排错方法。

### 业务演示验收

在模拟数据中，千帆能够完成以下操作：

1. 查询 `SYS001` 对应的服务器及数据库配置项；
2. 查询其指定时间段内的变更记录，返回工单号、时间与关联配置项；
3. 查询相同时间窗口内的告警记录；
4. 将查询结果结合上传的系统维护手册整理为**疑似关联变更和排查建议**，引用相关工单号/告警 ID，且不能将时间相近直接视为因果关系。

> 第 4 点属于千帆工作流配置及分析效果验收，不是 MCP Server 必须自行实现的功能。

## 11. 待一体化运维平台调研确认的问题

| 调研项 | 需要获取的信息 |
|---|---|
| CMDB API | 查询系统、配置项、依赖关系的路径/字段/筛选条件 |
| 变更 API | 工单和启停记录的接口，计划/实际时间字段、工单状态与配置项关联方式 |
| 告警 API | 查询时间范围、告警对象、级别、首次/末次发生时间、重复次数 |
| 统一标识 | `system_id`、`ci_id`、IP、实例 ID 能否跨接口关联 |
| 鉴权方式 | Token、API Key/Secret、自定义 Header、服务账号授权范围 |
| 数据与限制 | 分页、限流、超时、单次最大返回条数、日志脱敏规则 |
| 网络与审批 | MCP Server 部署区、千帆访问路径、平台授权与安全审批 |
| 文档资料 | 启停维护手册能否在千帆工作流中按系统/版本稳定检索 |

## 12. 后续扩展（不纳入 MVP）

- `query_logs`：日志摘要与错误聚合（需要日志 API 和结果限流方案）；
- `get_system_topology`：更完整的 CMDB 依赖拓扑；
- `search_operation_records`：独立查询启停/操作审计记录；
- `search_runbook`：按系统、版本、故障类型检索手册；
- 多租户/按用户精细授权、标准 OAuth 集成、请求限流与网关接入；
- 性能指标、Tracing 和生产可用性增强。

---

**交付重点：** 先确保“百度千帆 -> MCP -> 三个 Mock 查询工具”完整跑通，再根据行内一体化运维平台的接口调研结果接入真实数据。不要提前开发与当前目标无关的复杂功能。
