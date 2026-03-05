# Task 3 ▸ Ollama 客户端 + 意图理解引擎
# 预估：3-4 小时
# ─────────────────────────────────────

## 目标
封装 Ollama HTTP API，实现意图理解：用户输入自然语言 → Qwen3-4B 输出 JSON（intent/app_name/requirements）。

## 参考文件
@CLAUDE.md

## 要做的事

1. `internal/ai/ollama.go` — Ollama HTTP 客户端（Chat, Generate, ListModels, IsAvailable）
2. `internal/ai/intent.go` — 意图理解引擎：
   - 输入：用户自然语言
   - 调用 Qwen3-4B + JSON Schema 约束
   - 输出：`{intent: "create_app", app_name: "xxx", requirements: {...}}`
3. `api/ai.go` — GET /ai/models 返回模型状态，POST /ai/intent 调试用

## 意图输出 Schema

```json
{
  "intent": "create_app",
  "app_name": "project-board",
  "description": "项目看板，支持任务拖拽",
  "requirements": {
    "features": ["任务创建", "拖拽排序", "状态切换"],
    "data_model": "tasks with title, status, priority, due_date",
    "ui_style": "kanban board"
  },
  "confidence": 0.92
}
```

## 验收

```bash
# 模型状态
curl -H "Authorization: Bearer <token>" http://localhost:9000/api/v1/ai/models
# → [{"name":"qwen3:4b","status":"loaded"}, ...]

# 意图分析
curl -X POST -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"input":"做一个项目看板，支持任务拖拽和状态切换"}' \
  http://localhost:9000/api/v1/ai/intent
# → {intent: "create_app", app_name: "project-board", ...}
```

注意：先 `make models` 拉取 qwen3:4b。

## 请先列出实现计划，确认后开始编码。
