# Task 1 ▸ 配置加载 + DB/Redis 连接 + 健康检查
# 预估：2-3 小时
# ─────────────────────────────────────

## 目标
bitengined 启动时从环境变量读取配置，连接 PostgreSQL 和 Redis，自动执行 migrations，健康检查返回连接状态。

## 参考文件
@CLAUDE.md
@cmd/bitengined/main.go
@migrations/001_init.sql
@deploy/docker-compose.yml

## 要做的事

1. `internal/config/config.go` — 用 envconfig 从环境变量读取配置（DB URL, Redis URL, Ollama URL, JWT Secret, Listen Addr）
2. `cmd/bitengined/main.go` — 加载配置 → 连接 DB → 连接 Redis → 自动执行 migrations/ 下所有 .sql → 注册路由 → 启动 HTTP server
3. `api/router.go` — 用 chi 注册路由，挂载 CORS 中间件
4. `api/system.go` — GET /api/v1/system/status 返回 DB/Redis 连接状态

## 验收

```bash
make dev
# 等服务就绪后：
curl http://localhost:9000/api/v1/system/status
# → {"status":"ok","version":"0.1.0-mvp","db":"connected","redis":"connected"}
```

## 请先列出实现计划，确认后开始编码。
