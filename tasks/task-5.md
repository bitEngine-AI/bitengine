# Task 5 ▸ Docker 容器运行时
# 预估：3-4 小时
# ─────────────────────────────────────

## 目标
将生成的代码自动构建为 Docker 镜像，启动容器，配置 Caddy 反向代理路由。

## 参考文件
@CLAUDE.md

## 要做的事

1. `internal/runtime/docker.go` — Docker 容器管理（通过 Go SDK）：
   - Create: 写代码文件 → docker build → docker create
   - Start/Stop/Remove: 容器生命周期
   - Logs: 容器日志流
   - Status: 运行状态

2. `internal/runtime/builder.go` — 镜像构建：
   - 将 GeneratedCode 写入临时目录
   - 自动生成 Dockerfile（如果没有）
   - docker build -t bitengine-app-{slug}:v1

3. `internal/runtime/network.go` — 网络隔离：
   - 每个应用创建独立 Docker 网络 ef-app-{slug}
   - 应用容器只能访问 PostgreSQL，不能互访

4. `internal/caddy/caddy.go` — Caddy Admin API 路由管理：
   - AddRoute: app-{slug}.bit.local → container:port
   - RemoveRoute: 删除应用时清理

## 验收

```bash
# 手动测试构建和启动
# 在容器内执行：
docker exec -it be-core sh
# 测试 Docker SDK 连接
# 构建一个测试镜像 → 启动 → curl 访问 → 停止 → 删除
```

## 请先列出实现计划，确认后开始编码。
