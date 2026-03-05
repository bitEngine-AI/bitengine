# BitEngine

> 一句话 → AI 生成 Web 应用 → Docker 部署 → 浏览器访问

[English](#english) | 中文

## 什么是 BitEngine？

BitEngine 是一个 AI 驱动的 Web 应用生成平台。输入一句自然语言描述，BitEngine 自动：

1. 🧠 理解意图（本地 Qwen3-4B）
2. 💻 生成代码（云端大模型）
3. 🔍 审查安全（本地 Phi-4-mini）
4. 📦 构建镜像（Docker）
5. 🚀 部署运行（容器 + 反向代理）

**30 秒内，从想法到可访问的 Web 应用。**

## 快速开始

### 前置要求
- Docker + Docker Compose
- 8GB+ 内存（运行本地 AI 模型）

### 一行安装
```bash
curl -fsSL https://get.bitengine.dev | bash
```

或手动安装：
```bash
git clone https://github.com/bitEngine-AI/bitengine.git
cd bitengine
cp .env.example .env  # 编辑配置
docker compose up -d
```

打开 http://localhost:9000 → 完成设置向导 → 开始创建应用！

## 核心流程

```
用户输入: "做一个项目看板，支持任务拖拽"
  ↓
意图理解 (Qwen3-4B, 本地)
  ↓
代码生成 (Claude/DeepSeek, 云端)
  ↓
安全审查 (Phi-4-mini, 本地)
  ↓
Docker 构建 + 部署
  ↓
http://app-project-board.bit.local ← 可访问！
```

## 技术栈

| 组件 | 技术 |
|------|------|
| 后端 | Go 1.26 |
| 前端 | React 19 + TypeScript + Tailwind |
| 数据库 | PostgreSQL 16 |
| 缓存 | Redis 7 |
| AI 推理 | Ollama (Qwen3-4B, Phi-4-mini) |
| AI 代码生成 | Anthropic Claude / DeepSeek |
| 容器运行时 | Docker |
| 反向代理 | Caddy 2 |
| 生成应用栈 | Python Flask + SQLite |

## 内置模板

| 模板 | 说明 |
|------|------|
| 📋 待办事项 | 看板式任务管理，支持拖拽 |
| 💰 个人记账 | 收支记录，分类统计 |
| 👥 简易 CRM | 客户管理，状态追踪 |
| 📝 表单构建器 | 自定义表单，数据收集 |
| 📊 数据看板 | KPI 指标，数据可视化 |

## API 概览

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `/api/v1/apps` | 创建应用 (SSE 流) |
| GET | `/api/v1/apps` | 应用列表 |
| GET | `/api/v1/apps/:id` | 应用详情 |
| DELETE | `/api/v1/apps/:id` | 删除应用 |
| POST | `/api/v1/apps/:id/start` | 启动 |
| POST | `/api/v1/apps/:id/stop` | 停止 |
| GET | `/api/v1/apps/templates` | 模板列表 |
| POST | `/api/v1/apps/templates/:slug/deploy` | 部署模板 |
| GET | `/api/v1/system/status` | 系统状态 |
| GET | `/api/v1/system/metrics` | 系统指标 |

## 开发

```bash
# 启动依赖
docker compose -f deploy/docker-compose.yml up -d db redis ollama

# 后端
go run ./cmd/bitengined

# 前端
cd web && npm run dev
```

## License

[MIT](LICENSE)

---

<a id="english"></a>

## English

BitEngine is an AI-powered web application generator. Describe what you want in one sentence, and BitEngine automatically understands your intent, generates code, reviews it for security, builds a Docker image, and deploys it — all in under 30 seconds.

### Quick Start

```bash
curl -fsSL https://get.bitengine.dev | bash
```

Open http://localhost:9000 and follow the setup wizard.

Built with Go, React, PostgreSQL, Redis, Ollama, and Docker.
