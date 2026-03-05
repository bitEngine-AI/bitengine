# DD-02：AI 引擎详细设计

> 模块路径：`internal/ai/` + `internal/appcenter/` | 完整覆盖 MVP · v1.0 · v2.0
>
> **v7 中大改**：合并原 DD-06（AI 引擎）和原 DD-03（应用中心）为统一的 AI 引擎文档。新增 Model Router（多端点多模型路由）、Intent Input Adapter（多模态意图输入）、意图路由策略调整（应用生成优先）。AG-UI/A2UI 生成 → DD-08，A2A 编排 → DD-03，Governance → DD-03，WebLLM → DD-08。

---

## 1 模块职责

AI 引擎是平台"大脑"，负责意图理解、多模型路由、代码生成、安全审查、嵌入检索，以及应用生成全流水线。

| 子系统 | 职责 | 阶段 |
|--------|------|------|
| **model_router** | **Model Router 多端点路由（v7 新增）** | **v1.0** |
| **input_adapter** | **Intent Input Adapter 多模态意图输入（v7 新增）** | **v1.0** |
| router | Gemma3-1B 意图路由 (毫秒级分类) | MVP |
| intent | Qwen3-4B 深度意图理解 + Structured Output | MVP |
| codegen | 云端大模型代码生成 | MVP |
| review | Phi-4-mini 本地代码审查 | MVP |
| embedding | BGE-M3 向量嵌入 | MVP |
| cloud | 多云端 API 客户端 (Anthropic/DeepSeek/OpenAI) | MVP |
| structured | Structured Output 统一约束层 | MVP |
| generator | AI 生成流水线（意图→代码→审查→部署）（原 DD-03） | MVP |
| templates | 内置应用模板 5 个（原 DD-03） | MVP |
| iteration | AI 迭代已有应用 + 蓝绿部署（原 DD-03） | v1.0 |
| autopacks | 自主应用包 SOP 调度（原 DD-03） | v1.0 |
| ecosystem | OpenClaw / OpenFang 生态导入（原 DD-03） | v1.0 |
| injection | 提示注入扫描器 | v1.0 |

**迁出到其他 DD 的子系统**：

| 原子系统 | 新归属 | 原因 |
|---------|--------|------|
| agui（AG-UI 事件流生成） | DD-08 前端架构 | AG-UI 是前端运行时协议 |
| a2ui（A2UI 声明式 UI 生成） | DD-08 前端架构 | A2UI 是前端声明式 UI |
| a2a_agent（A2A 多 Agent 协作） | DD-03 A2H + A2A | A2A 是 Agent 间通信协议 |
| governance（Governance Agent） | DD-03 A2H + A2A | 治理是 Agent 编排层 |
| webllm（WebLLM 浏览器端推理） | DD-08 前端架构 | 浏览器端能力属前端 |

---

## 2 SLM 多模型矩阵 (MVP)

### 2.1 模型配置

```mermaid
graph LR
    INPUT["用户输入"] --> ROUTER["Gemma3-1B<br/>路由 ~100ms"]
    
    ROUTER -->|"复杂代码生成"| CLOUD["☁️ 云端大模型<br/>Claude Sonnet / DeepSeek / GPT-4.1"]
    ROUTER -->|"推理/意图"| QWEN["Qwen3-4B<br/>本地推理 ~2s"]
    ROUTER -->|"代码审查"| PHI["Phi-4-mini<br/>本地审查 ~3s"]
    ROUTER -->|"嵌入/检索"| BGE["BGE-M3<br/>本地嵌入 ~200ms"]
    ROUTER -->|"复杂本地推理"| QWEN7B["Qwen2.5-7B<br/>本地全能 ~5s"]
```

| 模型 | 参数 | 内存 | 用途 | 常驻策略 |
|------|------|------|------|---------|
| Gemma3-1B | 1B | ~1GB | 意图路由、快速分类 | 永久常驻 |
| Qwen3-4B | 4B | ~3GB | 意图理解、推理 | 永久常驻 |
| Phi-4-mini | 3.8B | ~3GB | 代码审查、配置生成 | 永久常驻 |
| BGE-M3 | 568M | ~1GB | 文本嵌入、RAG 检索 | 永久常驻 |
| Qwen2.5-7B | 7B | ~5GB | 复杂本地推理、中文优化 | 永久常驻 |
| **合计** | | **~13GB** | | 32GB 设备余 ~19GB |

### 2.2 Ollama 管理

```go
// internal/ai/router/ollama.go

type OllamaManager struct {
    baseURL string // http://localhost:11434
    client  *http.Client
}

var RequiredModels = []ModelSpec{
    {Name: "gemma3:1b", KeepAlive: -1},    // -1 = 永久常驻
    {Name: "qwen3:4b", KeepAlive: -1},
    {Name: "phi4-mini", KeepAlive: -1},
    {Name: "bge-m3", KeepAlive: -1},
    {Name: "qwen2.5:7b", KeepAlive: -1},
}

func (m *OllamaManager) Preload(ctx context.Context) error {
    for _, model := range RequiredModels {
        // POST /api/generate {"model": model.Name, "keep_alive": -1, "prompt": "warmup"}
        // 确保模型加载到内存并常驻
    }
    return nil
}

func (m *OllamaManager) Generate(ctx context.Context, model, prompt string, opts GenerateOpts) (string, error) {
    // POST /api/generate
    // opts 包含: temperature, max_tokens, format(json), system prompt
}

func (m *OllamaManager) Embed(ctx context.Context, model string, texts []string) ([][]float32, error) {
    // POST /api/embed {"model": "bge-m3", "input": texts}
}

func (m *OllamaManager) ListRunning(ctx context.Context) ([]RunningModel, error) {
    // GET /api/ps → 返回当前加载的模型和内存占用
}
```

### 2.3 Model Router（v7 新增）

当前设计只支持单 Ollama 实例。v7 新增 Model Router，支持多 Ollama 实例 + 多云端 API 自由切换。

```mermaid
graph TB
    IE["Intent Engine"] --> MR["Model Router"]
    MR -->|"意图分类（快/轻）"| OL1["Ollama 本地 qwen3:4b"]
    MR -->|"代码生成（重/精）"| OL2["Ollama 远程 qwen3:32b"]
    MR -->|"复杂推理（fallback）"| CLOUD["Cloud API Claude/GPT"]
    MR -->|"OOM/超时（降级）"| OL1

    style MR fill:#1a365d,stroke:#2b6cb0,color:#fff
```

```go
// internal/ai/router/model_router.go

type ModelRouter struct {
    endpoints []AIEndpoint
    rules     []RoutingRule
    fallback  string  // fallback endpoint ID
}

type AIEndpoint struct {
    ID       string `json:"id"`        // "local-primary" | "local-gpu" | "cloud-anthropic"
    Type     string `json:"type"`      // "ollama" | "openai_compat" | "anthropic" | "deepseek"
    BaseURL  string `json:"base_url"`
    Models   []string `json:"models"`  // 该端点可用的模型
    Priority int    `json:"priority"`  // 路由优先级
    Status   string `json:"status"`    // "healthy" | "degraded" | "offline"
}

type RoutingRule struct {
    TaskType    string `json:"task_type"`     // intent_classify | code_gen | code_review | embedding | reasoning
    EndpointID  string `json:"endpoint_id"`   // 目标端点
    ModelName   string `json:"model_name"`    // 指定模型
    MaxLatencyMs int   `json:"max_latency_ms"` // 超时阈值
}

func (r *ModelRouter) Route(ctx context.Context, taskType, prompt string) (string, error) {
    // 1. 查找匹配的路由规则
    rule := r.findRule(taskType)
    endpoint := r.endpoints[rule.EndpointID]
    
    // 2. 健康检查
    if endpoint.Status != "healthy" {
        endpoint = r.findFallback(taskType)
    }
    
    // 3. 调用对应端点
    result, err := r.callEndpoint(ctx, endpoint, rule.ModelName, prompt)
    if err != nil {
        // OOM/超时 → 自动 fallback
        slog.Warn("model router fallback", "endpoint", endpoint.ID, "err", err)
        return r.callEndpoint(ctx, r.endpoints[r.fallback], rule.ModelName, prompt)
    }
    return result, nil
}
```

**v1.0 最小方案**：
- 支持配置多个 AI endpoint（多 Ollama 实例 + OpenAI 兼容 API）
- 按 task type 路由（intent classification → 小模型、code generation → 大模型）
- 主模型超时/OOM → 自动 fallback 到备选 endpoint

**v2.0 增强**：Workspace 级模型配置、E-14 设备画像驱动自动路由、云端 API 费用预算控制。

---

## 3 意图理解引擎 (MVP)

### 3.0 Intent Input Adapter（v7 新增）

当前 Intent Engine 输入是纯文本 HTTP POST。v7 扩展为多模态消息输入接口。

```go
// internal/ai/intent/adapter.go

type IntentInput struct {
    Type     InputType         `json:"type"`      // text | audio_transcription | image_description | video_analysis | structured_command
    Content  string            `json:"content"`   // 文本内容或转写/分析结果
    Source   string            `json:"source"`    // web_ui | open_webui | home_assistant | voice_device
    Metadata InputMetadata     `json:"metadata"`
}

type InputType string
const (
    InputText             InputType = "text"
    InputAudioTranscript  InputType = "audio_transcription"
    InputImageDescription InputType = "image_description"
    InputVideoAnalysis    InputType = "video_analysis"
    InputStructuredCmd    InputType = "structured_command"
)

type InputMetadata struct {
    OriginalFormat string  `json:"original_format,omitempty"` // text | wav | jpeg | mp4
    Confidence     float64 `json:"confidence,omitempty"`      // 转写/识别置信度
    Context        any     `json:"context,omitempty"`         // 来源上下文
    MediaRef       string  `json:"media_ref,omitempty"`       // 关联 Media Asset ID
}
```

v1.0 只实现 `text` 和 `structured_command` 处理器。语音输入从 v1.0 即可通过浏览器 Web Speech API 零成本获得（转写为文本后通过 `audio_transcription` 类型进入）。

三层语音演进：

| 阶段 | 方案 | 依赖 |
|------|------|------|
| v1.0 | 浏览器 Web Speech API | 零后端依赖 |
| v1.5 | 内置 faster-whisper 容器 | GPU 可选 |
| v2.0+ | HA 语音管道 / 手机 App | 各自平台 |

核心原则：**Intent Input Adapter 是唯一集成点**。增加新输入来源只需对接 adapter，不改上层代码。

### 3.0.1 意图路由策略（v7 新增）

BitEngine 是"应用生成优先"的意图系统，与 Home Assistant 的"设备控制优先"形成差异化。意图分类模块增加"意图深度判断"——同样的输入，优先尝试解析为更高价值的意图：

```
优先级：应用生成 > 工作流编排 > 数据查询 > 设备控制
```

### 3.1 三级模型分工

```mermaid
graph TD
    INPUT["用户输入"] --> L1["Level 1: Gemma3-1B<br/>快速分类 ~100ms"]
    L1 -->|"confidence ≥ 0.8"| RESULT["IntentResult"]
    L1 -->|"confidence < 0.8"| L2["Level 2: Qwen3-4B<br/>深度理解 ~2s"]
    L2 -->|"confidence ≥ 0.7"| RESULT
    L2 -->|"confidence < 0.7"| L3["Level 3: 云端大模型<br/>复杂意图 ~3s"]
    L3 --> RESULT
```

### 3.2 意图分类 Schema

```go
// internal/ai/intent/schema.go

type IntentResult struct {
    Intent           string          `json:"intent"`            // create_app | modify_app | query_data | delete_app | system_cmd | control_device
    ResponseStrategy string          `json:"response_strategy"` // full_app | instant_ui | direct_answer
    AppName          string          `json:"app_name,omitempty"`
    Requirements     AppRequirements `json:"requirements,omitempty"`
    Confidence       float64         `json:"confidence"`
    TargetAppID      string          `json:"target_app_id,omitempty"` // modify_app 时指定
    TargetDeviceID   string          `json:"target_device_id,omitempty"` // control_device (v1.0)
}

type AppRequirements struct {
    DataSources []DataSourceReq `json:"data_sources"`
    Processing  []ProcessingReq `json:"processing"`
    Display     []DisplayReq    `json:"display"`
    Automation  []AutomationReq `json:"automation"`
}

// JSON Schema 定义 (用于 Structured Output 约束)
var IntentSchema = map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{
        "intent": map[string]interface{}{
            "type": "string",
            "enum": []string{"create_app", "modify_app", "query_data", "delete_app", "system_cmd", "control_device"},
        },
        "response_strategy": map[string]interface{}{
            "type": "string",
            "enum": []string{"full_app", "instant_ui", "direct_answer"},
        },
        "confidence": map[string]interface{}{
            "type": "number", "minimum": 0, "maximum": 1,
        },
    },
    "required": []string{"intent", "response_strategy", "confidence"},
}
```

### 3.3 意图引擎实现

```go
// internal/ai/intent/engine.go

type IntentEngine struct {
    ollama     *OllamaManager
    cloud      *CloudClient
    structured *StructuredOutput
}

func (e *IntentEngine) Route(ctx context.Context, input string) (*IntentResult, error) {
    // Level 1: Gemma3-1B 快速分类
    l1Prompt := fmt.Sprintf(`Classify the user intent. Respond in JSON.
User: %s`, input)
    
    l1Result, err := e.structured.Generate(ctx, "gemma3:1b", l1Prompt, IntentSchema)
    if err == nil {
        var intent IntentResult
        json.Unmarshal(l1Result, &intent)
        if intent.Confidence >= 0.8 {
            return &intent, nil
        }
    }
    
    // Level 2: Qwen3-4B 深度理解
    l2Prompt := buildDetailedIntentPrompt(input)
    l2Result, err := e.structured.Generate(ctx, "qwen3:4b", l2Prompt, IntentSchema)
    if err == nil {
        var intent IntentResult
        json.Unmarshal(l2Result, &intent)
        if intent.Confidence >= 0.7 {
            return &intent, nil
        }
    }
    
    // Level 3: 云端大模型 (fallback)
    l3Result, err := e.cloud.GenerateStructured(ctx, l2Prompt, IntentSchema)
    var intent IntentResult
    json.Unmarshal(l3Result, &intent)
    return &intent, err
}
```

---

## 4 Structured Output 统一约束层 (MVP)

```go
// internal/ai/structured/enforcer.go

type StructuredOutput struct {
    ollama *OllamaManager
    cloud  *CloudClient
}

// 统一接口: 无论本地还是云端，都保证 JSON Schema 合规
func (s *StructuredOutput) Generate(ctx context.Context, model, prompt string, schema map[string]interface{}) ([]byte, error) {
    if isLocalModel(model) {
        return s.generateLocal(ctx, model, prompt, schema)
    }
    return s.generateCloud(ctx, model, prompt, schema)
}

func (s *StructuredOutput) generateLocal(ctx context.Context, model, prompt string, schema map[string]interface{}) ([]byte, error) {
    // Ollama: format=json + grammar-based 约束解码
    result, err := s.ollama.Generate(ctx, model, prompt, GenerateOpts{
        Format: "json",
        Schema: schema, // Ollama 使用 JSON Schema 约束
    })
    if err != nil {
        return nil, err
    }
    
    // 二次验证: 用 jsonschema.Compiler 校验
    return s.validate([]byte(result), schema)
}

func (s *StructuredOutput) generateCloud(ctx context.Context, model, prompt string, schema map[string]interface{}) ([]byte, error) {
    // Anthropic: native json_schema response_format
    // OpenAI: native json_schema
    // DeepSeek: prompt engineering + post-validation
    result, err := s.cloud.Generate(ctx, prompt, CloudOpts{
        ResponseFormat: &ResponseFormat{Type: "json_schema", Schema: schema},
    })
    if err != nil {
        return nil, err
    }
    return s.validate(result, schema)
}

func (s *StructuredOutput) validate(data []byte, schema map[string]interface{}) ([]byte, error) {
    compiler := jsonschema.NewCompiler()
    // 编译 schema → 验证 data → 不合规则返回错误
    return data, nil
}
```

---

## 5 代码生成器 (MVP)

```go
// internal/ai/codegen/generator.go

type CodeGenerator struct {
    cloud      *CloudClient
    structured *StructuredOutput
}

type GeneratedCode struct {
    Code               string        `json:"code"`
    Files              map[string]string `json:"files"` // filename → content
    Dockerfile         string        `json:"dockerfile"`
    Requirements       string        `json:"requirements"` // pip requirements
    EstimatedResources ResourceQuota `json:"estimated_resources"`
}

func (g *CodeGenerator) Generate(ctx context.Context, intent IntentResult) (*GeneratedCode, error) {
    systemPrompt := g.buildSystemPrompt()
    userPrompt := g.buildUserPrompt(intent)
    
    result, err := g.cloud.Generate(ctx, systemPrompt+"\n\n"+userPrompt, CloudOpts{
        Model:     g.selectModel(intent), // 根据复杂度选模型
        MaxTokens: 8000,
    })
    if err != nil {
        return nil, fmt.Errorf("ai/codegen: generation failed: %w", err)
    }
    
    code := g.parseCodeResponse(result)
    code = g.injectRuntimeAdapters(code) // 注入 DATABASE_URL, PORT, /health
    return code, nil
}

func (g *CodeGenerator) buildSystemPrompt() string {
    return `You are an expert web application developer. Generate a complete Python web application.

CONSTRAINTS:
- Python 3.12 + Flask framework
- Use DATABASE_URL environment variable for database connection
- Listen on port 3000 (from PORT env var)
- Must include /health endpoint returning 200 OK
- No hardcoded secrets - use environment variables
- Use parameterized queries - no string concatenation for SQL
- No eval(), exec(), or __import__()
- File access only under /data directory
- Include requirements.txt with all dependencies
- Include a single app.py entry point

OUTPUT FORMAT: Return a JSON object with keys: files (dict of filename→content), requirements (string), dockerfile (string or null for default).`
}

func (g *CodeGenerator) selectModel(intent IntentResult) string {
    // 默认使用 primary (Claude Sonnet)
    // 中文场景或用户偏好 → DeepSeek
    // 用户显式选择 → 使用指定模型
    return "anthropic/claude-sonnet-4-5-20250929"
}

func (g *CodeGenerator) injectRuntimeAdapters(code *GeneratedCode) *GeneratedCode {
    // 确保 app.py 中:
    // 1. 从 os.environ 读取 DATABASE_URL
    // 2. 监听 int(os.environ.get('PORT', 3000))
    // 3. 存在 @app.route('/health') 返回 200
    return code
}
```

### 5.1 迭代代码生成 (v1.0)

```go
// internal/ai/codegen/iteration.go

func (g *CodeGenerator) GenerateUpdate(ctx context.Context, appID, prompt, currentCode string) (*GeneratedCode, error) {
    systemPrompt := `You are modifying an existing web application. 
The user wants to add or change functionality.

RULES:
- Only modify what's necessary - preserve existing functionality
- Maintain the same tech stack and architecture
- Keep all existing security measures
- Return the COMPLETE updated files (not just diffs)

CURRENT CODE:
` + currentCode
    
    userPrompt := fmt.Sprintf("Modification request: %s", prompt)
    
    result, err := g.cloud.Generate(ctx, systemPrompt+"\n\n"+userPrompt, CloudOpts{
        Model:     "anthropic/claude-sonnet-4-5-20250929",
        MaxTokens: 12000, // 迭代需要更多 token (含原代码)
    })
    
    return g.parseCodeResponse(result), err
}
```

---

## 6 代码审查器 (MVP)

```go
// internal/ai/review/reviewer.go

type CodeReviewer struct {
    ollama     *OllamaManager
    structured *StructuredOutput
}

type ReviewReport struct {
    Passed        bool          `json:"passed"`
    SecurityScore int           `json:"security_score"` // 0-100
    Issues        []ReviewIssue `json:"issues"`
    Suggestions   []string      `json:"suggestions"`
}

type ReviewIssue struct {
    Severity    string `json:"severity"`    // critical | warning | info
    Category    string `json:"category"`    // 见下方清单
    File        string `json:"file"`
    Line        int    `json:"line,omitempty"`
    Description string `json:"description"`
}

// 审查清单
var ReviewChecklist = []string{
    "hardcoded_secret",   // critical: API 密钥、密码硬编码
    "sql_injection",      // critical: SQL 拼接
    "xss",               // critical: 未转义 HTML 输出
    "eval_usage",         // critical: eval/exec/__import__
    "dangerous_import",   // warning: os.system, subprocess
    "network_access",     // warning: 未声明的外部请求
    "file_access",        // warning: /data 目录外的文件操作
    "no_health_check",    // info: 缺少 /health 端点
    "no_error_handling",  // info: 缺少异常处理
}

func (r *CodeReviewer) Review(ctx context.Context, code string) (*ReviewReport, error) {
    prompt := fmt.Sprintf(`Review the following Python web application code for security issues.
Check for: %s

Code:
%s`, strings.Join(ReviewChecklist, ", "), code)
    
    // 使用本地 Phi-4-mini (代码永远不上传到云端)
    result, err := r.structured.Generate(ctx, "phi4-mini", prompt, ReviewReportSchema)
    if err != nil {
        return nil, fmt.Errorf("ai/review: review failed: %w", err)
    }
    
    var report ReviewReport
    json.Unmarshal(result, &report)
    
    // critical issue → passed = false → 阻止部署
    for _, issue := range report.Issues {
        if issue.Severity == "critical" {
            report.Passed = false
            break
        }
    }
    
    return &report, nil
}
```

---

## 7 云端 API 客户端 (MVP)

```go
// internal/ai/cloud/client.go

type CloudClient struct {
    vault     *VaultService
    providers map[string]*ProviderConfig
}

type ProviderConfig struct {
    Name      string `json:"name"`      // anthropic | deepseek | openai
    Model     string `json:"model"`
    BaseURL   string `json:"base_url"`
    KeyVault  string `json:"key_vault"` // vault 中的 key 名称
}

func (c *CloudClient) Generate(ctx context.Context, prompt string, opts CloudOpts) ([]byte, error) {
    provider := c.selectProvider(opts.Model)
    
    // 从 vault 获取 API key (使用后零化)
    apiKey, _ := c.vault.Retrieve(ctx, provider.KeyVault)
    defer Zeroize([]byte(apiKey))
    
    switch provider.Name {
    case "anthropic":
        return c.callAnthropic(ctx, apiKey, prompt, opts)
    case "deepseek":
        return c.callDeepSeek(ctx, apiKey, prompt, opts)
    case "openai":
        return c.callOpenAI(ctx, apiKey, prompt, opts)
    }
    return nil, fmt.Errorf("ai/cloud: unknown provider %s", provider.Name)
}

func (c *CloudClient) callAnthropic(ctx context.Context, apiKey, prompt string, opts CloudOpts) ([]byte, error) {
    // POST https://api.anthropic.com/v1/messages
    // Headers: x-api-key, anthropic-version
    // Body: model, max_tokens, messages, response_format(json_schema)
}

func (c *CloudClient) callDeepSeek(ctx context.Context, apiKey, prompt string, opts CloudOpts) ([]byte, error) {
    // POST https://api.deepseek.com/v1/chat/completions
    // OpenAI 兼容格式 + JSON Schema response_format
}

func (c *CloudClient) callOpenAI(ctx context.Context, apiKey, prompt string, opts CloudOpts) ([]byte, error) {
    // POST https://api.openai.com/v1/chat/completions
    // response_format: {type: "json_schema", json_schema: {...}}
}
```

---

## 8 已迁出章节说明

以下原 DD-06 章节已迁出到其他 DD：

| 原章节 | 内容 | 新归属 | 原因 |
|--------|------|--------|------|
| §8 AG-UI 事件流生成 | AG-UI 事件类型、流式推送、INTERRUPT→A2H 桥接 | **DD-08** 前端架构 | AG-UI 是前端运行时协议 |
| §9 A2UI 声明式 UI 生成 | A2UI 组件类型、安全约束 | **DD-08** 前端架构 | A2UI 是前端声明式 UI |
| §10 A2A 多 Agent 协作 | 内部编排、任务分解、结果聚合 | **DD-03** A2H + A2A | A2A 是 Agent 间通信协议，通信层对齐 Google A2A |
| §11 Governance Agent | AI 监控 AI 行为合规 | **DD-03** A2H + A2A | 治理是编排层管控 |
| §12 WebLLM 浏览器端推理 | WebGPU + Gemma3-1B in browser | **DD-08** 前端架构 | 浏览器端能力属前端 |

---

# 应用中心（原 DD-03，v7 合并入 DD-02）

> 模块路径：`internal/appcenter/` | 以下 §9-14 原为独立的 DD-03 应用中心文档，v7 合并入 AI 引擎。

---

## 9 AI 生成流水线 (MVP)（原 DD-03 §2）

```mermaid
sequenceDiagram
    participant U as 用户
    participant FE as 前端 (SSE)
    participant GEN as GenerationPipeline
    participant AI as AI 引擎
    participant RT as Runtime (DD-01)
    participant A2H as A2H 网关 (DD-03)

    U->>FE: "帮我做银行监控系统"
    FE->>GEN: Generate(prompt, events)
    GEN->>AI: RouteIntent(prompt)
    GEN-->>FE: SSE {phase:intent, status:done}
    GEN->>AI: GenerateApp(intent)
    GEN-->>FE: SSE {phase:codegen, status:streaming}
    GEN->>AI: ReviewCode(code)
    GEN-->>FE: SSE {phase:review, score:95}
    GEN->>A2H: Authorize("部署 bank-monitor?")
    A2H->>U: 确认对话框
    U->>A2H: 批准
    GEN-->>FE: SSE {phase:authorize, status:approved}
    GEN->>RT: Create(appSpec)
    GEN-->>FE: SSE {phase:deploy, status:done, url:"bank.bit.local"}
```

```go
// internal/appcenter/generator/pipeline.go

type GenerationPipeline struct {
    ai      AIEngine
    runtime AppRuntime
    a2h     A2HGateway
    audit   AuditChain
}

func (p *GenerationPipeline) Generate(ctx context.Context, prompt string, events chan<- SSEEvent) (*AppInstance, error) {
    // Phase 1: 意图理解
    events <- SSEEvent{Phase: "intent", Status: "running"}
    intent, err := p.ai.RouteIntent(ctx, prompt)
    if err != nil { return nil, fmt.Errorf("appcenter: intent failed: %w", err) }
    events <- SSEEvent{Phase: "intent", Status: "done", Data: intent}
    
    // 响应策略判断
    switch intent.ResponseStrategy {
    case "instant_ui":
        return nil, p.handleInstantUI(ctx, intent, events) // 轻量 AG-UI/A2UI 路径
    case "direct_answer":
        return nil, p.handleDirectAnswer(ctx, intent, events)
    }
    
    // Phase 2: 代码生成（云端大模型，通过 Model Router 路由）
    events <- SSEEvent{Phase: "codegen", Status: "running"}
    code, err := p.ai.GenerateApp(ctx, *intent)
    if err != nil { return nil, fmt.Errorf("appcenter: codegen failed: %w", err) }
    events <- SSEEvent{Phase: "codegen", Status: "done"}
    
    // Phase 3: 代码审查（本地 Phi-4-mini）
    events <- SSEEvent{Phase: "review", Status: "running"}
    report, err := p.ai.ReviewCode(ctx, code.Code)
    if err != nil { return nil, fmt.Errorf("appcenter: review failed: %w", err) }
    if !report.Passed {
        events <- SSEEvent{Phase: "review", Status: "blocked", Data: report}
        return nil, fmt.Errorf("appcenter: code review critical issues: %v", report.Issues)
    }
    events <- SSEEvent{Phase: "review", Status: "done", Data: map[string]any{"security_score": report.SecurityScore}}
    
    // Phase 4: A2H 授权（DD-03 A2H 网关）
    events <- SSEEvent{Phase: "authorize", Status: "waiting"}
    approved, _ := p.a2h.Authorize(ctx, fmt.Sprintf("部署应用 %s?", intent.AppName), map[string]any{
        "app_name": intent.AppName, "security_score": report.SecurityScore, "resources": code.EstimatedResources,
    })
    if !approved {
        return nil, fmt.Errorf("appcenter: user rejected deployment")
    }
    events <- SSEEvent{Phase: "authorize", Status: "approved"}
    
    // Phase 5: 部署（DD-01 Runtime）
    events <- SSEEvent{Phase: "deploy", Status: "running"}
    app, err := p.runtime.Create(ctx, AppSpec{
        Name: intent.AppName, Slug: slugify(intent.AppName), Code: []byte(code.Code), Resources: code.EstimatedResources,
    })
    if err != nil { return nil, fmt.Errorf("appcenter: deploy failed: %w", err) }
    events <- SSEEvent{Phase: "deploy", Status: "done", Data: map[string]any{"url": app.Domain, "app_id": app.ID}}
    
    p.audit.Append(ctx, AuditEntry{EventType: "app.created", TargetID: app.ID, Detail: prompt})
    return app, nil
}
```

**SSE 进度事件**：

| Phase | Status | 前端展示 |
|-------|--------|---------|
| intent | running/done | "正在理解需求..." → "✓ 分析完成" |
| codegen | running/streaming/done | "正在生成代码..." → 流式代码展示 |
| review | running/done/blocked | "正在安全审查..." → "安全评分: 95/100" |
| authorize | waiting/approved/rejected | 弹出确认对话框 |
| deploy | running/done | "正在部署..." → "✅ 应用已就绪" |

---

## 10 内置模板 (MVP)（原 DD-03 §3）

```go
// internal/appcenter/templates/registry.go

var BuiltinTemplates = []Template{
    {Name: "项目看板", Slug: "project-board", TechStack: "Flask + SQLite + HTMX",
     Description: "看板式项目管理，拖拽卡片、多状态列"},
    {Name: "记账工具", Slug: "ledger", TechStack: "Flask + SQLite + Chart.js",
     Description: "收支记录、分类统计、月度趋势图表"},
    {Name: "CRM 客户管理", Slug: "crm", TechStack: "Flask + PostgreSQL + DataTables",
     Description: "客户信息管理、跟进记录、销售漏斗"},
    {Name: "表单构建器", Slug: "form-builder", TechStack: "Flask + SQLite + Sortable.js",
     Description: "拖拽式表单设计、数据收集、导出"},
    {Name: "数据看板", Slug: "dashboard", TechStack: "Flask + PostgreSQL + ECharts",
     Description: "多数据源看板、自定义图表、实时刷新"},
}

func (s *TemplateService) Deploy(ctx context.Context, slug string) (*AppInstance, error) {
    tmpl := findTemplate(slug)
    // 跳过 AI 生成/审查 (模板已预审) → 直接 Runtime.Create
}
```

---

## 11 应用迭代更新 (v1.0)（原 DD-03 §4）

```go
// internal/appcenter/iteration/service.go

type IterationService struct {
    ai       AIEngine
    runtime  AppRuntime
    deployer *BlueGreenDeployer  // DD-01 §19 蓝绿部署
    audit    AuditChain
}

func (s *IterationService) Iterate(ctx context.Context, appID string, prompt string, events chan<- SSEEvent) error {
    // 1. 获取应用当前代码
    app, _ := s.runtime.GetStatus(ctx, appID)
    currentCode, _ := s.extractCode(ctx, app)
    
    // 2. AI 生成更新代码（通过 Model Router 路由到云端）
    events <- SSEEvent{Phase: "codegen", Status: "running"}
    updated, _ := s.ai.GenerateAppUpdate(ctx, appID, prompt, currentCode)
    events <- SSEEvent{Phase: "codegen", Status: "done"}
    
    // 3. 代码审查
    events <- SSEEvent{Phase: "review", Status: "running"}
    report, _ := s.ai.ReviewCode(ctx, updated.Code)
    if !report.Passed {
        return fmt.Errorf("appcenter: iteration review failed")
    }
    events <- SSEEvent{Phase: "review", Status: "done", Data: report}
    
    // 4. 蓝绿部署
    events <- SSEEvent{Phase: "deploy", Status: "running"}
    err := s.deployer.Deploy(ctx, appID, AppSpec{Code: []byte(updated.Code), Resources: app.Resources})
    if err != nil {
        events <- SSEEvent{Phase: "deploy", Status: "failed"}
        return err
    }
    events <- SSEEvent{Phase: "deploy", Status: "done"}
    
    s.audit.Append(ctx, AuditEntry{EventType: "app.iterated", TargetID: appID, Detail: prompt})
    return nil
}
```

---

## 12 自主应用包 (v1.0)（原 DD-03 §5）

借鉴 OpenFang Hands 的自主运行策略（SOP）。

```go
// internal/appcenter/autopacks/pack.go

type AutonomousPack struct {
    Name        string            `toml:"name"`
    Description string            `toml:"description"`
    Category    string            `toml:"category"`
    Schedule    map[string]string `toml:"schedule"`
    SOP         SOPConfig         `toml:"sop"`
    Alerts      AlertConfig       `toml:"alerts"`
    Modules     []string          `toml:"modules"`
    Template    string            `toml:"template"`
}

var BuiltinPacks = []AutonomousPack{
    {Name: "竞品价格监控", Category: "business-intelligence",
     Schedule: map[string]string{"collect": "0 8 * * *", "analyze": "0 9 * * *", "report": "0 9 * * 1"}},
    {Name: "财务汇总助手", Category: "finance"},
    {Name: "社媒管理", Category: "marketing"},
    {Name: "库存管理", Category: "operations"},
    {Name: "客户满意度追踪", Category: "customer-success"},
}

func (s *PackService) Deploy(ctx context.Context, packSlug string) (*AppInstance, error) {
    pack := findPack(packSlug)
    app, _ := s.appCenter.DeployTemplate(ctx, pack.Template)
    for phase, cron := range pack.Schedule {
        s.runtime.ScheduleTask(ctx, CronTask{AppID: app.ID, Name: fmt.Sprintf("%s_%s", pack.Name, phase), Schedule: cron,
            Action: "http_call", Config: TaskConfig{URL: fmt.Sprintf("http://app-%s:3000/sop/%s", app.Slug, phase)}})
    }
    return app, nil
}
```

---

## 13 生态导入 (v1.0)（原 DD-03 §6）

### 13.1 OpenClaw 适配器

```go
// internal/appcenter/ecosystem/claw.go

func (a *ClawHubAdapter) Import(ctx context.Context, skillName string) (*AppInstance, error) {
    // 1. 从 ClawHub API 下载 Skill
    // 2. 安全扫描 (auto_reject / manual_review / auto_approve)
    // 3. 转换: SKILL.md → MODULE.md, 脚本 → Docker 容器
    // 4. Ed25519 签名 (标记 "adapted-import")
    // 5. 部署
}
```

### 13.2 OpenFang 适配器

```go
// internal/appcenter/ecosystem/fang.go

func (a *FangHubAdapter) Import(ctx context.Context, handName string) (*AppInstance, error) {
    // 1. 从 FangHub 下载 Hand
    // 2. 安全扫描
    // 3. 转换: HAND.toml → manifest + MODULE.md，保留 SOP 策略
    // 4. WASM 兼容部分直接复用沙箱
    // 5. 签名 + 部署
}
```

### 13.3 安全扫描规则

```go
var autoRejectPatterns = []string{
    `curl.*\|.*>>\.*\.ssh`,        // SSH 密钥操纵
    `nc\s+-l|ncat|socat`,          // 反向 shell
    `base64.*decode.*\|.*sh`,      // 编码命令执行
    `/etc/passwd|/etc/shadow`,     // 系统文件
    `eval\(.*fetch\(`,             // 远程代码加载
}

var manualReviewPatterns = []string{
    `requests\.post|fetch\(`,      // 外部网络请求
    `os\.system|subprocess`,       // 系统命令
    `open\(.*,'w'\)`,              // 文件写入
}
```

### 13.4 批量导入 CLI

```bash
bitengine import clawhub --category "productivity" --min-stars 10
bitengine import fanghub --category "research" --include-sop
bitengine import clawhub --skill "notion-integration"
```

```go
// internal/ai/agui/stream.go

type AGUIEventType string

const (
    AGUITextStream    AGUIEventType = "TEXT_MESSAGE_STREAM"
    AGUIToolCall      AGUIEventType = "TOOL_CALL"
    AGUIStateDelta    AGUIEventType = "STATE_DELTA"
    AGUIInterrupt     AGUIEventType = "INTERRUPT"
    AGUIGenerativeUI  AGUIEventType = "GENERATIVE_UI"
)

type AGUIEvent struct {
    Type      AGUIEventType  `json:"type"`
    Data      interface{}    `json:"data"`
    Timestamp time.Time      `json:"timestamp"`
}

type AGUIStreamService struct {
    wsHub   *WebSocketHub
    ai      *IntentEngine
    mcp     MCPClient
    datahub DataHub
}

func (s *AGUIStreamService) StreamResponse(ctx context.Context, sessionID string, intent *IntentResult, events chan<- AGUIEvent) error {
    switch intent.ResponseStrategy {
    case "instant_ui":
        return s.handleInstantUI(ctx, sessionID, intent, events)
    case "direct_answer":
        return s.handleDirectAnswer(ctx, sessionID, intent, events)
    }
    return nil
}

func (s *AGUIStreamService) handleInstantUI(ctx context.Context, sessionID string, intent *IntentResult, events chan<- AGUIEvent) error {
    // 1. 通过 MCP 获取数据
    events <- AGUIEvent{Type: AGUIToolCall, Data: ToolCallEvent{Name: "apps/query", Status: "running"}}
    data, _ := s.mcp.CallExternalTool(ctx, "local", "apps/query", intent.Requirements.DataSources[0].ToArgs())
    events <- AGUIEvent{Type: AGUIToolCall, Data: ToolCallEvent{Name: "apps/query", Status: "done"}}
    
    // 2. 生成 A2UI 声明式 UI
    a2uiSpec, _ := s.generateA2UI(ctx, intent, data)
    events <- AGUIEvent{Type: AGUIGenerativeUI, Data: a2uiSpec}
    
    return nil
}
```

---

## 13.5 应用中心数据库 Schema（原 DD-03 §8）

```sql
CREATE TABLE app_generations (
    id           TEXT PRIMARY KEY,
    app_id       TEXT REFERENCES apps(id),
    prompt       TEXT NOT NULL,
    intent       JSONB NOT NULL,
    code_hash    TEXT,
    review_score INTEGER,
    cloud_model  TEXT,
    tokens_used  INTEGER,
    duration_ms  INTEGER,
    status       TEXT NOT NULL,
    error_msg    TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE app_templates (
    slug         TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    description  TEXT,
    tech_stack   TEXT,
    resources    JSONB DEFAULT '{}',
    builtin      BOOLEAN DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- v1.0
CREATE TABLE autonomous_packs (
    slug         TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    description  TEXT,
    category     TEXT,
    schedule     JSONB NOT NULL,
    sop          JSONB NOT NULL,
    modules      JSONB DEFAULT '[]',
    builtin      BOOLEAN DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ecosystem_imports (
    id           TEXT PRIMARY KEY,
    source       TEXT NOT NULL,
    source_name  TEXT NOT NULL,
    app_id       TEXT REFERENCES apps(id),
    scan_result  TEXT NOT NULL,
    scan_details JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## 14 API 端点

**AI 引擎 API**：

| 方法 | 端点 | 说明 | 阶段 |
|------|------|------|------|
| POST | `/api/v1/ai/chat` | AI 对话 (SSE 流式) | MVP |
| POST | `/api/v1/ai/intent` | 意图分析 (调试用) | MVP |
| GET | `/api/v1/ai/models` | 本地模型状态 | MVP |
| POST | `/api/v1/ai/models/:name/load` | 加载指定模型到内存（v7 新增） | MVP |
| POST | `/api/v1/ai/models/:name/unload` | 从内存卸载指定模型（v7 新增） | MVP |
| GET | `/api/v1/ai/router/config` | Model Router 配置（v7 新增） | v1.0 |
| PUT | `/api/v1/ai/router/config` | 更新 Model Router 路由规则（v7 新增） | v1.0 |
| PUT | `/api/v1/ai/config` | AI 配置 (云端 provider 等) | MVP |
| POST | `/api/v1/ai/review` | 手动代码审查 (调试用) | MVP |
| POST | `/api/v1/ai/embed` | 文本嵌入 (调试用) | MVP |

**应用中心 API（原 DD-03）**：

| 方法 | 端点 | 说明 | 阶段 |
|------|------|------|------|
| POST | `/api/v1/apps` | 创建应用（AI 生成或模板） | MVP |
| GET | `/api/v1/apps` | 应用列表 | MVP |
| GET | `/api/v1/apps/:id` | 应用详情 | MVP |
| DELETE | `/api/v1/apps/:id` | 删除应用 | MVP |
| POST | `/api/v1/apps/:id/start` | 启动 | MVP |
| POST | `/api/v1/apps/:id/stop` | 停止 | MVP |
| GET | `/api/v1/apps/:id/logs` | 查看日志 | MVP |
| GET | `/api/v1/apps/templates` | 模板列表 | MVP |
| POST | `/api/v1/apps/templates/:slug/deploy` | 部署模板 | MVP |
| PUT | `/api/v1/apps/:id` | 迭代更新 | v1.0 |
| POST | `/api/v1/apps/:id/rollback` | 回滚 | v1.0 |
| GET | `/api/v1/apps/packs` | 自主应用包列表 | v1.0 |
| POST | `/api/v1/apps/packs/:slug/deploy` | 部署应用包 | v1.0 |
| POST | `/api/v1/apps/import/claw` | 导入 OpenClaw Skill | v1.0 |
| POST | `/api/v1/apps/import/fang` | 导入 OpenFang Hand | v1.0 |

---

## 15 错误码

| 错误码 | 说明 | 阶段 |
|--------|------|------|
| `AI_INTENT_FAILED` | 意图理解失败 (三级全部失败) | MVP |
| `AI_CODEGEN_FAILED` | 代码生成失败 | MVP |
| `AI_CODEGEN_TIMEOUT` | 代码生成超时 | MVP |
| `AI_REVIEW_CRITICAL` | 代码审查发现严重问题 | MVP |
| `AI_CLOUD_AUTH_FAILED` | 云端 API Key 无效 | MVP |
| `AI_CLOUD_RATE_LIMITED` | 云端 API 限流 | MVP |
| `AI_CLOUD_QUOTA_EXCEEDED` | 云端 API 配额用尽 | MVP |
| `AI_MODEL_NOT_LOADED` | 本地模型未加载 | MVP |
| `AI_STRUCTURED_INVALID` | Structured Output 校验失败 | MVP |
| `AI_ROUTER_NO_ENDPOINT` | Model Router 无可用端点（v7 新增） | v1.0 |
| `AI_ROUTER_ALL_FALLBACK_FAILED` | 所有 fallback 端点失败（v7 新增） | v1.0 |
| `AI_INPUT_UNSUPPORTED_TYPE` | 不支持的意图输入类型（v7 新增） | v1.0 |
| `AI_INJECTION_DETECTED` | 提示注入攻击检测 | v1.0 |
| `APP_GEN_INTENT_FAILED` | 应用生成意图失败（原 DD-03） | MVP |
| `APP_GEN_CODE_FAILED` | 应用生成代码失败（原 DD-03） | MVP |
| `APP_GEN_REVIEW_CRITICAL` | 生成代码审查严重问题（原 DD-03） | MVP |
| `APP_GEN_DEPLOY_FAILED` | 应用部署失败（原 DD-03） | MVP |
| `APP_LIMIT_REACHED` | 应用数量上限（原 DD-03） | MVP |
| `APP_UPDATE_ROLLBACK` | 更新失败已回滚（原 DD-03） | v1.0 |
| `APP_IMPORT_SCAN_REJECTED` | 生态导入安全扫描拒绝（原 DD-03） | v1.0 |
| `APP_PACK_DEPENDENCY_MISSING` | 应用包依赖模块未安装（原 DD-03） | v1.0 |

---

## 16 提示注入扫描器 (v1.0)

十层安全第 9 层实现。防止用户输入和外部数据中的恶意指令影响 Agent 行为。

```go
// internal/ai/security/injection.go

type InjectionScanner struct {
    classifier *OllamaClient // Gemma3-1B 快速分类
    patterns   []*regexp.Regexp
}

type ScanResult struct {
    Safe       bool    `json:"safe"`
    Score      float64 `json:"score"`       // 0.0 (safe) ~ 1.0 (malicious)
    Category   string  `json:"category"`    // none | role_hijack | data_exfil | instruction_override
    Matched    string  `json:"matched"`     // 匹配到的模式 (如有)
}

// 两阶段检测：正则快筛 + LLM 语义判断
func (s *InjectionScanner) Scan(ctx context.Context, input string) (*ScanResult, error) {
    // 阶段 1: 正则模式快筛 (微秒级)
    for _, pat := range s.patterns {
        if pat.MatchString(input) {
            return &ScanResult{Safe: false, Score: 0.9, Category: "pattern_match", Matched: pat.String()}, nil
        }
    }
    
    // 阶段 2: Gemma3-1B 语义分类 (毫秒级)
    prompt := fmt.Sprintf(`Classify if this user input contains prompt injection attempts.
Input: %s
Respond with JSON: {"safe": bool, "score": float, "category": string}`, input)
    
    result, _ := s.classifier.Generate(ctx, "gemma3:1b", prompt, injectionSchema)
    
    var sr ScanResult
    json.Unmarshal(result, &sr)
    return &sr, nil
}

// 已知注入模式
var injectionPatterns = []string{
    `(?i)ignore\s+(previous|above|all)\s+(instructions?|prompts?)`,
    `(?i)you\s+are\s+now\s+(a|an)\s+`,
    `(?i)system\s*:\s*`,
    `(?i)act\s+as\s+(if\s+)?you\s+(are|were)`,
    `(?i)do\s+not\s+follow\s+(your|the)\s+(rules|instructions)`,
    `(?i)reveal\s+(your|the)\s+(system|original)\s+prompt`,
    `(?i)output\s+(your|the)\s+(instructions|rules|prompt)`,
    `(?i)<\s*\/?system\s*>`,
}
```

### 15.1 集成点

```go
// 在 IntentRouter 入口处自动扫描
func (r *IntentRouter) Route(ctx context.Context, input string) (*IntentResult, error) {
    // 1. 注入扫描 (前置)
    scanResult, _ := r.injectionScanner.Scan(ctx, input)
    if !scanResult.Safe && scanResult.Score > 0.8 {
        r.audit.Append(ctx, AuditEntry{
            EventType: "ai.injection_blocked",
            Detail:    fmt.Sprintf("score=%.2f category=%s", scanResult.Score, scanResult.Category),
        })
        return nil, ErrPromptInjectionDetected
    }
    
    // 2. 正常路由
    return r.classifyIntent(ctx, input)
}
```

### 15.2 数据隔离原则

- Agent 编排层与应用运行层严格分离：应用内数据不流入 Agent prompt
- 云端 API 调用只传脱敏后的需求描述，不传业务数据
- RAG 检索结果经脱敏过滤后才注入 LLM prompt
- Structured Output 约束消除格式解析异常路径 (常见注入攻击向量)

---

## 17 测试策略

| 类型 | 覆盖 | 工具 |
|------|------|------|
| 单元测试 | Intent Schema 验证、Structured Output 校验、代码审查规则匹配、Model Router 路由逻辑（v7） | `testing` + `testify` |
| 集成测试 | Ollama 模型调用、三级意图路由完整链路 | `testcontainers-go` (Ollama) |
| 集成测试 | Model Router 多端点 fallback：主端点超时→备选端点（v7 新增） | Mock AI endpoints |
| 集成测试 | Intent Input Adapter：text / audio_transcription / image_description 各类型（v7 新增） | Mock adapter |
| 集成测试 | AI 生成流水线完整链路：意图→代码生成→审查→A2H→部署（原 DD-03） | 全栈 Mock |
| 代码审查测试 | 已知漏洞代码片段 → 确认审查器能检出 | 漏洞样本库 |
| Prompt 回归测试 | 固定输入 → 验证意图分类稳定性 (>95% 一致) | 自定义 |
| 安全测试 | 提示注入扫描器 + 生态导入安全扫描（原 DD-03） | 注入样本库 + 恶意代码样本 |
