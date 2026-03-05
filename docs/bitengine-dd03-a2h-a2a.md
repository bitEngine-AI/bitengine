# DD-03：A2H 人机协作 + A2A 多 Agent 编排详细设计

> 模块路径：`internal/a2h/` + `internal/a2a/` | 完整覆盖 MVP · v1.0 · v1.5 · v2.0
>
> **v7 新建文档**：从原 DD-07（协议集成层）抽出 A2H 网关 + A2A 编排，合并为独立的人机协作与 Agent 编排文档。A2A 通信层对齐 Google A2A 协议（Linux Foundation 标准，v0.3）。A2H 确认 UI 对齐 A2UI 声明式渲染。
>
> **核心原则：编排逻辑是我们的护城河，通信协议是标准的。**

---

## 1 模块职责

A2H 是 Agent 向人类回传的通道（问审批、发通知、收集信息）。A2A 是 Agent 之间协作的通道（拆分任务、委派专家、聚合结果）。两者共同构成 BitEngine 的"人机协作与多 Agent 编排"层——竞品无覆盖的护城河。

| 子系统 | 职责 | 阶段 |
|--------|------|------|
| a2h/gateway | A2H 网关核心（五种原子意图 + 策略路由 + JWS 证据） | MVP |
| a2h/channels/web | Web 控制台渠道 | MVP |
| a2h/channels/wechat | 微信渠道（Server 酱 / 企业微信） | v1.0 |
| a2h/channels/telegram | Telegram Bot 渠道 | v1.0 |
| a2h/channels/dingtalk | 钉钉渠道 | v1.0 |
| a2h/channels/email | 邮件渠道 | v1.0 |
| a2h/channels/webhook | 自定义 Webhook 渠道 | v1.0 |
| a2h/evidence | JWS 加密证据（Ed25519 签名 → Merkle 审计链） | MVP |
| **a2a/protocol** | **Google A2A 协议实现（v0.3，v7 新增）** | **v1.5** |
| **a2a/orchestrator** | **编排逻辑：任务分解 / Agent 调度 / 结果聚合（护城河）** | **v1.5** |
| a2a/federation | 跨设备 BitEngine 实例联邦 | v2.0 |
| **governance** | **Governance Agent（AI 监控 AI 行为合规，从 DD-06 迁入）** | **v2.0** |

---

## 2 A2H 网关 (MVP)

### 2.1 五种原子意图

```go
// internal/a2h/gateway.go

type A2HIntentType string

const (
    A2HInform    A2HIntentType = "INFORM"    // 即发即忘通知
    A2HCollect   A2HIntentType = "COLLECT"   // 收集数据（阻塞等待）
    A2HAuthorize A2HIntentType = "AUTHORIZE" // 请求授权（带加密证据）
    A2HEscalate  A2HIntentType = "ESCALATE"  // 升级处理
    A2HResult    A2HIntentType = "RESULT"    // 返回执行结果
)

type A2HMessage struct {
    ID        string        `json:"id"`
    Intent    A2HIntentType `json:"intent"`
    Title     string        `json:"title"`
    Body      string        `json:"body"`
    Risk      string        `json:"risk"`      // low | medium | high
    Options   []A2HOption   `json:"options,omitempty"`
    Timeout   time.Duration `json:"timeout"`
    ChannelID string        `json:"channel_id,omitempty"`
    A2UISpec  *A2UISpec     `json:"a2ui_spec,omitempty"` // v7: A2UI 声明式渲染
}

type A2HOption struct {
    Label string `json:"label"`
    Value string `json:"value"`
    Style string `json:"style"` // primary | danger | default
}

type A2HResponse struct {
    MessageID   string    `json:"message_id"`
    Value       string    `json:"value"`
    Evidence    *JWSProof `json:"evidence,omitempty"`
    RespondedAt time.Time `json:"responded_at"`
    Channel     string    `json:"channel"`
}
```

### 2.2 网关核心

```go
type A2HGateway struct {
    wsHub    *WebSocketHub
    channels map[string]A2HChannel
    evidence *EvidenceService
    audit    AuditChain
    eventBus EventBus
    policy   *A2HPolicy
}

type A2HChannel interface {
    Name() string
    Send(ctx context.Context, msg A2HMessage) error
    ReceiveResponse(ctx context.Context, msgID string, timeout time.Duration) (*A2HResponse, error)
    Available() bool
}

func (g *A2HGateway) Send(ctx context.Context, msg A2HMessage) (*A2HResponse, error) {
    // 1. 策略检查：风险等级 → 认证方式
    authLevel := g.policy.RequiredAuth(msg.Risk)
    
    // 2. 渠道路由（降级策略）
    channel := g.selectChannel(msg)
    
    // 3. 发送（v7：如果有 A2UISpec，通过 AG-UI 事件流传输声明式 UI）
    if err := channel.Send(ctx, msg); err != nil {
        channel = g.fallbackChannel(channel)
        channel.Send(ctx, msg)
    }
    
    // 4. 等待响应（INFORM 即发即忘）
    if msg.Intent == A2HInform {
        return nil, nil
    }
    
    resp, err := channel.ReceiveResponse(ctx, msg.ID, msg.Timeout)
    if err != nil {
        return nil, fmt.Errorf("a2h: timeout waiting for response: %w", err)
    }
    
    // 5. AUTHORIZE → 验证身份 + JWS 证据 + Merkle 审计链
    if msg.Intent == A2HAuthorize {
        evidence, _ := g.evidence.Sign(ctx, resp, authLevel)
        resp.Evidence = evidence
        g.audit.Append(ctx, AuditEntry{
            EventType: "a2h.authorize",
            Detail:    fmt.Sprintf("msg=%s value=%s", msg.ID, resp.Value),
            Evidence:  evidence,
        })
    }
    
    return resp, nil
}
```

### 2.3 A2H 确认 UI 对齐 A2UI（v7 变更）

```
修改前（自定义）：
  Governance Agent → 自定义 JSON → 前端硬编码 ConfirmDialog 组件

修改后（标准 A2UI）：
  Governance Agent → A2UI JSON 描述（Card + 风险等级 + 审批按钮）
  → AG-UI 事件流传输到前端
  → DD-08 A2UI Renderer 渲染为原生组件
```

好处：A2H 确认界面不再绑定我们的前端。任何支持 A2UI 的客户端（Open WebUI 插件、HA Dashboard、移动 App）都能渲染确认弹窗。

### 2.4 A2H 策略配置

```go
type A2HPolicy struct {
    AssuranceLevels map[string]AssuranceLevel `json:"assurance_levels"`
    ChannelFallback []string                  `json:"channel_fallback"`
    DefaultTimeout  time.Duration             `json:"default_timeout"`
}

type AssuranceLevel struct {
    Auth     string `json:"auth"`     // passkey | pin | none
    Evidence string `json:"evidence"` // jws_signed | logged_only
    Audit    string `json:"audit"`    // merkle_chain | log_only
}

var DefaultA2HPolicy = A2HPolicy{
    AssuranceLevels: map[string]AssuranceLevel{
        "high":   {Auth: "passkey", Evidence: "jws_signed", Audit: "merkle_chain"},
        "medium": {Auth: "pin", Evidence: "jws_signed", Audit: "merkle_chain"},
        "low":    {Auth: "none", Evidence: "logged_only", Audit: "log_only"},
    },
    ChannelFallback: []string{"web"},
    DefaultTimeout:  5 * time.Minute,
}
```

### 2.5 JWS 加密证据

```go
// internal/a2h/evidence.go

type JWSProof struct {
    Header    string `json:"protected"`
    Payload   string `json:"payload"`
    Signature string `json:"signature"`
}

type EvidenceService struct {
    privateKey ed25519.PrivateKey
}

func (e *EvidenceService) Sign(ctx context.Context, resp *A2HResponse, authLevel AssuranceLevel) (*JWSProof, error) {
    payload := map[string]any{
        "message_id":   resp.MessageID,
        "value":        resp.Value,
        "responded_at": resp.RespondedAt.Unix(),
        "channel":      resp.Channel,
        "auth_method":  authLevel.Auth,
    }
    payloadBytes, _ := json.Marshal(payload)
    header := base64url(mustMarshal(map[string]string{"alg": "EdDSA", "typ": "JWT"}))
    payloadB64 := base64url(payloadBytes)
    signature := ed25519.Sign(e.privateKey, []byte(header+"."+payloadB64))
    
    return &JWSProof{Header: header, Payload: payloadB64, Signature: base64url(signature)}, nil
}
```

---

## 3 A2H 多渠道实现 (v1.0)

### 3.1 Web 控制台渠道 (MVP)

```go
// internal/a2h/channels/web.go

type WebChannel struct {
    wsHub   *WebSocketHub
    pending map[string]chan *A2HResponse
}

func (c *WebChannel) Name() string { return "web" }

func (c *WebChannel) Send(ctx context.Context, msg A2HMessage) error {
    c.wsHub.SendTo(msg.SessionID, WSEvent{Type: "a2h", Data: msg})
    return nil
}

func (c *WebChannel) ReceiveResponse(ctx context.Context, msgID string, timeout time.Duration) (*A2HResponse, error) {
    ch := make(chan *A2HResponse, 1)
    c.pending[msgID] = ch
    defer delete(c.pending, msgID)
    select {
    case resp := <-ch:
        return resp, nil
    case <-time.After(timeout):
        return nil, ErrA2HTimeout
    }
}
```

### 3.2 微信渠道 (v1.0)

```go
// internal/a2h/channels/wechat.go

type WeChatChannel struct {
    webhookURL string
    httpClient *http.Client
}

func (c *WeChatChannel) Name() string { return "wechat" }

func (c *WeChatChannel) Send(ctx context.Context, msg A2HMessage) error {
    payload := map[string]string{"title": msg.Title, "desp": c.formatMarkdown(msg)}
    body, _ := json.Marshal(payload)
    _, err := c.httpClient.Post(c.webhookURL, "application/json", bytes.NewReader(body))
    return err
}
```

### 3.3 Telegram / 钉钉 / 邮件 / Webhook 渠道 (v1.0)

实现模式与微信渠道一致——每个渠道实现 `A2HChannel` 接口。Telegram 使用 Bot API + inline keyboard，钉钉使用 Webhook + HMAC 签名，邮件使用 SMTP，Webhook 使用 HTTP POST + 自定义 headers。详细实现见各渠道源码。

### 3.4 渠道配置数据库

```sql
CREATE TABLE a2h_channels (
    id           TEXT PRIMARY KEY DEFAULT gen_ulid(),
    name         TEXT NOT NULL,
    display_name TEXT NOT NULL,
    config       JSONB NOT NULL,
    enabled      BOOLEAN NOT NULL DEFAULT true,
    priority     INTEGER NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE a2h_messages (
    id           TEXT PRIMARY KEY DEFAULT gen_ulid(),
    intent       TEXT NOT NULL,
    title        TEXT NOT NULL,
    body         TEXT NOT NULL,
    risk         TEXT NOT NULL DEFAULT 'low',
    options      JSONB,
    channel      TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'sent',
    response     JSONB,
    evidence     JSONB,
    a2ui_spec    JSONB,              -- v7: A2UI 声明式 UI 描述
    sent_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    responded_at TIMESTAMPTZ
);
```

---

## 4 A2A 协议实现 — 对齐 Google A2A v0.3 (v1.5)

### 4.1 为什么对齐 Google A2A

Google A2A 已成为 Linux Foundation 行业标准，150+ 企业支持。不对齐意味着我们的 Agent 无法与任何外部 A2A 兼容 Agent 互操作。

**关键区分：A2A 定义通信协议，不做编排。** 编排逻辑（谁做什么、怎么分配、做完怎么验证）是我们的护城河，A2A 只解决"Agent 之间怎么说话"。

### 4.2 A2A 核心概念映射

| A2A 概念 | BitEngine 已有设计 | 映射关系 |
|---------|-------------------|---------|
| Agent Card | MCP Server Registry | 每个 Agent 发布 Agent Card（`.well-known/agent.json`） |
| Task | Orchestrator 任务 | 生命周期：submitted → working → input-required → completed |
| Message / Artifact | Agent 间消息和执行产物 | 标准 JSON-RPC 2.0 消息格式 |
| input-required 状态 | A2H 确认 | Agent 需要人工输入时挂起任务，转 A2H |

### 4.3 A2A 协议实现

```go
// internal/a2a/protocol.go

// Google A2A v0.3 标准消息格式
type A2ATaskMessage struct {
    JSONRPC string    `json:"jsonrpc"` // "2.0"
    Method  string    `json:"method"`  // tasks/send | tasks/get | tasks/cancel
    Params  A2AParams `json:"params"`
    ID      string    `json:"id"`
}

type A2AParams struct {
    TaskID           string        `json:"id"`
    Message          A2AMessage    `json:"message"`
    PushNotification *PushConfig   `json:"pushNotification,omitempty"`
}

type A2AMessage struct {
    Role  string    `json:"role"` // user | agent
    Parts []A2APart `json:"parts"`
}

type A2APart struct {
    Type string `json:"type"` // text | data | file
    Text string `json:"text,omitempty"`
    Data any    `json:"data,omitempty"`
}

// A2A Task 生命周期（v7：标准状态机）
type A2ATaskState string
const (
    A2ATaskSubmitted     A2ATaskState = "submitted"
    A2ATaskWorking       A2ATaskState = "working"
    A2ATaskInputRequired A2ATaskState = "input-required" // → 转 A2H
    A2ATaskCompleted     A2ATaskState = "completed"
    A2ATaskFailed        A2ATaskState = "failed"
    A2ATaskCanceled      A2ATaskState = "canceled"
)

// Agent Card — 对齐 Google A2A v0.3
type AgentCard struct {
    Name           string       `json:"name"`
    Description    string       `json:"description"`
    URL            string       `json:"url"`
    Skills         []AgentSkill `json:"skills"`
    Authentication AuthConfig   `json:"authentication"`
    Protocol       string       `json:"protocol"` // "a2a/v0.3"
}

type AgentSkill struct {
    ID          string `json:"id"`
    Description string `json:"description"`
}

type AuthConfig struct {
    Schemes     []string `json:"schemes"`      // ["oauth2"]
    CardSigning bool     `json:"card_signing"` // v0.3 安全卡签名
}
```

### 4.4 A2A Server（接收外部 Agent 请求）

```go
// internal/a2a/server.go

type A2AServer struct {
    orchestrator *Orchestrator
    a2h          *A2HGateway
    audit        AuditChain
}

func (s *A2AServer) HandleTaskSend(w http.ResponseWriter, r *http.Request) {
    var msg A2ATaskMessage
    json.NewDecoder(r.Body).Decode(&msg)
    
    // 1. 验证 Agent Card 签名（DD-06 安全要求）
    if !s.verifyAgentCard(r) {
        writeA2AError(w, "unauthorized")
        return
    }
    
    // 2. 所有外部来源的请求都必须过 Governance Agent 风险评估
    // 不因来源是"Agent"就跳过 A2H 审批
    risk := s.orchestrator.AssessRisk(msg)
    if risk == "high" {
        // A2H 确认
        approved, _ := s.a2h.Send(r.Context(), A2HMessage{
            Intent: A2HAuthorize,
            Title:  fmt.Sprintf("外部 Agent 请求: %s", msg.Params.Message.Parts[0].Text),
            Risk:   risk,
        })
        if approved == nil || approved.Value != "approve" {
            writeA2AError(w, "rejected by human")
            return
        }
    }
    
    // 3. 交给编排器处理
    taskID := s.orchestrator.Submit(r.Context(), msg)
    
    // 4. 审计
    s.audit.Append(r.Context(), AuditEntry{EventType: "a2a.task.received", Detail: taskID})
    
    writeA2AResult(w, map[string]string{"task_id": taskID, "state": string(A2ATaskSubmitted)})
}
```

---

## 5 A2A 编排器 — 护城河 (v1.5)

编排逻辑是核心竞争力。Google A2A 只解决通信，以下全部是我们自己的设计。

### 5.1 任务分解

```go
// internal/a2a/orchestrator.go

type Orchestrator struct {
    agents   map[string]AgentEndpoint  // agent_id → endpoint
    aiEngine AIEngine
    a2h      *A2HGateway
}

type AgentEndpoint struct {
    ID       string    `json:"id"`
    Card     AgentCard `json:"card"`
    Endpoint string    `json:"endpoint"`  // 内部 Agent 或外部 A2A Agent
    Internal bool      `json:"internal"`  // true=平台内部 Agent
}

func (o *Orchestrator) Submit(ctx context.Context, task A2ATaskMessage) string {
    taskID := uuid.New().String()
    
    // 1. 任务分解（AI 驱动）
    subtasks := o.decompose(ctx, task)
    
    // 2. Agent 调度（按 skill 匹配）
    assignments := o.assign(subtasks)
    
    // 3. 并行/串行执行
    results := o.execute(ctx, assignments)
    
    // 4. 结果聚合
    o.aggregate(ctx, taskID, results)
    
    return taskID
}

func (o *Orchestrator) decompose(ctx context.Context, task A2ATaskMessage) []SubTask {
    // 使用 AI 将复杂任务分解为子任务
    // 例："做银行监控系统" → [数据库设计, 前端生成, 后端生成, 测试]
    prompt := fmt.Sprintf("分解任务: %s\n可用 Agent: %s", task.Params.Message.Parts[0].Text, o.listAgentSkills())
    result, _ := o.aiEngine.StructuredOutput(ctx, prompt, subtaskSchema)
    var subtasks []SubTask
    json.Unmarshal(result, &subtasks)
    return subtasks
}

func (o *Orchestrator) assign(subtasks []SubTask) []Assignment {
    var assignments []Assignment
    for _, st := range subtasks {
        // 按 skill 匹配最佳 Agent
        agent := o.findBestAgent(st.RequiredSkill)
        assignments = append(assignments, Assignment{SubTask: st, Agent: agent})
    }
    return assignments
}

func (o *Orchestrator) execute(ctx context.Context, assignments []Assignment) []TaskResult {
    var results []TaskResult
    g, gCtx := errgroup.WithContext(ctx)
    var mu sync.Mutex
    
    for _, a := range assignments {
        assignment := a
        g.Go(func() error {
            // 通过标准 A2A 协议发送任务（内部和外部 Agent 统一接口）
            msg := A2ATaskMessage{
                JSONRPC: "2.0",
                Method:  "tasks/send",
                Params: A2AParams{
                    TaskID:  uuid.New().String(),
                    Message: A2AMessage{Role: "user", Parts: []A2APart{{Type: "text", Text: assignment.SubTask.Description}}},
                },
            }
            
            result, err := o.sendToAgent(gCtx, assignment.Agent, msg)
            mu.Lock()
            results = append(results, TaskResult{SubTask: assignment.SubTask, Result: result, Error: err})
            mu.Unlock()
            return nil
        })
    }
    g.Wait()
    return results
}
```

### 5.2 多级审批流程

```go
// 风险评估 → 审批等级 → 审批链
func (o *Orchestrator) AssessRisk(task A2ATaskMessage) string {
    // 根据任务类型、来源、影响范围评估风险等级
    // high: 删除应用、部署新应用、修改系统配置
    // medium: 修改应用、控制设备
    // low: 查询数据、读取状态
    return "medium"
}
```

---

## 6 A2A 跨设备联邦 (v2.0)

```go
// internal/a2a/federation.go

type FederationTransport struct {
    peers   map[string]*BitEnginePeer
    authKey []byte  // v7: 使用 A2A v0.3 安全卡签名替代 HMAC
}

type BitEnginePeer struct {
    ID         string      `json:"id"`
    Hostname   string      `json:"hostname"`
    IP         string      `json:"ip"`
    AgentCards []AgentCard `json:"agent_cards"` // 对端暴露的 Agent Card
    Online     bool        `json:"online"`
    LastSeen   time.Time   `json:"last_seen"`
}

func (f *FederationTransport) DiscoverPeers(ctx context.Context) ([]*BitEnginePeer, error) {
    // 方式 1: mDNS 局域网发现 _bitengine._tcp.local.
    // 方式 2: Tailscale 网络查找
    // 方式 3: A2A Agent Card 发现（v7 新增）
    // 方式 4: 手动配置远程 peer URL
    return nil, nil
}

func (f *FederationTransport) SendTask(ctx context.Context, peerID string, task A2ATaskMessage) (*A2ATaskMessage, error) {
    peer := f.peers[peerID]
    body, _ := json.Marshal(task)
    
    req, _ := http.NewRequestWithContext(ctx, "POST",
        fmt.Sprintf("https://%s/api/v1/a2a/tasks", peer.IP), bytes.NewReader(body))
    // v7: 使用 A2A v0.3 标准安全卡签名
    req.Header.Set("Authorization", "Bearer "+f.generateA2AToken(peerID))
    
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    var result A2ATaskMessage
    json.NewDecoder(resp.Body).Decode(&result)
    return &result, nil
}
```

---

## 7 Governance Agent (v1.5 基础版 / v2.0 完整版)（从 DD-06 迁入）

用 AI 监控 AI——Governance Agent 审查其他 Agent 的行为是否合规。

**v1.5 基础版**：规则匹配 + 审计记录（不依赖 AI 语义审查，纯规则驱动）。
**v2.0 完整版**：规则匹配 + AI 语义审查（Qwen3-4B 判断超出规则覆盖的异常行为）。

```go
// internal/a2a/governance/agent.go

type GovernanceAgent struct {
    aiEngine AIEngine       // v2.0: AI 语义审查
    audit    AuditChain
    a2h      *A2HGateway
    rules    []GovernanceRule
}

type GovernanceRule struct {
    Name      string `json:"name"`
    Condition string `json:"condition"` // "action.type == 'delete' && action.target == 'production'"
    Action    string `json:"action"`    // block | warn | log
}

func (g *GovernanceAgent) Review(ctx context.Context, agentID string, action AgentAction) (*ReviewResult, error) {
    // 1. 规则匹配（v1.5 基础版）
    for _, rule := range g.rules {
        if matchesCondition(rule.Condition, action) {
            switch rule.Action {
            case "block":
                return &ReviewResult{Allowed: false, Reason: rule.Name}, nil
            case "warn":
                g.a2h.Send(ctx, A2HMessage{Intent: A2HInform, Title: "Governance 警告", Body: rule.Name, Risk: "medium"})
            }
        }
    }
    
    // 2. AI 语义审查（v2.0 完整版：超出规则覆盖的行为）
    if g.aiEngine != nil {
        prompt := fmt.Sprintf("审查 Agent %s 的操作: %+v\n是否存在安全风险或异常行为?", agentID, action)
        result, _ := g.aiEngine.StructuredOutput(ctx, prompt, governanceSchema)
        var review ReviewResult
        json.Unmarshal(result, &review)
        if !review.Allowed {
            g.audit.Append(ctx, AuditEntry{EventType: "governance.review", Detail: fmt.Sprintf("agent=%s action=%+v ai_blocked", agentID, action)})
            return &review, nil
        }
    }
    
    // 3. 审计记录（v1.5 基础版即有）
    g.audit.Append(ctx, AuditEntry{EventType: "governance.review", Detail: fmt.Sprintf("agent=%s action=%+v allowed", agentID, action)})
    
    return &ReviewResult{Allowed: true}, nil
}
```

---

## 8 API 端点

**A2H API**：

| 方法 | 端点 | 说明 | 阶段 |
|------|------|------|------|
| POST | `/api/v1/a2h/send` | 发送 A2H 消息（内部） | MVP |
| POST | `/api/v1/a2h/respond/:id` | 响应 A2H 消息 | MVP |
| GET | `/api/v1/a2h/messages` | A2H 消息历史 | MVP |
| GET | `/api/v1/a2h/channels` | A2H 渠道列表 | v1.0 |
| POST | `/api/v1/a2h/channels` | 添加 A2H 渠道 | v1.0 |
| PUT | `/api/v1/a2h/channels/:id` | 更新渠道配置 | v1.0 |
| DELETE | `/api/v1/a2h/channels/:id` | 删除渠道 | v1.0 |
| PUT | `/api/v1/a2h/policy` | 更新 A2H 策略 | v1.0 |

**A2A API（v7 新增）**：

| 方法 | 端点 | 说明 | 阶段 |
|------|------|------|------|
| POST | `/api/v1/a2a/tasks` | 接收 A2A 任务（外部 Agent 入口） | v1.5 |
| GET | `/api/v1/a2a/tasks/:id` | 查询任务状态 | v1.5 |
| POST | `/api/v1/a2a/tasks/:id/cancel` | 取消任务 | v1.5 |
| GET | `/.well-known/agent.json` | A2A Agent Card（发现协议，DD-07 实现） | v1.5 |
| GET | `/api/v1/a2a/peers` | 联邦 Peer 列表 | v2.0 |
| POST | `/api/v1/a2a/governance/check` | Governance 治理检查 | v2.0 |

---

## 9 错误码

| 错误码 | 说明 | 阶段 |
|--------|------|------|
| `A2H_TIMEOUT` | A2H 响应超时 | MVP |
| `A2H_CHANNEL_UNAVAILABLE` | A2H 所有渠道不可用 | MVP |
| `A2H_EVIDENCE_INVALID` | JWS 证据验证失败 | MVP |
| `A2H_CHANNEL_CONFIG_INVALID` | 渠道配置无效 | v1.0 |
| `A2A_AGENT_CARD_INVALID` | Agent Card 签名验证失败（v7 新增） | v1.5 |
| `A2A_TASK_REJECTED` | A2A 任务被拒绝（Governance 或人工审批） | v1.5 |
| `A2A_TASK_FAILED` | A2A 子任务失败 | v1.5 |
| `A2A_ORCHESTRATION_FAILED` | 编排逻辑执行失败 | v1.5 |
| `A2A_PEER_UNREACHABLE` | 联邦 Peer 不可达 | v2.0 |
| `A2A_GOVERNANCE_REJECTED` | Governance Agent 拒绝 | v2.0 |

---

## 10 测试策略

| 类型 | 覆盖 | 工具 |
|------|------|------|
| 单元测试 | A2H JWS 签名验证、A2H 策略路由、A2A 消息格式校验 | `testing` + `testify` |
| 集成测试 | A2H Authorize 完整流程：发送→弹窗→确认→JWS 证据 | WebSocket 测试 |
| 集成测试 | A2A 编排：任务分解→Agent 调度→并行执行→结果聚合 | Mock Agent |
| 集成测试 | A2A 外部请求：Agent Card 验证→Governance 审查→A2H 审批→执行 | Mock External Agent |
| 契约测试 | A2A 消息格式符合 Google A2A v0.3 规范 | JSON Schema 验证 |
| 契约测试 | Agent Card 格式符合 `.well-known/agent.json` 规范 | JSON Schema 验证 |
| 安全测试 | 伪造 Agent Card → 拒绝；跳过 A2H 审批 → 拒绝 | 攻击模拟 |
| E2E | A2H 多渠道降级：web 不可用→微信→telegram→email | Mock 渠道 |
