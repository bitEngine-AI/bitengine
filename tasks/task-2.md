# Task 2 ▸ JWT 认证 + 首次设置向导
# 预估：2-3 小时
# ─────────────────────────────────────

## 目标
单用户 JWT 认证。首次访问进入设置向导创建管理员账户。

## 参考文件
@CLAUDE.md
@internal/config/config.go（Task 1 产出）
@api/router.go（Task 1 产出）

## 要做的事

1. `internal/auth/auth.go` — JWT 生成/验证，bcrypt 密码哈希，AuthMiddleware
2. `internal/setup/wizard.go` — 向导状态机（step 0: 未开始 → step 1: 创建用户 → completed）
3. `api/auth.go` — POST /login, POST /refresh, POST /logout
4. `api/setup.go` — GET /setup/status, POST /setup/step/1

## 验收

```bash
# 向导状态
curl http://localhost:9000/api/v1/setup/status
# → {"completed":false,"step":0}

# 创建管理员
curl -X POST http://localhost:9000/api/v1/setup/step/1 \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"test1234"}'
# → 201

# 登录
curl -X POST http://localhost:9000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"test1234"}'
# → {"access_token":"eyJ...","refresh_token":"...","expires_in":3600}

# 认证访问
curl -H "Authorization: Bearer <token>" http://localhost:9000/api/v1/system/status
# → 200

# 无 token
curl http://localhost:9000/api/v1/apps
# → 401
```

## 请先列出实现计划，确认后开始编码。
