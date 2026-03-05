# Task 6 ▸ 端到端流水线 + SSE 流式响应
# 预估：4-5 小时（最关键的一个 Task）
# ─────────────────────────────────────

## 目标
串联全部链路：POST /api/v1/apps → 意图理解 → 代码生成 → 构建 → 部署 → 返回 URL。通过 SSE 实时推送进度。

## 参考文件
@CLAUDE.md
@internal/ai/intent.go（Task 3）
@internal/ai/codegen.go（Task 4）
@internal/runtime/docker.go（Task 5）

## 要做的事

1. `internal/apps/generator.go` — AI 生成流水线：
   ```
   GenerateApp(ctx, prompt, sseWriter) → AppInstance
     step 1: IntentEngine.Understand(prompt) → IntentResult    → SSE: {step:"intent",status:"done"}
     step 2: CodeGen.Generate(intentResult)  → GeneratedCode   → SSE: {step:"codegen",status:"done"}
     step 3: Review.Check(code) → ReviewResult                 → SSE: {step:"review",score:95}
     step 4: Runtime.Build(code) → imageTag                    → SSE: {step:"build",status:"done"}
     step 5: Runtime.Start(imageTag) → containerID             → SSE: {step:"deploy",status:"done"}
     step 6: Caddy.AddRoute(slug, port)                        → SSE: {step:"route",url:"http://..."}
     → 写入 DB → SSE: {step:"complete",app_id:"...",url:"http://..."}
   ```

2. `internal/apps/service.go` — 应用 CRUD：
   - List, Get, Delete, Start, Stop, Logs

3. `api/apps.go` — 应用 API：
   - POST /apps → SSE 流式响应（Content-Type: text/event-stream）
   - GET /apps, GET /apps/:id, DELETE /apps/:id
   - POST /apps/:id/start, POST /apps/:id/stop
   - GET /apps/:id/logs

## 验收（这是 MVP 的核心验收）

```bash
# 创建应用（SSE 流）
curl -N -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"prompt":"做一个简单的待办事项应用"}' \
  http://localhost:9000/api/v1/apps

# 输出 SSE 事件流：
# data: {"step":"intent","status":"running"}
# data: {"step":"intent","status":"done","result":{...}}
# data: {"step":"codegen","status":"running"}
# data: {"step":"codegen","status":"done"}
# data: {"step":"build","status":"running"}
# data: {"step":"build","status":"done"}
# data: {"step":"deploy","status":"done","url":"http://localhost:3001"}
# data: {"step":"complete","app_id":"abc123","url":"http://localhost:3001"}

# 验证应用在运行
curl http://localhost:3001
# → HTML 页面

# 应用列表
curl -H "Authorization: Bearer <token>" http://localhost:9000/api/v1/apps
# → [{id, name, slug, status:"running", domain, ...}]
```

**这个 Task 完成后，核心价值主张已经成立：一句话 → 运行中的应用。**

## 请先列出实现计划，确认后开始编码。
