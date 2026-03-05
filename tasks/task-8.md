# Task 8 ▸ 内置模板 + 安装脚本 + 收尾
# 预估：3-4 小时
# ─────────────────────────────────────

## 目标
5 个一键部署模板，install.sh 安装脚本，系统指标 API，准备上线 GitHub。

## 参考文件
@CLAUDE.md

## 要做的事

1. **5 个应用模板**（每个是一个完整的 Flask 应用目录）：
   - `templates/todo/` — 待办事项看板
   - `templates/accounting/` — 个人记账
   - `templates/crm/` — 简易 CRM
   - `templates/form-builder/` — 表单构建器
   - `templates/dashboard/` — 数据看板
   
   每个模板包含：app.py + templates/ + static/ + requirements.txt + Dockerfile
   
2. `internal/apps/templates.go` — 模板服务：
   - ListTemplates() — 列出可用模板
   - DeployTemplate(slug) — 构建+部署指定模板

3. `api/apps.go` 补充：
   - GET /apps/templates — 模板列表
   - POST /apps/templates/:slug/deploy — 一键部署

4. `internal/monitor/monitor.go` — 系统指标（gopsutil）：
   - CPU 使用率, 内存使用量, 磁盘使用量
   - GET /api/v1/system/metrics

5. `scripts/install.sh` — 安装脚本：
   - 检测 Docker
   - 拉取 docker-compose.yml
   - 启动服务
   - 输出访问地址

6. `README.md` — GitHub 首页：
   - 30 秒 GIF 演示
   - 一行安装命令
   - 功能截图

## 验收

```bash
# 模板列表
curl -H "Authorization: Bearer <token>" http://localhost:9000/api/v1/apps/templates
# → [{slug:"todo", name:"待办事项", ...}, ...]

# 一键部署模板
curl -X POST -H "Authorization: Bearer <token>" \
  http://localhost:9000/api/v1/apps/templates/todo/deploy
# → {id, name, status:"running", domain:"http://..."}

# 系统指标
curl -H "Authorization: Bearer <token>" http://localhost:9000/api/v1/system/metrics
# → {cpu_percent: 23.5, memory_used_gb: 14.2, disk_used_gb: 28, ...}
```

**这个 Task 完成后，BitEngine MVP 可以上 GitHub 了。**

## 请先列出实现计划，确认后开始编码。
