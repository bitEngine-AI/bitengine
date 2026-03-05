cat >> docs/DECISIONS.md << 'EOF'
# BitEngine 架构决策日志

> 代码变更的设计决策记录。日后回溯或更新 DD 文档时参考。

---

## D-001: 代码生成双模式（本地 + 云端）

**日期**: 2026-03-05
**状态**: 已实现
**影响**: DD-02 §5 代码生成器

**背景**: MVP 原设计要求用户提供 ANTHROPIC_API_KEY 或 DEEPSEEK_API_KEY 才能生成代码。这会拦住大量试用用户。

**决策**: 
- 默认本地模式：用 Ollama qwen3:4b 生成代码，零配置
- 增强模式：检测到 API Key 自动切换云端大模型
- 启动日志明确告知当前模式

**实现**:
- internal/ai/codegen.go: LocalGenerator + CloudGenerator + 自动选择
- internal/config/config.go: API Key 改为 optional
- api/system.go: status 增加 codegen_mode 字段

**权衡**: 本地模型生成质量低于云端，限制为单文件 Flask 应用。用户可随时加 Key 升级。
EOF

git add docs/DECISIONS.md
git commit -m "docs: add architecture decision log D-001"
```

**流程就是**：
```
每次功能变更 →  1. 在 DECISIONS.md 追加一条（30 秒）
               2. 粘贴 prompt 到 Claude Code 改代码
               3. git commit 代码 + 决策日志一起提交

---

## D-002: MVP 验证阶段构建修复

**日期**: 2026-03-05
**状态**: 已修复
**背景**: Task 8 完成后验证前端，发现构建和部署链路有缺失。

### 修复清单

| # | 问题 | 原因 | 修复 | 文件 |
|---|------|------|------|------|
| 1 | `npm: not found` | Dockerfile.dev 只装了 Go，没有 Node.js | 用独立 `node:20-alpine` 容器构建前端 | — |
| 2 | Caddy 返回空白页 | `web/dist/` 未挂载到 Caddy 的 `/srv` | volumes 加 `../web/dist:/srv` | `deploy/docker-compose.yml` |
| 3 | caddy 服务 YAML 解析失败 | 缩进跑到 services 外层 | 修复缩进 | `deploy/docker-compose.yml` |
| 4 | `depends_on: edgeforged` 找不到 | 全局替换时服务名应为 `bitengined` | 改为 `depends_on: [bitengined]` | `deploy/docker-compose.yml` |

### 前端构建命令
```bash
# 手动构建（当前方式）
docker run --rm -v $(pwd)/web:/app -w /app node:20-alpine sh -c "npm install && npm run build"
```

### 待办
- [ ] Makefile 加 `make web` 命令封装上述构建
- [ ] entrypoint-dev.sh 加首次自动构建逻辑
