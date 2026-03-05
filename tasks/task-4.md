# Task 4 ▸ 代码生成器（云端 API）
# 预估：3-4 小时
# ─────────────────────────────────────

## 目标
将意图理解的 JSON 结果发送给云端大模型（Claude Sonnet / DeepSeek），生成完整的 Flask 应用代码。

## 参考文件
@CLAUDE.md
@internal/ai/intent.go（Task 3 产出）

## 要做的事

1. `internal/ai/codegen.go` — 代码生成器：
   - 输入：IntentResult JSON
   - 构造 prompt（含技术栈约束：Flask + SQLite + HTML/JS）
   - 调用 Anthropic API 或 DeepSeek API
   - 输出：GeneratedCode{files: map[string]string, dockerfile: string}
   
2. `internal/ai/review.go` — 代码审查（可选，Phi-4-mini 本地）：
   - 基础检查：有没有硬编码密钥、SQL 注入风险
   - 输出：{passed: bool, issues: [], score: 0-100}

3. 生成的代码结构约定：
```
app.py              # Flask 入口
templates/           # HTML 模板
  index.html
  base.html
static/             # CSS/JS
  style.css
  app.js
requirements.txt    # Flask + 依赖
Dockerfile          # 自动生成
```

## 验收

```bash
# 先设置云端 API Key
export ANTHROPIC_API_KEY=sk-ant-...
# 或 export DEEPSEEK_API_KEY=sk-...

# 生成代码（调试接口）
curl -X POST -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"intent":"create_app","app_name":"todo","requirements":{"features":["添加任务","完成任务","删除任务"]}}' \
  http://localhost:9000/api/v1/ai/generate
# → {files: {"app.py": "...", "templates/index.html": "...", ...}, dockerfile: "FROM python:3.12-slim..."}
```

## 请先列出实现计划，确认后开始编码。
