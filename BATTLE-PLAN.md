# BitEngine MVP — 2 周作战计划

> **目标**：2 周内上 GitHub，核心验证"一句话→完整应用→Docker 运行"。

## 时间线

| 天 | Task | 产出 | 验证 |
|----|------|------|------|
| D1 | T1: 配置+DB+Redis | 健康检查 API | curl /status → ok |
| D1 | T2: JWT+设置向导 | 登录+认证 | curl /auth/login → token |
| D2 | T3: Ollama+意图 | 意图理解 | curl /ai/intent → JSON |
| D3 | T4: 代码生成 | Flask 代码 | curl /ai/generate → 文件 |
| D4-5 | T5: Docker 运行时 | 容器管理 | 手动 build+run |
| D6-7 | **T6: 端到端** | **核心链路** | **一句话→运行的应用** |
| D8-9 | T7: React 前端 | Web UI | 浏览器完整体验 |
| D10 | T8: 模板+安装 | 5模板+install.sh | 新机器一键安装 |
| D11 | 录制 GIF + README | 传播素材 | 30秒演示 |
| D12 | **上 GitHub** | 🚀 | 发 HN/Reddit |

## 每日流程

```
早上：./run-task.sh next → 拿到任务
     ./run-task.sh <N> → 复制 prompt 到 Claude Code
白天：Claude Code 写代码 → air 自动重载 → curl 验证
晚上：./run-task.sh done <N> → 提交 git
```

## 启动命令

```bash
make dev          # 启动基础设施
make models       # 拉取 AI 模型（首次）
./run-task.sh 1   # 开始第一个任务
```

## 上线后传播

1. GIF: 终端 install → 浏览器输入 → 应用生成 → 运行
2. Hacker News: "Show HN: One sentence → deployed web app, running on your machine"
3. r/selfhosted: "I built a local AI that generates and deploys web apps"
4. r/LocalLLaMA: "Using Qwen3-4B + Claude to generate full-stack apps locally"
5. V2EX / 即刻: "一句话生成 Web 应用，全部跑在你自己的机器上"
