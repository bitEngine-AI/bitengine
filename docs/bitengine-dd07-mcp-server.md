# DD-07：MCP Server 详细设计

> 模块路径：`internal/mcp/` | 完整覆盖 v1.0 · v1.5 · v2.0
>
> **v7 重构**：DD-07 从"协议集成层全覆盖"收窄为 **MCP Server 统一控制面**。A2H 网关 + A2A 编排 → DD-03，AG-UI + A2UI 渲染 → DD-08，事件总线 / WebSocket 保留为基础设施简述。新增 MCP Elicitation、无状态设计、A2A Agent Card 暴露、生态兼容性。

---

## 1 模块职责与定位升级

**v7 核心变更：MCP 从"对外暴露平台能力的协议"升级为"平台统一控制面协议"。**

不仅对外暴露给 Open WebUI / HA LLM Agent / Claude Desktop，也是内部 Intent Engine 调用设备、应用、数据能力的**唯一接口**。

```
修改前：Intent Engine → 内部 API → 各子系统
修改后：Intent Engine → MCP tool call → MCP Server → 各子系统
```

| 子系统 | 职责 | 阶段 |
|--------|------|------|
| server/app | App Manager MCP Server（应用生命周期管理） | v1.0 |
| server/data | Data & RAG MCP Server（数据查询、RAG 检索、知识库） | v1.0 |
| server/system | System MCP Server（系统状态、用户管理、Workspace） | v1.0 |
| server/device | Device Aggregator MCP Server（设备控制聚合路由，详见 DD-05） | v1.0 |
| client | MCP Client Manager（调用外部 MCP Server） | v1.0 |
| elicitation | MCP Elicitation 实现（意图协商 + 操作确认） | v1.0 |
| openai_api | OpenAI 兼容 API (`/v1/chat/completions`) | v1.0 |
| server/workflow | Workflow MCP Server（工作流管理，E-02 固化后） | v2.0 |

**迁移说明**：

| 旧 DD-07 章节 | 新归属 | 原因 |
|--------------|--------|------|
| §4-5 A2H 网关 + 多渠道 | **DD-03** A2H + A2A | A2H 是人机协作层，与 A2A 编排同属 DD-03 |
| §8 AG-UI 事件流 | **DD-08** 前端架构 | AG-UI 是前端运行时协议 |
| §9 A2UI 渲染协议 | **DD-08** 前端架构 | A2UI 是前端声明式 UI |
| §11-12 A2A 编排 + 联邦 | **DD-03** A2H + A2A | A2A 是 Agent 间通信协议 |
| §13 聊天桥接 | **DD-08** 前端架构 | 聊天入口是前端形态 |

---

## 2 MCP Server 完整清单

BitEngine 作为 MCP Server 暴露的能力矩阵：

| MCP Server | 模块路径 | 职责 | 对应 DD | 阶段 |
|-----------|---------|------|---------|------|
| **App Manager** | `server/app` | 应用生命周期：创建/构建/部署/启停/删除/查询 | DD-01 | v1.0 |
| **Data & RAG** | `server/data` | 数据查询、RAG 检索、知识库管理、自然语言查询 | DD-04 | v1.0 |
| **System** | `server/system` | 系统状态、用户管理、Workspace 操作、备份/恢复 | DD-01 | v1.0 |
| **Device Aggregator** | DD-05 `aggregator` | 设备控制聚合路由（详见 DD-05 §3） | DD-05 | v1.0 |
| **MQTT Direct Provider** | DD-05 `provider/mqtt` | 内置 MQTT 5.0/HTTP 设备直连 | DD-05 | v1.0 |
| **HA Provider** | DD-05 `provider/ha` | Home Assistant 设备桥接 | DD-05/DD-10 | v1.0 |
| **EdgeX Provider** | DD-05 `provider/edgex` | EdgeX Foundry 工业设备桥接 | DD-05/DD-10 | v2.0 |
| **Workflow** | `server/workflow` | 工作流管理（E-02 固化后） | Enhancement | v2.0 |

所有 MCP Server 注册到 DD-01 的 MCP Server Registry，统一管理。

---

## 3 MCP Server 核心实现 (v1.0)

### 3.1 工具注册

```go
// internal/mcp/server/app/server.go

type AppManagerMCPServer struct {
    appCenter       AppCenter
    port            int  // 9100
}

func (s *AppManagerMCPServer) ListTools() []ToolSchema {
    return []ToolSchema{
        {Name: "apps/list", Description: "列出所有应用", InputSchema: nil},
        {Name: "apps/create", Description: "创建新应用", InputSchema: createAppSchema},
        {Name: "apps/query", Description: "查询应用数据", InputSchema: queryAppSchema},
        {Name: "apps/control", Description: "启动/停止/删除应用", InputSchema: controlAppSchema},
        {Name: "apps/logs", Description: "查看应用日志", InputSchema: logsSchema},
        {Name: "apps/update", Description: "AI 迭代更新应用", InputSchema: updateAppSchema},
    }
}
```

```go
// internal/mcp/server/data/server.go

type DataRAGMCPServer struct {
    dataHub   DataHub
    ragEngine RAGEngine
}

func (s *DataRAGMCPServer) ListTools() []ToolSchema {
    return []ToolSchema{
        {Name: "data/query", Description: "自然语言查询数据", InputSchema: nlQuerySchema},
        {Name: "data/search", Description: "语义搜索知识库", InputSchema: searchSchema},
        {Name: "data/files", Description: "文件管理操作", InputSchema: fileSchema},
        {Name: "data/kb/upload", Description: "上传文档到知识库", InputSchema: kbUploadSchema},
    }
}
```

```go
// internal/mcp/server/system/server.go

type SystemMCPServer struct {
    foundation Foundation
}

func (s *SystemMCPServer) ListTools() []ToolSchema {
    return []ToolSchema{
        {Name: "system/status", Description: "系统状态（CPU/内存/磁盘/应用健康）", InputSchema: nil},
        {Name: "system/users", Description: "用户管理", InputSchema: usersSchema},
        {Name: "system/backup", Description: "触发备份/恢复", InputSchema: backupSchema},
        {Name: "system/workspace", Description: "Workspace 管理", InputSchema: workspaceSchema},
    }
}
```

### 3.2 JSON-RPC 处理（Streamable HTTP）

```go
// internal/mcp/server/transport.go

// MCP 使用 JSON-RPC 2.0 over Streamable HTTP

type MCPRequest struct {
    JSONRPC string          `json:"jsonrpc"` // "2.0"
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
    ID      interface{}     `json:"id"`
}

type MCPResponse struct {
    JSONRPC string      `json:"jsonrpc"` // "2.0"
    Result  interface{} `json:"result,omitempty"`
    Error   *MCPError   `json:"error,omitempty"`
    ID      interface{} `json:"id"`
}

// 统一 MCP Server 入口：路由到对应子 Server
type MCPServerRouter struct {
    servers map[string]MCPServerHandler  // "apps" → AppManager, "data" → DataRAG, etc.
}

func (r *MCPServerRouter) HandleRequest(w http.ResponseWriter, req *http.Request) {
    var mcpReq MCPRequest
    json.NewDecoder(req.Body).Decode(&mcpReq)
    
    switch mcpReq.Method {
    case "initialize":
        r.handleInitialize(w, mcpReq)
    case "tools/list":
        r.handleToolsList(w, mcpReq)
    case "tools/call":
        r.handleToolCall(w, mcpReq)
    case "resources/list":
        r.handleResourcesList(w, mcpReq)
    case "resources/read":
        r.handleResourceRead(w, mcpReq)
    case "elicitation/create":      // v7 新增
        r.handleElicitation(w, mcpReq)
    default:
        writeError(w, mcpReq.ID, -32601, "Method not found")
    }
}

func (r *MCPServerRouter) handleToolCall(w http.ResponseWriter, req MCPRequest) {
    var call ToolCall
    json.Unmarshal(req.Params, &call)
    
    // 按 tool 前缀路由到对应子 Server
    prefix := strings.Split(call.Name, "/")[0]  // "apps" | "data" | "iot" | "system"
    server, ok := r.servers[prefix]
    if !ok {
        writeError(w, req.ID, -32602, "Unknown tool prefix: "+prefix)
        return
    }
    
    result, err := server.ExecuteTool(context.Background(), call.Name, call.Arguments)
    if err != nil {
        writeError(w, req.ID, -32000, err.Error())
        return
    }
    writeResult(w, req.ID, result)
}
```

### 3.3 MCP Resources（平台数据暴露）

```go
func (r *MCPServerRouter) handleResourcesList(w http.ResponseWriter, req MCPRequest) {
    resources := []ResourceSchema{
        {URI: "bitengine://apps", Name: "应用列表", MimeType: "application/json"},
        {URI: "bitengine://system/metrics", Name: "系统指标", MimeType: "application/json"},
        {URI: "bitengine://iot/devices", Name: "IoT 设备状态", MimeType: "application/json"},
        {URI: "bitengine://data/files", Name: "文件列表", MimeType: "application/json"},
    }
    writeResult(w, req.ID, map[string]interface{}{"resources": resources})
}
```

---

## 4 MCP Elicitation（v7 新增）

MCP 2025-06-18 规范新增 Elicitation 能力——MCP Server 可在工具执行中暂停，通过 Client 向用户请求结构化输入。BitEngine 用 Elicitation 实现意图协商和操作确认，**无需自建协商协议**。

### 4.1 两种模式

| 模式 | 用途 | 示例 |
|------|------|------|
| **Form mode** | 请求结构化输入（JSON Schema 验证） | 意图信息不足时请求补充；高风险设备操作确认 |
| **URL mode** | 引导用户到外部页面完成流程 | HA OAuth 授权；安全页面输入 API Key |

### 4.2 Form Mode 实现

```go
// internal/mcp/elicitation/handler.go

type ElicitationHandler struct {
    pendingRequests map[string]*ElicitationRequest  // requestID → pending
    mu              sync.RWMutex
}

type ElicitationRequest struct {
    ID              string          `json:"id"`
    Message         string          `json:"message"`
    RequestedSchema json.RawMessage `json:"requestedSchema"`  // JSON Schema
    CreatedAt       time.Time       `json:"created_at"`
    ResponseCh      chan *ElicitationResponse
}

type ElicitationResponse struct {
    Action string          `json:"action"`  // "accept" | "decline" | "cancel"
    Data   json.RawMessage `json:"data,omitempty"`
}

// MCP Server 发起 Elicitation（工具执行中暂停）
func (h *ElicitationHandler) RequestInput(ctx context.Context, message string, schema json.RawMessage) (*ElicitationResponse, error) {
    req := &ElicitationRequest{
        ID:              uuid.New().String(),
        Message:         message,
        RequestedSchema: schema,
        CreatedAt:       time.Now(),
        ResponseCh:      make(chan *ElicitationResponse, 1),
    }
    
    h.mu.Lock()
    h.pendingRequests[req.ID] = req
    h.mu.Unlock()
    
    // 发送 elicitation/create 请求给 MCP Client
    // Client 原生渲染表单（任何 MCP 兼容客户端都支持）
    
    select {
    case resp := <-req.ResponseCh:
        return resp, nil
    case <-ctx.Done():
        return nil, ctx.Err()
    case <-time.After(5 * time.Minute):
        return nil, fmt.Errorf("elicitation timeout")
    }
}
```

### 4.3 意图协商场景

```go
// Intent Engine 中使用 Elicitation 替代自建协商
func (e *IntentEngine) handleAppGeneration(ctx context.Context, rawIntent string) error {
    completeness := e.assessCompleteness(rawIntent)
    
    if completeness < 0.7 {  // 应用生成阈值
        // 通过 MCP Elicitation 请求补充信息
        schema := json.RawMessage(`{
            "type": "object",
            "properties": {
                "scale": {"type": "string", "description": "商品数量", "enum": ["< 50", "50-500", "> 500"]},
                "payment": {"type": "string", "description": "需要支付功能吗？", "enum": ["是", "否", "待定"]},
                "style": {"type": "string", "description": "设计偏好", "enum": ["简约", "商务", "品牌定制"]}
            }
        }`)
        
        resp, err := e.elicitation.RequestInput(ctx, "电商网站需求确认", schema)
        if err != nil || resp.Action != "accept" {
            return fmt.Errorf("intent negotiation cancelled")
        }
        
        // 将补充信息合并到意图
        rawIntent = enrichIntent(rawIntent, resp.Data)
    }
    
    return e.executeAppGeneration(ctx, rawIntent)
}
```

### 4.4 URL Mode（外部授权）

```go
// HA Provider 需要 OAuth 授权时
func (h *HAProvider) requestOAuth(ctx context.Context) error {
    resp, err := h.elicitation.RequestURL(ctx, 
        "请授权 BitEngine 访问 Home Assistant",
        "https://homeassistant.local:8123/auth/authorize?client_id=bitengine",
    )
    // 用户在 HA 页面完成授权后，回调带回 token
    // URL 白名单机制防钓鱼
    return err
}
```

---

## 5 无状态设计（v7 新增）

MCP 协议正在向"应用有状态、协议无状态"方向演进（计划 2026-06 新规范）。BitEngine 的 MCP Server 从设计上考虑无状态兼容。

### 5.1 设计原则

| 原则 | 实现方式 |
|------|---------|
| 业务状态不依赖 MCP session | 所有状态存 PostgreSQL/Redis，MCP session 断开不丢失业务数据 |
| Elicitation 可重建状态 | Server 返回 Elicitation 请求时，Client 连同原始请求一起返回响应，Server 从消息重建上下文 |
| 元数据发现无需连接 | 为 `.well-known` URL 元数据发现做准备——MCP Server 发布能力描述无需建立 MCP session |
| Tool 调用幂等 | 关键操作（创建应用、删除设备）通过 idempotency key 实现幂等，重试安全 |

### 5.2 状态存储分离

```go
// 所有 MCP Server 通过 shared state store 访问业务状态
type MCPStateStore struct {
    pg    *pgxpool.Pool  // 持久状态
    redis *redis.Client  // 临时状态 / 缓存
}

// Elicitation 请求-响应设计为可重建状态
func (s *MCPStateStore) SaveElicitationContext(reqID string, ctx ElicitationContext) error {
    data, _ := json.Marshal(ctx)
    return s.redis.Set(context.Background(), "elicitation:"+reqID, data, 10*time.Minute).Err()
}

func (s *MCPStateStore) RestoreElicitationContext(reqID string) (*ElicitationContext, error) {
    data, err := s.redis.Get(context.Background(), "elicitation:"+reqID).Bytes()
    if err != nil {
        return nil, err
    }
    var ctx ElicitationContext
    json.Unmarshal(data, &ctx)
    return &ctx, nil
}
```

---

## 6 A2A Agent Card 暴露（v7 新增）

每个 MCP Server 同时发布 A2A Agent Card（`.well-known/agent.json`），使外部 A2A Agent 能发现和调用 BitEngine 的能力。

```go
// internal/mcp/a2a_card/handler.go

func HandleAgentCard(w http.ResponseWriter, r *http.Request) {
    card := AgentCard{
        Name:        "BitEngine",
        Description: "边缘智能平台——AI 生成应用、管理数据、控制设备",
        URL:         getBaseURL(),
        Skills: []Skill{
            {ID: "app_generation", Description: "Generate, build, and deploy containerized applications from natural language"},
            {ID: "data_query", Description: "Query and analyze data with natural language, RAG search"},
            {ID: "device_control", Description: "Control IoT devices through unified MCP interface"},
            {ID: "system_management", Description: "System status, backup, user management"},
        },
        Authentication: AuthConfig{
            Schemes:     []string{"oauth2"},
            CardSigning: true,  // v0.3 安全卡签名
        },
        Protocol: "a2a/v0.3",
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(card)
}
```

A2A Agent Card 发布在 `GET /.well-known/agent.json`，外部 A2A Agent（Salesforce Agent、企业内部 Agent 等）可通过此端点发现 BitEngine 的能力。实际的 A2A 协议通信处理在 DD-03 中实现。

---

## 7 MCP Client (v1.0)

调用外部 MCP Server（Gmail、GitHub 等第三方服务）。

```go
// internal/mcp/client/manager.go

type MCPClientManager struct {
    connections map[string]*MCPConnection  // connectionID → connection
    vault       *VaultService
    mu          sync.RWMutex
}

type MCPConnection struct {
    ID       string `json:"id"`
    URL      string `json:"url"`       // mcp://gmail.googleapis.com
    AuthType string `json:"auth_type"` // oauth2 | pat | apikey | none
    Status   string `json:"status"`    // connected | disconnected | error
}

func (m *MCPClientManager) Connect(ctx context.Context, url, authType string) (string, error) {
    // 建立 MCP 连接（Streamable HTTP）
    // 1. 发送 initialize 请求
    // 2. 获取 server capabilities（包括 Elicitation 支持）
    // 3. 缓存 tools/list 结果
    return connectionID, nil
}

func (m *MCPClientManager) CallTool(ctx context.Context, connID, toolName string, args map[string]any) (any, error) {
    conn := m.connections[connID]
    req := MCPRequest{
        JSONRPC: "2.0",
        Method:  "tools/call",
        Params:  mustMarshal(ToolCall{Name: toolName, Arguments: args}),
    }
    return conn.Send(ctx, req)
}

// 处理远程 Server 发来的 Elicitation 请求
func (m *MCPClientManager) HandleRemoteElicitation(ctx context.Context, connID string, req ElicitationRequest) (*ElicitationResponse, error) {
    // 将远程 Server 的 Elicitation 请求转发给前端渲染
    // 用户填写后返回响应
    return m.frontend.RenderElicitation(ctx, req)
}
```

---

## 8 OpenAI 兼容 API (v1.0)

提供标准 `/v1/chat/completions` 格式 API，让 Open WebUI 等第三方应用接入 BitEngine AI 能力。

```go
// internal/mcp/openai_api/handler.go

type OpenAICompatHandler struct {
    ai     AIEngine
    ollama *OllamaManager
    cloud  *CloudClient
}

type ChatCompletionRequest struct {
    Model    string         `json:"model"`    // bitengine-local | bitengine-cloud | 具体模型名
    Messages []ChatMessage  `json:"messages"`
    Stream   bool           `json:"stream"`
    Tools    []ToolDef      `json:"tools,omitempty"`
}

func (h *OpenAICompatHandler) HandleChatCompletion(w http.ResponseWriter, r *http.Request) {
    var req ChatCompletionRequest
    json.NewDecoder(r.Body).Decode(&req)
    
    model := h.resolveModel(req.Model)
    
    if req.Stream {
        h.streamResponse(w, model, req)
    } else {
        h.syncResponse(w, model, req)
    }
}

func (h *OpenAICompatHandler) resolveModel(model string) string {
    switch model {
    case "bitengine-local":
        return "qwen2.5:7b"
    case "bitengine-fast":
        return "qwen3:4b"
    case "bitengine-cloud":
        return "anthropic/claude-sonnet-4-5-20250929"
    default:
        return model
    }
}
```

---

## 9 事件基础设施

### 9.1 双通道事件架构（v7 变更）

v7 引入 MQTT 5.0 作为 IoT 数据面后，事件基础设施变为双通道：

| 通道 | 协议 | 用途 | 订阅方 |
|------|------|------|--------|
| **IoT 数据面** | MQTT 5.0 | 设备遥测、状态变更、告警 | 规则引擎、数据中心、前端 |
| **平台内部** | Redis PubSub | 应用事件、系统事件、A2H 响应 | 前端、审计、监控 |

```go
// Redis PubSub 保留用于平台内部事件（非 IoT）
type RedisEventBus struct {
    client *redis.Client
}

func (b *RedisEventBus) Publish(ctx context.Context, topic string, payload any) error {
    data, _ := json.Marshal(Event{Topic: topic, Payload: payload, Timestamp: time.Now()})
    return b.client.Publish(ctx, topic, data).Err()
}
```

### 9.2 平台内部事件主题

| 主题 | 发布方 | 订阅方 | 阶段 |
|------|--------|--------|------|
| `app.created` | AppCenter | Frontend, Audit | v1.0 |
| `app.started` / `app.stopped` | Runtime | Frontend, Monitoring | v1.0 |
| `app.generation.progress` | AppCenter | Frontend (SSE) | v1.0 |
| `app.health.changed` | Monitoring | Frontend, Runtime (自动重启) | v1.0 |
| `backup.completed` | Foundation | Frontend, Audit | v1.0 |
| `a2h.sent` / `a2h.responded` | A2H Gateway | Frontend, Audit | v1.0 |
| `data.import.completed` | Data Hub | Frontend | v1.0 |
| `kb.indexed` | Data Hub | Frontend | v1.0 |
| `module.installed` | Market | AppCenter, Frontend | v2.0 |
| `governance.alert` | Governance Agent | A2H, Audit | v2.0 |

**IoT 事件走 MQTT 5.0**（详见 DD-05 §5），不再走 Redis。

---

## 10 生态兼容性（v7 新增）

BitEngine 的 MCP Server 不是封闭系统，而是标准协议的开放平台：

| 外部客户端 | 对接方式 | 能力 |
|-----------|---------|------|
| **Open WebUI** | 在 MCP 配置中添加 BitEngine MCP Server URL | 设备控制、应用管理、数据查询 |
| **Claude Desktop** | MCP Server 配置 | 通过对话管理 BitEngine |
| **Home Assistant LLM Agent** | MCP 调用 | "在 HA 说一句话，BitEngine 生成应用" |
| **外部 A2A Agent** | Agent Card 发现 + A2A 协议 | Salesforce Agent 调用代码生成 |
| **Cursor / VS Code** | MCP 配置 | 开发者通过 IDE 管理 BitEngine |
| **任何 MCP 兼容客户端** | 标准 MCP 协议 | 无需理解 BitEngine 内部架构 |

**配置示例**（Open WebUI 添加 BitEngine）：

```json
{
  "mcp_servers": [
    {
      "name": "BitEngine",
      "url": "http://bitengine.local:9100/mcp",
      "auth": {"type": "bearer", "token": "<user_token>"}
    }
  ]
}
```

---

## 11 API 端点

| 方法 | 端点 | 说明 | 阶段 |
|------|------|------|------|
| POST | `/mcp` | MCP Server 统一入口（JSON-RPC 2.0 over Streamable HTTP） | v1.0 |
| GET | `/.well-known/agent.json` | A2A Agent Card（v7 新增） | v1.5 |
| GET | `/api/v1/mcp/servers` | MCP Server 清单（v7 新增） | v1.0 |
| GET | `/api/v1/mcp/connections` | MCP Client 连接列表 | v1.0 |
| POST | `/api/v1/mcp/connections` | 添加 MCP Client 连接 | v1.0 |
| DELETE | `/api/v1/mcp/connections/:id` | 断开 MCP 连接 | v1.0 |
| POST | `/api/v1/mcp/connections/:id/tools/:tool` | 调用远程 MCP 工具 | v1.0 |
| POST | `/v1/chat/completions` | OpenAI 兼容 API | v1.0 |
| GET | `/v1/models` | OpenAI 兼容模型列表 | v1.0 |

**已迁移到其他 DD 的端点**：

| 端点 | 新归属 |
|------|--------|
| `/api/v1/a2h/*` | DD-03 |
| `/ws` (AG-UI + A2H) | DD-08 |
| `/api/v1/a2a/*` | DD-03 |
| `/api/v1/chatbridge/*` | DD-08 |

---

## 12 错误码

| 错误码 | 说明 | 阶段 |
|--------|------|------|
| `MCP_TOOL_NOT_FOUND` | MCP 工具未注册 | v1.0 |
| `MCP_TOOL_EXEC_FAILED` | MCP 工具执行失败 | v1.0 |
| `MCP_SERVER_NOT_FOUND` | MCP Server 不存在（v7 新增） | v1.0 |
| `MCP_CONNECTION_FAILED` | MCP Client 连接失败 | v1.0 |
| `MCP_AUTH_FAILED` | MCP Client 认证失败 | v1.0 |
| `MCP_ELICITATION_TIMEOUT` | Elicitation 响应超时（v7 新增） | v1.0 |
| `MCP_ELICITATION_DECLINED` | 用户拒绝 Elicitation 请求（v7 新增） | v1.0 |
| `MCP_ELICITATION_URL_BLOCKED` | URL mode 目标不在白名单（v7 新增） | v1.0 |
| `OPENAI_API_MODEL_NOT_FOUND` | OpenAI 兼容 API 模型不存在 | v1.0 |
| `EVENTBUS_PUBLISH_FAILED` | 事件发布失败 | v1.0 |

**已迁移到其他 DD 的错误码**：A2H 相关 → DD-03，AG-UI/A2UI 相关 → DD-08，A2A 相关 → DD-03。

---

## 13 测试策略

| 类型 | 覆盖 | 工具 |
|------|------|------|
| 单元测试 | MCP JSON-RPC 解析、tool 路由、Elicitation Schema 验证 | `testing` + `testify` |
| 集成测试 | MCP Server 暴露 tools → Mock Client 调用 → 正确路由到子 Server | Mock MCP Client |
| 集成测试 | MCP Elicitation 完整流程：Server 发起 → Client 渲染 → 用户填写 → 响应返回 | Mock Client + Schema 验证 |
| 集成测试 | MCP Client 连接外部 Server → tools/list → tools/call | Mock MCP Server |
| 集成测试 | OpenAI 兼容 API 流式/同步响应 | `net/http/httptest` |
| 集成测试 | Redis PubSub 发布 → 订阅 → 接收 | `testcontainers-go` (Redis) |
| 契约测试 | A2A Agent Card 格式符合 Google A2A v0.3 规范 | JSON Schema 验证 |
| 安全测试 | Elicitation URL mode 白名单机制（非白名单 URL → 拒绝） | 白名单绕过测试 |
