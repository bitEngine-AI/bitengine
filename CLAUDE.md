# CLAUDE.md — BitEngine MVP（战时模式）

> **目标：2 周内上线。一句话 → AI 生成 Web 应用 → Docker 部署 → 浏览器访问。**
> **不做的事：IoT、MQTT、A2A、OTel、企业版、多租户、RAG、数据湖、WASM。全部砍掉。**

## 技术栈

| 组件 | 技术 | 版本 |
|------|------|------|
| 后端 | Go | 1.26 |
| 前端 | React + TypeScript | React 19 |
| 数据库 | PostgreSQL | 16 |
| 缓存/事件 | Redis | 7 |
| AI 推理 | Ollama | latest |
| 反向代理 | Caddy | 2 |
| AI 生成应用栈 | Python (Flask) + SQLite + HTML/JS | — |

## 代码布局

```
bitengine/
├── cmd/bitengined/main.go       # 主入口
├── internal/
│   ├── config/config.go         # 环境变量配置
│   ├── auth/auth.go             # JWT 单用户认证
│   ├── setup/wizard.go          # 首次设置向导
│   ├── ai/
│   │   ├── ollama.go            # Ollama HTTP 客户端
│   │   ├── intent.go            # 意图理解（Qwen3-4B）
│   │   ├── codegen.go           # 代码生成（云端 API）
│   │   └── review.go            # 代码审查（Phi-4-mini）
│   ├── runtime/
│   │   ├── docker.go            # Docker 容器生命周期
│   │   ├── builder.go           # 镜像构建
│   │   └── network.go           # 网络隔离
│   ├── apps/
│   │   ├── service.go           # 应用 CRUD
│   │   ├── generator.go         # AI 生成流水线（意图→代码→审查→部署）
│   │   └── templates.go         # 内置模板
│   ├── monitor/monitor.go       # 系统指标
│   ├── backup/backup.go         # 每日备份
│   ├── eventbus/bus.go          # Redis Pub/Sub 事件
│   └── caddy/caddy.go           # Caddy 路由管理
├── api/
│   ├── router.go                # 路由注册
│   ├── auth.go                  # /api/v1/auth/*
│   ├── apps.go                  # /api/v1/apps/*
│   ├── ai.go                    # /api/v1/ai/*
│   └── system.go                # /api/v1/system/*
├── web/src/                     # React 前端
│   ├── App.tsx
│   ├── pages/Desktop.tsx        # 应用桌面
│   ├── pages/Setup.tsx          # 设置向导
│   ├── components/AIPanel.tsx   # AI 对话面板
│   ├── components/AppCard.tsx   # 应用卡片
│   ├── components/Progress.tsx  # 生成进度
│   ├── stores/appStore.ts       # Zustand
│   └── api/client.ts            # API 客户端
├── migrations/001_init.sql
├── templates/                   # 5 个应用模板
├── deploy/
│   ├── docker-compose.yml
│   ├── Dockerfile.dev
│   ├── Caddyfile
│   └── entrypoint-dev.sh
├── Makefile
└── .air.toml
```

## 编码规范

- 错误：`fmt.Errorf("模块: %w", err)`
- 首参：`context.Context`
- 日志：`slog.Info("msg", "key", val)`
- ID：UUIDv7
- API 错误：`{"error":{"code":"XX","message":"...","trace_id":"uuid"}}`
- 路由前缀：`/api/v1/`

## API 路由（MVP 全部）

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/api/v1/system/status` | 健康检查 |
| GET | `/api/v1/system/metrics` | CPU/内存/磁盘 |
| GET | `/api/v1/setup/status` | 向导状态 |
| POST | `/api/v1/setup/step/:n` | 完成向导步骤 |
| POST | `/api/v1/auth/login` | 登录 → TokenPair |
| POST | `/api/v1/auth/refresh` | 刷新 token |
| POST | `/api/v1/apps` | 创建应用（AI 生成，SSE 流） |
| GET | `/api/v1/apps` | 应用列表 |
| GET | `/api/v1/apps/:id` | 应用详情 |
| DELETE | `/api/v1/apps/:id` | 删除应用 |
| POST | `/api/v1/apps/:id/start` | 启动 |
| POST | `/api/v1/apps/:id/stop` | 停止 |
| GET | `/api/v1/apps/:id/logs` | 容器日志 |
| GET | `/api/v1/apps/templates` | 模板列表 |
| POST | `/api/v1/apps/templates/:slug/deploy` | 部署模板 |
| GET | `/api/v1/ai/models` | 模型状态 |

## 数据库（一个 Schema，6 张表）

```sql
platform.users          -- 单用户
platform.config         -- 配置 KV
platform.setup_state    -- 向导状态
runtime.apps            -- 应用实例
runtime.templates       -- 模板
runtime.app_logs        -- 生成日志
```

## Docker Compose（4 个服务）

bitengined + PostgreSQL + Redis + Ollama + Caddy

## 关键流程

```
用户输入 "做一个项目看板"
  → POST /api/v1/apps {prompt: "做一个项目看板"}
  → SSE 事件流开始
  → 1. Qwen3-4B 意图理解 → JSON
  → 2. 云端大模型代码生成 → Python/Flask 代码
  → 3. Phi-4-mini 代码审查 → 通过
  → 4. 写入文件系统 + 生成 Dockerfile
  → 5. docker build + docker run
  → 6. Caddy 添加路由 app-xxx.bit.local
  → SSE: {event: "complete", url: "http://app-xxx.bit.local"}
  → 前端显示应用卡片，点击即可访问
```

## 不要做的事

- ❌ MCP Server/Client
- ❌ A2H 多渠道通知
- ❌ A2A Agent 协作
- ❌ MQTT Broker
- ❌ OTel 追踪
- ❌ IoT 设备
- ❌ RAG 知识库
- ❌ 数据湖/管道/可视化
- ❌ WASM 沙箱
- ❌ 多用户 RBAC
- ❌ 企业多租户
- ❌ 蓝绿部署
- ❌ Cron 调度
- ❌ 能力市场
