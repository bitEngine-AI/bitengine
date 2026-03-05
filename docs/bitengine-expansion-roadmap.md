# BitEngine 扩展路线：从 Warmode MVP 到完整 v7

> 核心原则：**每一波扩展由用户信号驱动，不由架构完整性驱动。**
> 
> MVP 上线后，根据 GitHub issues、社区反馈、star 增长曲线决定下一波做什么。
> 以下是按"用户价值 × 实现难度"排序的推荐路径，不是固定计划。

---

## 总览：5 波扩展，从 MVP 到 v7

```
Week 1-2    ██████████████████████████████  MVP（8 tasks）→ 上 GitHub
Week 3-4    ████████████████                Wave 1: 让用户留下来
Week 5-6    ██████████████████████          Wave 2: 扩大用户群
Week 7-8    ████████████████████            Wave 3: 差异化（IoT）
Week 9-10   ██████████████                  Wave 4: 协议对齐
Week 11+    ████████████████████████████    Wave 5: 企业版
```

---

## MVP 代码结构 → 扩展映射

```
bitengine/
├── internal/
│   ├── config/          ← 不变，只加新配置项
│   ├── auth/            ← Wave 1 加 RBAC，Wave 5 加 SSO
│   ├── setup/           ← 不变
│   ├── ai/
│   │   ├── ollama.go    ← 不变
│   │   ├── intent.go    ← Wave 2 加 Input Adapter（多模态）
│   │   ├── codegen.go   ← 不变
│   │   ├── review.go    ← 不变
│   │   ├── router.go    ← Wave 2 新增（Model Router，多端点）
│   │   ├── embed.go     ← Wave 2 新增（BGE-M3 嵌入）
│   │   └── injection.go ← Wave 1 新增（注入扫描）
│   ├── runtime/
│   │   ├── docker.go    ← 不变
│   │   ├── builder.go   ← 不变
│   │   ├── network.go   ← 不变
│   │   ├── wasm.go      ← Wave 3 新增
│   │   └── scheduler.go ← Wave 1 新增（Cron）
│   ├── apps/
│   │   ├── service.go   ← Wave 1 加迭代更新
│   │   ├── generator.go ← 不变
│   │   ├── templates.go ← 不变
│   │   └── importer.go  ← Wave 2 新增（Claw/Fang 导入）
│   ├── monitor/         ← 不变
│   ├── backup/          ← 不变
│   ├── eventbus/
│   │   └── bus.go       ← Wave 3 加 MQTT 5.0 双通道
│   ├── caddy/           ← 不变
│   │
│   │  ══ 以下全部是新增目录 ══
│   │
│   ├── datahub/         ← Wave 2 新增
│   │   ├── filemanager/   （MVP 已有文件管理概念，直接扩展）
│   │   ├── rag/           RAG 引擎
│   │   ├── media/         媒体资产
│   │   ├── lake/          数据湖
│   │   ├── nlquery/       自然语言查询
│   │   ├── pipeline/      数据管道
│   │   └── export/        导出
│   ├── mcp/             ← Wave 2 新增
│   │   ├── server/        MCP Server
│   │   ├── client/        MCP Client
│   │   └── elicitation/   Elicitation
│   ├── a2h/             ← Wave 1 新增（从简单通知开始）
│   │   ├── gateway/       A2H 网关
│   │   └── channels/      多渠道
│   ├── iothub/          ← Wave 3 新增
│   │   ├── aggregator/    Device Aggregator
│   │   ├── providers/     设备提供者
│   │   ├── bridge/        MCP↔MQTT Bridge
│   │   └── rule/          AI 规则引擎
│   ├── a2a/             ← Wave 4 新增
│   │   ├── server/        A2A Server
│   │   ├── orchestrator/  编排器
│   │   └── governance/    Governance Agent
│   ├── ecosystem/       ← Wave 4 新增
│   │   ├── otel/          OTel GenAI
│   │   └── openaicompat/  OpenAI 兼容 API
│   └── enterprise/      ← Wave 5 新增
│       ├── tenant/
│       ├── worker/
│       └── ha/
```

关键点：**MVP 的代码不需要重构**。每个新功能是新增文件/目录，然后在 `api/router.go` 注册新路由。

---

## Wave 1：让用户留下来（Week 3-4）

> **触发信号**：MVP 上线后有人 star 了，有人在用。
> **目标**：让试用的人变成持续使用的人。

### 功能清单

| # | 功能 | 用户价值 | 工作量 | DD 参考 |
|---|------|---------|--------|--------|
| 1.1 | **应用迭代更新** | "把看板改成列表视图" → AI 修改现有应用 | 1 天 | DD-02 §11 |
| 1.2 | A2H 基础通知（Web 弹窗） | 生成完成/失败弹窗，危险操作确认 | 0.5 天 | DD-03 §2 |
| 1.3 | 应用日志查看器 UI | 前端实时看容器日志 | 0.5 天 | DD-08 §4 |
| 1.4 | 更多模板（10→20） | 更多一键部署选择 | 1 天 | DD-02 §10 |
| 1.5 | 基础安全审查强化 | 代码审查结果展示在 UI 上 | 0.5 天 | DD-02 §6 |

### 代码变更

```bash
# 1.1 应用迭代 — 在 apps/ 里加一个方法
internal/apps/service.go
  + func (s *Service) IterateApp(ctx, appID, prompt string, sse SSEWriter) error
  # 读取现有代码 → 和新 prompt 一起发给云端 → 生成更新 → 蓝绿部署

# 1.2 A2H 基础 — 新增目录
internal/a2h/gateway/gateway.go
  + type A2HGateway struct { ... }
  + func (g *A2HGateway) Inform(ctx, message) error    // Web 控制台弹窗
  + func (g *A2HGateway) Authorize(ctx, action) bool    // 确认弹窗
  # 通过现有 eventbus → WebSocket 推送到前端

# 1.3-1.5 都是小改动，不展开
```

### 新增 API

```
PUT  /api/v1/apps/:id         # 迭代更新（SSE）
POST /api/v1/apps/:id/rollback  # 回滚
GET  /api/v1/a2h/pending       # 待确认列表
POST /api/v1/a2h/:id/respond   # 确认/拒绝
```

### 新增迁移

```sql
-- migrations/002_wave1.sql
ALTER TABLE runtime.apps ADD COLUMN prev_source_code TEXT;  -- 回滚用
ALTER TABLE runtime.apps ADD COLUMN version INTEGER DEFAULT 1;
CREATE TABLE platform.a2h_messages (
    id TEXT PRIMARY KEY, intent TEXT, message TEXT,
    status TEXT DEFAULT 'pending', created_at TIMESTAMPTZ DEFAULT NOW()
);
```

---

## Wave 2：扩大用户群（Week 5-6）

> **触发信号**：100+ stars，社区开始问"能不能搜索我的文档""能不能从 Claude Desktop 控制"。
> **目标**：RAG + MCP 让 BitEngine 融入用户现有工作流。

### 功能清单

| # | 功能 | 用户价值 | 工作量 | DD 参考 |
|---|------|---------|--------|--------|
| 2.1 | **RAG 知识库** | 上传 PDF/Word → AI 对话能搜索 | 2 天 | DD-04 §6 |
| 2.2 | **MCP Server** | Claude Desktop/Open WebUI 可控制 BitEngine | 2 天 | DD-07 §2-3 |
| 2.3 | MCP Client | 应用能调外部 MCP 服务 | 1 天 | DD-07 §7 |
| 2.4 | OpenAI 兼容 API | 第三方工具接入 | 0.5 天 | DD-07 §8 |
| 2.5 | 文件管理器 | 浏览/上传/下载本地文件 | 1 天 | DD-04 §3 |
| 2.6 | Model Router | 多 Ollama 实例 + 云端自动路由 | 1 天 | DD-02 §2.3 |

### 代码变更

```bash
# 2.1 RAG — 新增 datahub 目录
internal/datahub/rag/engine.go
  + type RAGEngine struct { chroma ChromaClient; embedder Embedder }
  + func (r *RAGEngine) Ingest(ctx, appID, doc io.Reader) error
  + func (r *RAGEngine) Search(ctx, appID, query string, topK int) ([]Result, error)

# docker-compose 加 ChromaDB
deploy/docker-compose.yml
  + chromadb: { image: chromadb/chroma, ports: ["8000:8000"] }

# 2.2 MCP Server — 新增 mcp 目录
internal/mcp/server/server.go
  + type MCPServer struct { apps AppService; ... }
  + tools: apps/list, apps/create, apps/query, system/status
  # 注册到 POST /mcp 端点（JSON-RPC 2.0）

# 2.6 Model Router — 扩展 ai/
internal/ai/router.go
  + type ModelRouter struct { endpoints []Endpoint; rules []Rule }
  + func (r *ModelRouter) Route(ctx, taskType, prompt) (model, endpoint, error)
```

### 新增 docker-compose overlay

```yaml
# deploy/docker-compose.wave2.yml
services:
  chromadb:
    image: chromadb/chroma:latest
    ports: ["8000:8000"]
    networks: [ef]
```

启动命令变为：
```bash
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.wave2.yml up
```

### 新增 API

```
POST /mcp                              # MCP JSON-RPC 端点
POST /v1/chat/completions              # OpenAI 兼容
POST /api/v1/data/kb/ingest            # 知识库摄取
POST /api/v1/data/kb/search            # 语义搜索
GET  /api/v1/data/files/*              # 文件浏览
```

### 传播动作

发布 MCP 后，立即做一个 GIF：

```
Claude Desktop → 配置 BitEngine MCP Server
→ "帮我在 BitEngine 上创建一个记账应用"
→ Claude 调用 MCP tool → BitEngine 自动生成 → 应用运行
```

发到 r/ClaudeAI：**"Control BitEngine from Claude Desktop via MCP"** —— 这会引爆 MCP 社区。

---

## Wave 3：差异化——IoT（Week 7-8）

> **触发信号**：社区开始问"能不能控制智能家居""能不能接传感器"。
> **目标**：这是 Open WebUI/Dify 做不到的事。IoT 是我们的护城河。

### 功能清单

| # | 功能 | 用户价值 | 工作量 | DD 参考 |
|---|------|---------|--------|--------|
| 3.1 | **Device Aggregator** | 局域网自动发现 MCP 设备 | 2 天 | DD-05 §3 |
| 3.2 | **MQTT 5.0 Broker** | Mosquitto + MCP↔MQTT Bridge | 2 天 | DD-05 §5-6 |
| 3.3 | **AI 规则引擎** | "温度>35°开空调" 自然语言创建规则 | 2 天 | DD-05 §8 |
| 3.4 | Home Assistant 桥接 | 接入 HA 的 2000+ 设备 | 1 天 | DD-05 §4.2 |
| 3.5 | IoT 前端页面 | 设备列表 + 状态 + 控制面板 | 1 天 | DD-08 §9 |

### 代码变更

```bash
# 事件总线升级为双通道
internal/eventbus/bus.go
  + func (b *Bus) PublishMQTT(ctx, topic, payload, opts) error
  + func (b *Bus) SubscribeMQTT(ctx, topicFilter) (<-chan MQTTMessage, error)

# docker-compose 加 Mosquitto
deploy/docker-compose.wave3.yml
  + mosquitto: { image: eclipse-mosquitto:2, ports: ["1883:1883"] }

# IoT 全新目录
internal/iothub/aggregator/aggregator.go    # 设备聚合
internal/iothub/providers/mqtt_direct/      # MQTT 直连设备
internal/iothub/providers/home_assistant/   # HA 桥接
internal/iothub/bridge/mqtt_bridge.go       # MCP↔MQTT Bridge
internal/iothub/rule/engine.go              # AI 规则引擎
```

### 新增 API

```
GET  /api/v1/iot/devices                # 设备列表
POST /api/v1/iot/devices/:id/tools/:t   # 调用设备工具
POST /api/v1/iot/rules                  # 创建规则
GET  /api/v1/iot/rules                  # 规则列表
```

### 传播动作

GIF：**"AI generates a smart home dashboard, connected to real devices, running on a Raspberry Pi"**

---

## Wave 4：协议对齐（Week 9-10）

> **触发信号**：500+ stars，有企业/团队用户询问。A2A 生态已经有实际案例。
> **目标**：对齐行业标准，为 v2.0 企业版做准备。

### 功能清单

| # | 功能 | 工作量 | DD 参考 |
|---|------|--------|--------|
| 4.1 | **A2A Server + Agent Card** | 2 天 | DD-03 §4 |
| 4.2 | A2A 编排器（内部多 Agent） | 2 天 | DD-03 §5 |
| 4.3 | **OTel GenAI Collector** | 1.5 天 | DD-10 §8 |
| 4.4 | AG-UI 事件流渲染 | 1.5 天 | DD-08 §7 |
| 4.5 | A2UI 声明式 UI + 可信目录 | 1.5 天 | DD-08 §8 |
| 4.6 | Governance Agent 基础版 | 1 天 | DD-03 §7 |
| 4.7 | MCP Elicitation | 1 天 | DD-07 §4 |
| 4.8 | 多用户 RBAC | 1.5 天 | DD-01 §2 |

### 代码变更

```bash
# A2A — 全新目录
internal/a2a/server/server.go           # A2A Server
internal/a2a/orchestrator/orchestrator.go # 编排器
internal/a2a/governance/agent.go        # Governance

# OTel — 全新目录
internal/ecosystem/otel/collector.go    # OTel GenAI Collector

# MCP 扩展
internal/mcp/elicitation/handler.go     # Elicitation

# Auth 扩展
internal/auth/rbac.go                   # RBAC 多用户

# 新端点
GET  /.well-known/agent.json            # A2A Agent Card
POST /api/v1/a2a/tasks                  # A2A 任务
GET  /api/v1/otel/traces                # OTel 追踪
GET  /api/v1/otel/metrics/genai         # GenAI 指标
```

这一波完成后，BitEngine 的七层协议栈全部落地——对应 v7 架构文档的 v1.5 里程碑。

---

## Wave 5：企业版（Week 11+）

> **触发信号**：有团队/企业用户明确表达付费意愿。
> **目标**：多租户 + HA + 合规 = 商业化基础。

### 功能清单

| # | 功能 | DD 参考 |
|---|------|--------|
| 5.1 | 多租户 Workspace 隔离 | DD-09 §3 |
| 5.2 | Worker 节点调度 | DD-09 §4 |
| 5.3 | Active-Standby HA | DD-09 §5 |
| 5.4 | GPU 推理节点卸载 | DD-09 §6 |
| 5.5 | SSO (OIDC/SAML) | DD-01 §2 |
| 5.6 | 合规报告 + GDPR | DD-09 §7 |
| 5.7 | Ed25519 License 管理 | DD-09 §8 |

```bash
# 全新目录
internal/enterprise/tenant/
internal/enterprise/worker/
internal/enterprise/ha/
internal/enterprise/license/
internal/enterprise/compliance/

# 新的 docker-compose
deploy/docker-compose.ha.yml            # Primary + Standby + Sentinel
```

---

## 每波扩展的标准流程

无论做哪一波，流程一样：

```
1. 创建 Git 分支
   git checkout -b wave-2-rag-mcp

2. 新建迁移文件
   migrations/00X_waveN.sql

3. 新增 internal/ 下的目录和文件
   （参考上面的代码变更清单）

4. 在 api/router.go 注册新路由
   r.Route("/api/v1/data", dataHandler.Routes)

5. 如果需要新服务，加 docker-compose overlay
   deploy/docker-compose.waveN.yml

6. 写测试 → curl 验证 → 提交
   git commit -m "feat(datahub): RAG engine with ChromaDB"

7. 合并 → 发布 → 写 changelog → 社区传播
```

---

## DD 文档怎么用

你之前写的 DD-01 到 DD-10 现在变成了**施工蓝图**：

| 当你做… | 打开… | 看哪些章节 |
|---------|-------|----------|
| Wave 1 迭代更新 | DD-02 | §11 应用迭代, §6 代码审查 |
| Wave 2 RAG | DD-04 | §6 RAG 引擎 (完整实现方案) |
| Wave 2 MCP | DD-07 | §2-3 MCP Server (JSON-RPC, 工具注册) |
| Wave 3 IoT | DD-05 | 全文 (Aggregator, Bridge, 规则引擎) |
| Wave 4 A2A | DD-03 | §4 A2A Server, §5 编排器 |
| Wave 4 OTel | DD-10 | §8 OTel GenAI Collector |
| Wave 5 企业 | DD-09 | 全文 (多租户, HA, License) |
| 安全审查 | DD-06 | 全文 (十层防御, 新协议攻击面) |

每个 DD 文档里有完整的 Go 代码骨架、数据库 Schema、API 端点定义、测试策略。直接复制到 Claude Code 的 prompt 里用。

---

## 决策框架：下一波做什么？

每波结束后问三个问题：

```
1. GitHub issues 里呼声最高的功能是什么？
   → 做那个

2. 有没有竞品刚发布了我们没有的功能？
   → 跑 competitive radar scanner，评估是否紧急

3. 有没有付费意愿的信号？
   → 有 → 优先 Wave 5 企业版
   → 没有 → 继续做差异化功能
```

不要按照 DD 编号顺序做。按照用户需要什么做。
