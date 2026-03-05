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
