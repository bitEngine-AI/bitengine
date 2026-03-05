# BitEngine 开发指南（唯一版本）

> **本文档是你唯一需要读的操作手册。**
>
> 旧文件说明：如果你之前拿到过 `bitengine-starter-v7.zip`（76 个任务的完整版）或 `bitengine-usage-guide-v7.md`，**请忽略它们**。那些是架构设计阶段的产物。现在进入执行阶段，用本包即可。

---

## 这个包里有什么

```
bitengine/
├── CLAUDE.md              ← Claude Code 自动读取的项目说明（已精简到 MVP）
├── BATTLE-PLAN.md         ← 12 天作战计划（一页纸看完）
├── Makefile               ← 所有命令入口
├── run-task.sh            ← 任务执行器（8 个任务）
├── tasks/                 ← 8 个预生成的 Claude Code prompt
│   ├── task-1.md          ← 配置 + DB + Redis + 健康检查
│   ├── task-2.md          ← JWT 认证 + 设置向导
│   ├── task-3.md          ← Ollama + 意图理解
│   ├── task-4.md          ← 代码生成（云端 API）
│   ├── task-5.md          ← Docker 容器运行时
│   ├── task-6.md          ← 端到端流水线（最关键）
│   ├── task-7.md          ← React 前端
│   └── task-8.md          ← 模板 + 安装脚本 + 上线准备
├── cmd/bitengined/main.go ← Go 主入口（骨架）
├── internal/              ← 后端代码目录（按模块）
├── api/                   ← HTTP handler 目录
├── web/                   ← React 前端目录
├── migrations/001_init.sql ← 数据库初始化
├── templates/             ← 5 个应用模板目录
├── deploy/                ← Docker Compose + Caddy + 启动脚本
├── docs/                  ← 设计文档（后续扩展时参考）
│   ├── bitengine-expansion-roadmap.md   ← MVP 之后怎么扩展
│   ├── bitengine-architecture-v7.md     ← 架构愿景（参考）
│   ├── bitengine-hld-v7.md              ← 总体设计（参考）
│   ├── bitengine-dd01 ~ dd10            ← 10 份详细设计（参考）
│   └── bitengine-competitive-radar-v7.md ← 竞品雷达
└── .env.example           ← 环境变量模板
```

**关键区分**：
- **现在用的**：根目录的文件（CLAUDE.md / tasks/ / Makefile / deploy/）
- **以后用的**：docs/ 下的文件（DD 文档是 Wave 1-5 扩展时的施工蓝图）

---

## 前置条件

| 工具 | 必须？ | 说明 |
|------|--------|------|
| Docker + Docker Compose ≥ 24.0 | **必须** | 所有服务都在容器里 |
| Git ≥ 2.40 | **必须** | 版本控制 |
| Claude Code CLI | **必须** | AI 写代码 |
| 一个云端 API Key | **必须** | Anthropic 或 DeepSeek（代码生成用） |
| Make | 推荐 | 命令快捷入口 |

**不需要**本地装 Go、Node.js、PostgreSQL。全在 Docker 里。

---

## 第一步：启动

```bash
# 解压
unzip bitengine-final.zip
cd bitengine

# 复制环境变量并填入你的 API Key
cp .env.example .env
# 编辑 .env，填入 ANTHROPIC_API_KEY 或 DEEPSEEK_API_KEY

# 启动开发环境（一条命令，约 2 分钟）
make dev

# 另开一个终端，拉取 AI 模型（首次约 10 分钟）
make models

# 验证
curl http://localhost:9000/api/v1/system/status
# → {"status":"ok","version":"0.1.0-mvp"}
```

启动后你有 5 个服务在运行：

| 服务 | 端口 | 用途 |
|------|------|------|
| be-core | 9000 | BitEngine 后端（热重载） |
| be-postgres | 5432 | 数据库 |
| be-redis | 6379 | 事件总线 |
| be-ollama | 11434 | AI 模型推理 |
| be-caddy | 80 | 反向代理 |

---

## 第二步：逐个完成 8 个任务

每天的工作流是一样的：

```bash
# 1. 看下一个任务
./run-task.sh next
# → "下一个: Task 1 ▸ 配置加载 + DB/Redis 连接 + 健康检查"

# 2. 获取完整 prompt
./run-task.sh 1
# → 输出一段文字，包含目标、参考文件、验收标准

# 3. 复制粘贴到 Claude Code
# Claude Code 会自动读取 CLAUDE.md，然后根据 prompt 写代码
# 代码保存后 air 自动编译重启（~1 秒）

# 4. 验证
curl http://localhost:9000/api/v1/system/status
# → 按 prompt 里的验收标准检查

# 5. 标记完成
./run-task.sh done 1
git add -A && git commit -m "feat(config): DB + Redis + health check"

# 6. 看进度
./run-task.sh status
# → 进度: 1/8 tasks [██░░░░░░░░░░░░░░░░░░] 12%
```

### 8 个任务的推荐时间线

| 天 | 任务 | 做完后能验证什么 |
|----|------|----------------|
| **D1** | Task 1 + Task 2 | `curl /auth/login → token` |
| **D2** | Task 3 | `curl /ai/intent → JSON 意图结果` |
| **D3** | Task 4 | `curl /ai/generate → Flask 应用代码` |
| **D4-5** | Task 5 | Docker 能自动构建和启动容器 |
| **D6-7** | **Task 6** | **一句话 → 浏览器打开运行中的应用** ⭐ |
| **D8-9** | Task 7 | 浏览器完整 UI 体验 |
| **D10** | Task 8 | 5 个模板可一键部署 + install.sh |

**Task 6 是核心**——完成它的那一刻，BitEngine 的核心价值主张成立。

---

## 第三步：上 GitHub

Task 8 完成后：

```bash
# 1. 录制 30 秒 GIF（用 asciinema 或屏幕录制）
#    内容：install → 浏览器 → 输入需求 → 进度条 → 应用运行

# 2. 写 README.md（Task 8 会帮你生成骨架）

# 3. 推送
git remote add origin git@github.com:你的用户名/bitengine.git
git push -u origin main

# 4. 发布传播
# Hacker News: "Show HN: One sentence → deployed web app on your machine"
# r/selfhosted: "I built a local AI that generates and deploys web apps"
# r/LocalLLaMA: "Using Qwen3-4B + Claude to generate full-stack apps locally"
```

---

## 第四步：MVP 之后怎么扩展

上 GitHub 拿到用户反馈后，按 5 波扩展推进。详见 `docs/bitengine-expansion-roadmap.md`：

| 波次 | 时间 | 功能 | 触发信号 |
|------|------|------|---------|
| **Wave 1** | Week 3-4 | 应用迭代 + A2H 弹窗 + 更多模板 | 有人在用 |
| **Wave 2** | Week 5-6 | RAG 知识库 + MCP Server + OpenAI API | 社区问"能搜文档吗""能从 Claude 控制吗" |
| **Wave 3** | Week 7-8 | IoT 设备 + MQTT 5.0 + AI 规则 | 社区问"能控制智能家居吗" |
| **Wave 4** | Week 9-10 | A2A + OTel + AG-UI/A2UI + RBAC | 500+ stars，有团队用户 |
| **Wave 5** | Week 11+ | 多租户 + HA + License | 有人愿意付费 |

每波扩展时，打开 `docs/` 里对应的 DD 文档，找到具体章节复制到 Claude Code prompt 里——DD 文档就是施工蓝图。

**扩展不需要重构 MVP 代码**：每波是加新目录 + 新文件 + 注册新路由，不改现有代码。

---

## 常用命令速查

```bash
make dev          # 启动开发环境（前台，看日志）
make dev-bg       # 后台启动
make stop         # 停止
make logs         # bitengined 实时日志
make status       # 所有服务状态
make models       # 拉取 AI 模型
make test         # 跑测试
make shell        # 进入后端容器
make db           # 进入 PostgreSQL
make clean        # 停止 + 删除数据卷（慎用）

./run-task.sh list    # 任务列表 + 完成状态
./run-task.sh next    # 下一个待做任务
./run-task.sh <1-8>   # 输出指定任务的 prompt
./run-task.sh done N  # 标记完成
./run-task.sh status  # 进度条
```

---

## 常见问题

**Q: 之前的 bitengine-starter-v7.zip 还用吗？**
不用。那个是 76 任务的完整架构版，现在用本包（8 任务 MVP 版）。所有设计文档已复制到 `docs/` 目录，后续扩展时参考。

**Q: 之前的 DD-01~DD-10 文档白写了吗？**
没有。它们现在是 `docs/` 里的施工蓝图。Wave 2 做 RAG 时打开 DD-04 §6，Wave 3 做 IoT 时打开 DD-05——每个 DD 里有完整的 Go 代码、Schema、API、测试策略。

**Q: Claude Code 上下文溢出怎么办？**
`/compact` 压缩或 `/clear` 清空。每个 Task prompt 只引用必要文件，不会溢出。

**Q: 本地没 GPU 能跑吗？**
能。Ollama 支持 CPU 推理，只是慢一些。意图理解（Qwen3-4B）约 5-10 秒，代码审查（Phi-4-mini）约 3-5 秒。代码生成走云端 API 不受影响。

**Q: 需要什么配置的机器？**
最低 16GB RAM（Ollama 占约 4-6GB）。推荐 32GB。

**Q: MQTT / A2A / OTel 什么时候加？**
看 `docs/bitengine-expansion-roadmap.md`。MQTT 在 Wave 3（Week 7-8），A2A 和 OTel 在 Wave 4（Week 9-10）。按用户反馈决定优先级。
