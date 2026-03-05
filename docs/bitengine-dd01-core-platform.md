# DD-01：核心平台详细设计

> 模块路径：`internal/foundation/` | 完整覆盖 MVP · v1.0 · v2.0
>
> **v7 变更**：标题从"平台底座"改为"核心平台"。新增 MQTT 5.0 Broker 基础设施、MCP Server Registry、媒体文件存储策略。Redis 角色从"平台事件总线"收窄为"内部服务通信和缓存"（IoT 事件走 MQTT 5.0）。

---

## 1 模块职责

平台底座是用户无感但不可或缺的基础设施，包含十大子系统：

| 子系统 | 职责 | 阶段 |
|--------|------|------|
| auth | 认证中间件 + JWT | MVP |
| identity | 用户管理 + RBAC + 应用共享 | MVP(单用户) / v1.0(多用户) / v2.0(SSO) |
| network | Caddy 反向代理 + DNS + 远程访问 + 防火墙 | MVP / v1.0 / v2.0 |
| backup | 定时备份 + 快照 + 恢复 + 异地同步 | MVP / v2.0 |
| monitor | 资源采集 + 健康检查 + 告警规则 | MVP |
| database | Schema 管理 + 应用隔离 | MVP |
| **mqtt_broker** | **Mosquitto MQTT 5.0 Broker（IoT 数据面）（v7 新增）** | **v1.0** |
| **mcp_registry** | **MCP Server 注册与发现（v7 新增）** | **v1.0** |
| **media_storage** | **媒体文件存储（视频/音频/图片，v7 新增）** | **v1.0** |
| audit | Merkle 审计链 | MVP |
| vault | 密钥保险库 + 零化 | MVP |
| taint | 污点追踪 + 数据脱敏 | v1.0 |
| firewall | nftables + 限流 + Anti-DDoS | v1.0 / v2.0 |

---

## 2 认证与身份

> **OAuth 2.1 框架对齐**：BitEngine 的认证体系基于 OAuth 2.1（IETF）统一框架。JWT access_token / refresh_token 遵循 OAuth 2.1 Bearer Token 规范。MCP Server、A2A Server、OpenAI 兼容 API 的外部接口授权均通过 OAuth 2.1 Bearer Token 认证。内部服务间使用 HMAC-SHA256 签名。

### 2.1 认证中间件 (MVP)

```go
// internal/foundation/auth/middleware.go

type Claims struct {
    UID   string   `json:"uid"`
    Role  Role     `json:"role"`
    Apps  []string `json:"apps,omitempty"` // v1.0: 被授权的应用 ID 列表
}

type TokenPair struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int    `json:"expires_in"`
}

func AuthMiddleware(secret []byte) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 1. 从 Authorization: Bearer <token> 提取 token
            // 2. 验证 JWT 签名和过期时间
            // 3. 解析 Claims，注入 context
            // 4. 调用 next.ServeHTTP
        })
    }
}
```

```mermaid
sequenceDiagram
    participant U as 用户
    participant C as Caddy
    participant A as Auth 中间件
    participant S as 业务服务

    U->>C: POST /api/v1/auth/login {user, pass}
    C->>A: 转发
    A->>A: bcrypt 验证 → 生成 JWT
    A-->>U: {access_token, refresh_token}
    
    U->>C: GET /api/v1/apps (Bearer token)
    C->>A: 验证 JWT
    A->>A: 解析 Claims
    A->>S: 注入 Claims 到 context
    S-->>U: 应用列表
```

### 2.2 权限矩阵

#### MVP：单用户 Owner

安装时自动创建 owner 用户，拥有所有权限。

#### v1.0：多用户 RBAC

```go
// internal/foundation/identity/rbac.go

type Role string

const (
    RoleOwner  Role = "owner"   // 全部权限
    RoleAdmin  Role = "admin"   // 管理应用/数据/IoT/用户
    RoleMember Role = "member"  // 使用被授权的应用
    RoleGuest  Role = "guest"   // 只读访问共享应用
)

type Permission struct {
    Resource string // apps | data | iot | system | users
    Action   string // create | read | update | delete | use | manage
    Scope    string // * | {app_id} | {shared_link}
}

// 权限矩阵
var RolePermissions = map[Role][]Permission{
    RoleOwner: {
        {Resource: "*", Action: "*", Scope: "*"},
    },
    RoleAdmin: {
        {Resource: "apps", Action: "*", Scope: "*"},
        {Resource: "data", Action: "*", Scope: "*"},
        {Resource: "iot", Action: "*", Scope: "*"},
        {Resource: "users", Action: "manage", Scope: "*"},
        {Resource: "system", Action: "read", Scope: "*"},
    },
    RoleMember: {
        {Resource: "apps", Action: "use", Scope: "{assigned}"},
        {Resource: "data", Action: "read", Scope: "{assigned}"},
        {Resource: "iot", Action: "read", Scope: "*"},
    },
    RoleGuest: {
        {Resource: "apps", Action: "use", Scope: "{shared_link}"},
    },
}

func (s *IdentityService) CheckPermission(ctx context.Context, claims *Claims, resource, action, target string) error {
    // 1. 获取角色的权限列表
    // 2. 匹配 resource + action
    // 3. 检查 scope: * 通过; {assigned} 检查 claims.Apps; {shared_link} 检查共享表
    // 4. 不匹配返回 ErrForbidden
}
```

#### v1.0：用户管理

```go
// internal/foundation/identity/service.go

type CreateUserRequest struct {
    Username string `json:"username" validate:"required,min=3,max=32"`
    Password string `json:"password" validate:"required,min=8"`
    Role     Role   `json:"role" validate:"required,oneof=admin member guest"`
    Apps     []string `json:"apps,omitempty"` // member 角色需要指定
}

type User struct {
    ID        string    `json:"id" db:"id"`
    Username  string    `json:"username" db:"username"`
    PassHash  string    `json:"-" db:"pass_hash"`
    Role      Role      `json:"role" db:"role"`
    Active    bool      `json:"active" db:"active"`
    LastLogin time.Time `json:"last_login" db:"last_login"`
    CreatedBy string    `json:"created_by" db:"created_by"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
}

func (s *IdentityService) InviteUser(ctx context.Context, email string, role Role) (*InviteLink, error) {
    // 生成一次性邀请链接，有效期 72 小时
    // 用户点击链接后设置密码并激活账户
}
```

#### v1.0：应用级共享

```go
// internal/foundation/identity/sharing.go

type ShareLink struct {
    ID          string     `json:"id" db:"id"`
    AppID       string     `json:"app_id" db:"app_id"`
    Token       string     `json:"token" db:"token"`
    Password    string     `json:"-" db:"password_hash"` // 可选密码保护
    ReadOnly    bool       `json:"read_only" db:"read_only"`
    ExpiresAt   *time.Time `json:"expires_at,omitempty" db:"expires_at"`
    MaxViews    int        `json:"max_views" db:"max_views"` // 0 = 无限制
    ViewCount   int        `json:"view_count" db:"view_count"`
    CreatedBy   string     `json:"created_by" db:"created_by"`
    CreatedAt   time.Time  `json:"created_at" db:"created_at"`
}

// 生成公开访问链接: https://bit.local/s/{token}
// 密码保护: 访问时需输入密码
// 过期控制: 时间过期或访问次数用完自动失效
```

#### v2.0：SSO 单点登录

```go
// internal/foundation/identity/sso.go

type SSOProvider struct {
    Type         string `json:"type"`          // oidc | saml
    ClientID     string `json:"client_id"`
    ClientSecret string `json:"client_secret"` // 存入 vault
    IssuerURL    string `json:"issuer_url"`    // OIDC discovery URL
    RedirectURI  string `json:"redirect_uri"`
}

type SSOService struct {
    vault    *VaultService
    identity *IdentityService
}

func (s *SSOService) InitOIDC(ctx context.Context, provider SSOProvider) (*SSOConfig, error) {
    // 1. 从 IssuerURL 获取 .well-known/openid-configuration
    // 2. 验证配置有效性
    // 3. 存储 client_secret 到 vault
    // 4. 返回登录跳转 URL
}

func (s *SSOService) HandleCallback(ctx context.Context, code string) (*TokenPair, error) {
    // 1. 用 code 换取 id_token
    // 2. 验证 id_token 签名
    // 3. 提取用户信息 (sub, email, name)
    // 4. 查找或创建本地用户 (自动关联, 默认 member 角色)
    // 5. 生成平台 JWT
}

// SAML 类似流程，使用 crewjam/saml 库
```

#### 用户数据库 Schema

```sql
-- MVP
CREATE TABLE users (
    id         TEXT PRIMARY KEY,
    username   TEXT UNIQUE NOT NULL,
    pass_hash  TEXT NOT NULL,
    role       TEXT NOT NULL DEFAULT 'owner',
    active     BOOLEAN NOT NULL DEFAULT true,
    last_login TIMESTAMPTZ,
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- v1.0
CREATE TABLE user_app_permissions (
    user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    app_id  TEXT NOT NULL,
    access  TEXT NOT NULL DEFAULT 'use', -- use | read | admin
    PRIMARY KEY (user_id, app_id)
);

CREATE TABLE shared_links (
    id            TEXT PRIMARY KEY,
    app_id        TEXT NOT NULL,
    token         TEXT UNIQUE NOT NULL,
    password_hash TEXT,
    read_only     BOOLEAN NOT NULL DEFAULT true,
    expires_at    TIMESTAMPTZ,
    max_views     INTEGER NOT NULL DEFAULT 0,
    view_count    INTEGER NOT NULL DEFAULT 0,
    created_by    TEXT REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE invitations (
    id         TEXT PRIMARY KEY,
    email      TEXT NOT NULL,
    role       TEXT NOT NULL,
    token      TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_by TEXT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- v2.0
CREATE TABLE sso_providers (
    id            TEXT PRIMARY KEY,
    type          TEXT NOT NULL, -- oidc | saml
    client_id     TEXT NOT NULL,
    issuer_url    TEXT NOT NULL,
    enabled       BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE sso_user_mappings (
    sso_provider_id TEXT REFERENCES sso_providers(id),
    external_id     TEXT NOT NULL, -- SSO provider 的 sub/nameID
    user_id         TEXT REFERENCES users(id),
    PRIMARY KEY (sso_provider_id, external_id)
);
```

---

## 3 网络中心

### 3.1 Caddy 反向代理 (MVP)

```go
// internal/foundation/network/caddy.go

type CaddyManager struct {
    adminURL string // http://localhost:2019
}

func (c *CaddyManager) AddRoute(ctx context.Context, domain, upstream string) error {
    // POST /config/apps/http/servers/srv0/routes
    // 配置: domain → upstream (如 bank.bit.local → app-bank:3000)
    // 自动 HTTPS: 公网域名用 Let's Encrypt; 局域网用 mkcert 自签 CA
}

func (c *CaddyManager) RemoveRoute(ctx context.Context, domain string) error
func (c *CaddyManager) ListRoutes(ctx context.Context) ([]Route, error)
```

默认路由表：

| 域名 | 后端 | 阶段 |
|------|------|------|
| `console.bit.local` | `bitengined:9000` (前端静态文件) | MVP |
| `api.bit.local` | `bitengined:9000` (API) | MVP |
| `{app-slug}.bit.local` | `app-{slug}:3000` (动态) | MVP |
| `data.bit.local` | `bitengined:9000/data` | v1.0 |
| `iot.bit.local` | `bitengined:9000/iot` | v1.0 |

### 3.2 远程访问 (v1.0)

```go
// internal/foundation/network/tailscale.go

type TailscaleManager struct {
    socketPath string // /var/run/tailscaled.sock
}

type TailscaleStatus struct {
    Connected bool     `json:"connected"`
    IP        string   `json:"ip"`           // Tailscale IP
    Hostname  string   `json:"hostname"`
    Peers     []string `json:"peers"`
    ExpiresAt time.Time `json:"expires_at"`
}

func (t *TailscaleManager) Connect(ctx context.Context, authKey string) error {
    // 1. 启动 tailscaled 守护进程
    // 2. tailscale up --authkey=<key> --hostname=bitengine-<uuid>
    // 3. 配置 split tunnel: 只路由 BitEngine 流量
    // 4. 注册 MagicDNS: bitengine.tail-xxx.ts.net
}

func (t *TailscaleManager) Disconnect(ctx context.Context) error
func (t *TailscaleManager) GetStatus(ctx context.Context) (*TailscaleStatus, error)

// 备选: Cloudflare Tunnel
type CFTunnelManager struct {
    tunnelToken string
}

func (cf *CFTunnelManager) Connect(ctx context.Context, token string) error {
    // cloudflared tunnel run --token <token>
}
```

### 3.3 DNS 管理 (v1.0)

```go
// internal/foundation/network/dns.go

type DNSManager struct {
    caddy *CaddyManager
}

func (d *DNSManager) SetLocalDomain(domain string) error {
    // 使用 Avahi 发布 .local 域名
}

func (d *DNSManager) SetCustomDomain(ctx context.Context, domain string) error {
    // 1. 验证 DNS 指向正确 (查询 A/CNAME 记录)
    // 2. 在 Caddy 中配置自定义域名
    // 3. 自动获取 Let's Encrypt 证书
}
```

### 3.4 防火墙 (v1.0)

```go
// internal/foundation/firewall/nftables.go

type FirewallRule struct {
    ID          string `json:"id"`
    Direction   string `json:"direction"` // inbound | outbound
    Protocol    string `json:"protocol"`  // tcp | udp
    Port        int    `json:"port"`
    Source      string `json:"source"`    // CIDR 或 any
    Action      string `json:"action"`    // allow | deny
    Description string `json:"description"`
}

type FirewallManager struct {
    conn *nftables.Conn
}

func (f *FirewallManager) Apply(rules []FirewallRule) error {
    // 1. 清除旧规则
    // 2. 设置默认策略: deny_all_inbound
    // 3. 允许必要端口: 80, 443, 1883(如果 MQTT 启用)
    // 4. 应用自定义规则
    // 5. 启用请求频率限制: 300 req/60s per IP
}

type RateLimiter struct {
    window     time.Duration // 60s
    maxRequests int          // 300
    store      sync.Map      // IP → counter
}

func (r *RateLimiter) Allow(ip string) bool
```

### 3.5 Anti-DDoS (v2.0)

```go
// internal/foundation/firewall/antiddos.go

type AntiDDoSConfig struct {
    SmartShield     bool   `json:"smart_shield"`      // 自适应防护
    CaptchaFallback bool   `json:"captcha_fallback"`  // 异常流量验证码
    GeoBlocking     []string `json:"geo_blocking"`    // 封禁国家代码
    Threshold       int    `json:"threshold"`          // 触发阈值 (req/s)
}

type SmartShield struct {
    config    AntiDDoSConfig
    baseline  *TrafficBaseline // 学习正常流量模式
    anomaly   *AnomalyDetector
}

func (s *SmartShield) Analyze(r *http.Request) Action {
    // 1. 计算当前流量指纹 (IP分布、请求频率、User-Agent分布)
    // 2. 与 baseline 对比
    // 3. 异常 → 触发验证码 / 临时封禁
    // 4. 严重异常 → 启用 Geo-blocking
}
```

---

## 4 备份与恢复

### 4.1 定时备份 (MVP)

```go
// internal/foundation/backup/service.go

type BackupScope struct {
    AppData       bool `json:"app_data"`        // 应用数据卷
    AppConfigs    bool `json:"app_configs"`      // 应用配置
    KnowledgeBase bool `json:"knowledge_base"`   // v1.0: RAG 向量数据
    IoTRules      bool `json:"iot_rules"`         // v1.0: IoT 规则
    IoTDevices    bool `json:"iot_devices"`       // v1.0: 设备注册信息
    PlatformConfig bool `json:"platform_config"` // 平台配置
    AuditChain    bool `json:"audit_chain"`      // 审计链
}

type Backup struct {
    ID          string      `json:"id" db:"id"`
    Type        string      `json:"type" db:"type"` // daily | weekly | monthly | manual
    Scope       BackupScope `json:"scope" db:"scope"`
    Size        int64       `json:"size" db:"size"`
    Checksum    string      `json:"checksum" db:"checksum"`
    EncKey      string      `json:"-" db:"enc_key"` // AES-256-GCM 密钥 (加密存储)
    FilePath    string      `json:"file_path" db:"file_path"`
    Status      string      `json:"status" db:"status"`
    OffsiteSynced bool      `json:"offsite_synced" db:"offsite_synced"` // v2.0
    StartedAt   time.Time   `json:"started_at" db:"started_at"`
    CompletedAt *time.Time  `json:"completed_at" db:"completed_at"`
}
```

```mermaid
sequenceDiagram
    participant Cron as Cron (每天 03:00)
    participant BS as BackupService
    participant PG as PostgreSQL
    participant FS as 文件系统
    participant Enc as AES-256-GCM
    participant Offsite as 异地目标 (v2.0)
    
    Cron->>BS: TriggerBackup(daily)
    BS->>PG: pg_dump --format=custom
    BS->>FS: tar 应用数据卷 + 配置
    BS->>FS: tar 向量数据 (v1.0)
    BS->>FS: tar IoT 数据 (v1.0)
    BS->>FS: 复制审计链 SQLite
    BS->>Enc: zstd 压缩 → AES-256-GCM 加密
    Enc->>FS: 保存 backup-{date}.tar.zst.enc
    BS->>BS: 清理过期备份 (7日/4周/3月)
    BS->>Offsite: SyncOffsite (v2.0)
```

保留策略：7 个每日 + 4 个每周 + 3 个每月。

### 4.2 应用快照 (MVP)

```go
type Snapshot struct {
    ID        string    `json:"id" db:"id"`
    AppID     string    `json:"app_id" db:"app_id"`
    Version   int       `json:"version" db:"version"`
    ImageTag  string    `json:"image_tag" db:"image_tag"`
    DataPath  string    `json:"data_path" db:"data_path"`
    Trigger   string    `json:"trigger" db:"trigger"` // before_update | manual
    CreatedAt time.Time `json:"created_at" db:"created_at"`
}

func (s *BackupService) CreateSnapshot(ctx context.Context, appID string) (*Snapshot, error) {
    // 1. docker commit 保存容器镜像
    // 2. tar 应用数据卷
    // 3. 记录当前 version 和配置
}

func (s *BackupService) Rollback(ctx context.Context, snapshotID string) error {
    // 1. 停止当前容器
    // 2. 从快照恢复镜像和数据卷
    // 3. 启动恢复后的容器
    // 4. 更新 Caddy 路由
}
```

### 4.3 异地备份 (v2.0)

```go
// internal/foundation/backup/offsite.go

type OffsiteTarget struct {
    Type     string `json:"type"`     // s3 | bitengine | nas
    Endpoint string `json:"endpoint"` // S3 URL / 另一台 BitEngine IP / NAS 路径
    Bucket   string `json:"bucket,omitempty"`
    AuthKey  string `json:"auth_key"` // 存入 vault
    Schedule string `json:"schedule"` // cron 表达式
}

type OffsiteService struct {
    vault   *VaultService
    targets []OffsiteTarget
}

func (o *OffsiteService) SyncToS3(ctx context.Context, backup *Backup, target OffsiteTarget) error {
    // 使用 minio-go SDK 上传加密备份到 S3 兼容存储
}

func (o *OffsiteService) SyncToBitEngine(ctx context.Context, backup *Backup, target OffsiteTarget) error {
    // 通过 A2A 协议 (v2.0) 将备份发送到另一台 BitEngine
    // HMAC-SHA256 双向认证
}

func (o *OffsiteService) SyncToNAS(ctx context.Context, backup *Backup, target OffsiteTarget) error {
    // 挂载 NFS/SMB 共享 → 复制加密备份文件
}
```

### 4.4 数据库 Schema

```sql
CREATE TABLE backups (
    id            TEXT PRIMARY KEY,
    type          TEXT NOT NULL,
    scope         JSONB NOT NULL,
    size          BIGINT,
    checksum      TEXT,
    enc_key       TEXT,
    file_path     TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending',
    offsite_synced BOOLEAN DEFAULT false,
    started_at    TIMESTAMPTZ NOT NULL,
    completed_at  TIMESTAMPTZ
);

CREATE TABLE snapshots (
    id         TEXT PRIMARY KEY,
    app_id     TEXT NOT NULL,
    version    INTEGER NOT NULL,
    image_tag  TEXT NOT NULL,
    data_path  TEXT NOT NULL,
    trigger    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- v2.0
CREATE TABLE offsite_targets (
    id        TEXT PRIMARY KEY,
    type      TEXT NOT NULL,
    endpoint  TEXT NOT NULL,
    bucket    TEXT,
    schedule  TEXT,
    enabled   BOOLEAN DEFAULT true,
    last_sync TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## 5 系统监控

### 5.1 资源采集 (MVP)

```go
// internal/foundation/monitor/collector.go

type SystemMetrics struct {
    Timestamp    time.Time        `json:"timestamp"`
    CPU          float64          `json:"cpu_percent"`
    Memory       MemoryInfo       `json:"memory"`
    Disk         DiskInfo         `json:"disk"`
    NetworkIO    NetworkIO        `json:"network_io"`
    Containers   []ContainerStats `json:"containers"`
    AIModels     []ModelStatus    `json:"ai_models"`
    IoTDevices   *IoTDeviceCount  `json:"iot_devices,omitempty"` // v1.0
}

type MetricsCollector struct {
    dockerClient *docker.Client
    interval     time.Duration // 5s
}

func (c *MetricsCollector) Collect(ctx context.Context) (*SystemMetrics, error) {
    // 并发采集: /proc/stat, /proc/meminfo, docker stats API, ollama ps
}
```

### 5.2 健康检查 (MVP)

```go
type HealthCheck struct {
    Type      string `json:"type"`       // http_ping | container_running | memory_threshold
    Target    string `json:"target"`     // 容器ID或URL
    Interval  time.Duration `json:"interval"` // 30s
    Timeout   time.Duration `json:"timeout"`  // 5s
    Threshold int    `json:"threshold"`  // 连续失败次数
}

type HealthChecker struct {
    checks      map[string]*HealthCheck
    autoRestart AutoRestartConfig
}

type AutoRestartConfig struct {
    Enabled    bool          `json:"enabled"`
    MaxRetries int           `json:"max_retries"` // 3
    Cooldown   time.Duration `json:"cooldown"`    // 300s
}

func (h *HealthChecker) CheckApp(ctx context.Context, appID string) (*HealthStatus, error) {
    // 1. HTTP GET {app-domain}/health → 200 OK
    // 2. docker inspect → running
    // 3. docker stats → memory < threshold
    // 失败 → 自动重启 (最多 MaxRetries 次, 间隔 Cooldown)
}
```

### 5.3 告警规则 (MVP)

```go
type AlertRule struct {
    ID        string `json:"id" db:"id"`
    Condition string `json:"condition" db:"condition"` // 表达式
    Severity  string `json:"severity" db:"severity"`   // info | warning | critical
    Message   string `json:"message" db:"message"`
    Channel   string `json:"channel" db:"channel"`     // a2h | log
    Enabled   bool   `json:"enabled" db:"enabled"`
}

// 内置告警规则
var DefaultAlerts = []AlertRule{
    {Condition: "disk_usage > 85%", Severity: "warning", Message: "磁盘空间不足"},
    {Condition: "memory_usage > 90%", Severity: "critical", Message: "内存即将耗尽"},
    {Condition: "app_health_check_failed", Severity: "warning", Message: "应用 {app_name} 健康检查失败"},
    {Condition: "backup_failed", Severity: "critical", Message: "自动备份失败"},
    // v1.0 新增
    {Condition: "iot_device_offline > 5min", Severity: "info", Message: "设备 {device_name} 已离线"},
    {Condition: "data_pipeline_failed", Severity: "warning", Message: "数据管道 {pipeline_name} 执行失败"},
}
```

---

## 6 数据库服务 (MVP)

```go
// internal/foundation/database/manager.go

type DBManager struct {
    pool *pgxpool.Pool
}

// 预定义 Schema
var PredefinedSchemas = []string{
    "bitengine", // 平台核心（含 mcp_servers 表，v7 新增）
    "apps",      // 应用元数据
    "datalake",  // v1.0: 数据湖
    "iot",       // v1.0: IoT
}

func (m *DBManager) CreateAppSchema(ctx context.Context, appID string) error {
    schema := fmt.Sprintf("app_%s", sanitize(appID))
    // CREATE SCHEMA IF NOT EXISTS <schema>
    // GRANT ALL ON SCHEMA <schema> TO app_user
}

func (m *DBManager) DropAppSchema(ctx context.Context, appID string) error {
    // DROP SCHEMA IF EXISTS <schema> CASCADE
}

// 为每个应用容器生成独立的 DATABASE_URL
func (m *DBManager) GetAppDatabaseURL(appID string) string {
    return fmt.Sprintf("postgres://app_%s:***@postgresql:5432/bitengine?search_path=app_%s", appID, appID)
}
```

**Redis 角色收窄（v7 变更）**

Redis 从"平台事件总线"收窄为"内部服务通信和缓存"。IoT 设备事件改走 MQTT 5.0 标准协议（DD-05）。

| 用途 | 修改前 | 修改后 |
|------|--------|--------|
| IoT 设备事件传输 | Redis Pub/Sub | **MQTT 5.0**（DD-05 数据面） |
| 平台内部服务通信 | Redis Pub/Sub | Redis Pub/Sub（保留） |
| 会话缓存 | Redis | Redis（不变） |
| 任务队列（媒体处理等） | Redis | Redis（不变） |
| Elicitation 临时上下文 | — | Redis（v7 新增，DD-07） |

---

## 6.5 MQTT 5.0 Broker 基础设施（v7 新增）

内嵌 Mosquitto Broker 作为平台 IoT 数据面的核心传输组件。

```go
// internal/foundation/mqtt/broker.go

type MQTTBrokerManager struct {
    configPath string // /etc/mosquitto/mosquitto.conf
    process    *os.Process
}

// 随平台启动
func (m *MQTTBrokerManager) Start(ctx context.Context) error {
    // 生成 Mosquitto 配置
    config := MosquittoConfig{
        ProtocolVersion: 5,             // 必须 MQTT 5.0
        Listeners: []Listener{
            {Port: 1883, Protocol: "mqtt"},   // 内部通信
            {Port: 8883, Protocol: "mqtt", TLS: true}, // 外部设备 TLS
            {Port: 9001, Protocol: "websockets"},       // 前端直连
        },
        Persistence: true,
        ACLFile:     "/etc/mosquitto/acl",
    }
    
    writeConfig(m.configPath, config)
    return m.startProcess(ctx)
}
```

**必须使用 MQTT 5.0 而非 3.1.1**，六个关键特性价值：

| MQTT 5.0 特性 | 对 BitEngine 的价值 |
|--------------|-------------------|
| User Properties | 消息附加元数据（device_id、provider、priority），媒体管道标记 MIME 类型 |
| Shared Subscriptions | 规则引擎多实例负载均衡，DD-09 HA 场景必需 |
| Message Expiry | 设备事件自动过期清理，避免处理过时遥测数据 |
| Request/Response | MCP ↔ MQTT Bridge 中控制面操作的标准请求-响应模式 |
| Content Type | 标记 payload 格式（JSON/Protobuf/媒体流），入库管道自动识别 |
| Reason Code | 设备断连原因上报，改善诊断和自愈 |

**资源占用**：Mosquitto 单进程，空闲 ~5MB 内存，对 32GB 设备无感。

---

## 6.6 MCP Server Registry（v7 新增）

统一管理和发现平台内所有 MCP Server。DD-07 的 Device Aggregator 通过 Registry 发现设备提供者，DD-05 的各 Provider 在此注册。

```go
// internal/foundation/mcp_registry/registry.go

type MCPServerRegistry struct {
    pool *pgxpool.Pool
}

type MCPServerEntry struct {
    ID          string            `json:"id"`        // "mqtt-direct" | "homeassistant" | "edgex"
    Type        string            `json:"type"`      // "device-provider" | "app-manager" | "data-rag" | "system"
    Endpoint    string            `json:"endpoint"`  // unix:// 或 http://
    Protocols   []string          `json:"protocols"` // 设备提供者的协议类型
    Config      map[string]string `json:"config"`    // 连接配置
    Status      string            `json:"status"`    // "active" | "disconnected" | "error"
    AutoDiscover bool             `json:"auto_discover"`
    RegisteredAt time.Time        `json:"registered_at"`
}

func (r *MCPServerRegistry) Register(ctx context.Context, entry MCPServerEntry) error {
    _, err := r.pool.Exec(ctx,
        `INSERT INTO bitengine.mcp_servers (id, type, endpoint, protocols, config, status, auto_discover, registered_at)
         VALUES ($1, $2, $3, $4, $5, 'active', $6, now())
         ON CONFLICT (id) DO UPDATE SET endpoint=$3, status='active'`,
        entry.ID, entry.Type, entry.Endpoint, entry.Protocols, entry.Config, entry.AutoDiscover)
    return err
}

func (r *MCPServerRegistry) ListByType(ctx context.Context, serverType string) ([]MCPServerEntry, error) {
    rows, err := r.pool.Query(ctx,
        `SELECT id, type, endpoint, protocols, config, status FROM bitengine.mcp_servers WHERE type=$1 AND status='active'`,
        serverType)
    // ...parse rows...
    return entries, err
}
```

```sql
-- MCP Server Registry 表
CREATE TABLE bitengine.mcp_servers (
    id             VARCHAR(100) PRIMARY KEY,
    type           VARCHAR(50) NOT NULL,     -- device-provider | app-manager | data-rag | system
    endpoint       TEXT NOT NULL,
    protocols      TEXT[],
    config         JSONB,
    status         VARCHAR(20) NOT NULL DEFAULT 'active',
    auto_discover  BOOLEAN DEFAULT false,
    registered_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**预注册的内置 MCP Server**：

| ID | Type | Endpoint | 说明 |
|----|------|----------|------|
| `app-manager` | app-manager | `unix:///var/run/bitengine/mcp-app.sock` | 应用生命周期管理 |
| `data-rag` | data-rag | `unix:///var/run/bitengine/mcp-data.sock` | 数据查询 + RAG |
| `system` | system | `unix:///var/run/bitengine/mcp-system.sock` | 系统管理 |
| `mqtt-direct` | device-provider | `unix:///var/run/bitengine/mcp-mqtt.sock` | MQTT/HTTP 设备直连 |

---

## 6.7 媒体文件存储策略（v7 新增）

视频/音频文件体积大（单个视频数百 MB），不适合存入数据库。新增文件系统存储层。

```go
// internal/foundation/media/storage.go

type MediaStorage struct {
    basePath    string // /data/bitengine/media/
    maxSizeGB   int    // 可配置存储上限
}

// 存储路径：{year}/{month}/{hash_prefix}/{full_hash}.{ext}
func (s *MediaStorage) Store(ctx context.Context, reader io.Reader, filename string) (*MediaAsset, error) {
    hash := sha256.New()
    tmpFile, _ := os.CreateTemp(s.basePath, "upload-*")
    
    // 边读边计算哈希
    tee := io.TeeReader(reader, hash)
    size, _ := io.Copy(tmpFile, tee)
    
    hashStr := hex.EncodeToString(hash.Sum(nil))
    now := time.Now()
    destPath := fmt.Sprintf("%s/%d/%02d/%s/%s%s",
        s.basePath, now.Year(), now.Month(),
        hashStr[:4], hashStr, filepath.Ext(filename))
    
    os.MkdirAll(filepath.Dir(destPath), 0755)
    os.Rename(tmpFile.Name(), destPath)
    
    return &MediaAsset{
        ID:       gen_ulid(),
        Path:     destPath,
        Size:     size,
        Hash:     hashStr,
        MimeType: detectMIME(filename),
    }, nil
}
```

```yaml
storage:
  media:
    path: /data/bitengine/media/
    structure: "{year}/{month}/{hash}"
    max_size: 50GB                     # 可配置
  metadata:
    store: postgresql                  # 元数据和衍生内容入库
```

媒体入库管道（DD-04）是 GPU 密集任务，复用 Redis 任务队列做异步处理，支持优先级和中断恢复。

---

## 7 Merkle 审计链 (MVP)

```go
// internal/foundation/audit/chain.go

type AuditChain struct {
    db *sql.DB // SQLite, append-only
}

func (c *AuditChain) Append(ctx context.Context, entry AuditEntry) error {
    // 1. 获取上一条记录的 Hash
    // 2. entry.PrevHash = lastHash
    // 3. entry.Hash = SHA256(entry.ID + entry.EventType + entry.ActorID + entry.Detail + entry.PrevHash + entry.Timestamp)
    // 4. INSERT INTO audit_chain
}

func (c *AuditChain) Verify(ctx context.Context) (bool, error) {
    // 遍历全链, 重新计算每条记录的 Hash, 与存储的 Hash 对比
    // 任何不匹配 → 链条被篡改 → 发出 A2H 告警
}
```

审计事件类型（全版本）：

| 事件类型 | 说明 | 阶段 |
|---------|------|------|
| `auth.login` | 用户登录 | MVP |
| `auth.login_failed` | 登录失败 | MVP |
| `auth.user_created` | 创建用户 | v1.0 |
| `auth.role_changed` | 角色变更 | v1.0 |
| `auth.sso_login` | SSO 登录 | v2.0 |
| `app.created` | 应用创建 | MVP |
| `app.deleted` | 应用删除 | MVP |
| `app.deployed` | 应用部署 | MVP |
| `app.iterated` | 应用迭代 | v1.0 |
| `config.changed` | 配置变更 | MVP |
| `backup.completed` | 备份完成 | MVP |
| `backup.failed` | 备份失败 | MVP |
| `backup.offsite_synced` | 异地同步 | v2.0 |
| `iot.device.connected` | 设备连接 | v1.0 |
| `iot.device.disconnected` | 设备断开 | v1.0 |
| `iot.rule.fired` | 规则触发 | v1.0 |
| `data.exported` | 数据导出 | v1.0 |
| `data.document_ingested` | 文档摄取 | v1.0 |
| `cross_center.trigger` | 跨中心联动 | v1.0 |
| `mcp.server.registered` | MCP Server 注册（v7 新增） | v1.0 |
| `mcp.server.removed` | MCP Server 注销（v7 新增） | v1.0 |
| `market.module_installed` | 模块安装 | v1.0 |
| `market.module_published` | 模块发布 | v2.0 |
| `remote.session_start` | 远程会话 | v1.0 |
| `sharing.link_created` | 共享链接创建 | v1.0 |
| `sharing.link_accessed` | 共享链接访问 | v1.0 |

---

## 8 密钥保险库 (MVP)

```go
// internal/foundation/vault/service.go

type VaultService struct {
    db        *sql.DB // 独立 SQLite
    masterKey []byte  // 从环境变量或 TPM 获取
}

func (v *VaultService) Store(ctx context.Context, key string, value string) error {
    // 1. AES-256-GCM 加密 value
    // 2. INSERT INTO vault (key, encrypted_value, nonce)
}

func (v *VaultService) Retrieve(ctx context.Context, key string) (string, error) {
    // 1. SELECT encrypted_value, nonce FROM vault WHERE key = ?
    // 2. AES-256-GCM 解密
    // 3. 返回明文值
    // 4. 调用方使用完毕后应调用 Zeroize()
}

func Zeroize(data []byte) {
    for i := range data {
        data[i] = 0
    }
    for i := range data {
        data[i] = 0xFF
    }
    for i := range data {
        data[i] = 0
    }
    // 3 次覆写: 0x00 → 0xFF → 0x00
}
```

---

## 9 污点追踪 (v1.0)

```go
// internal/foundation/taint/tracker.go

type TaintLabel string

const (
    TaintAPIKey       TaintLabel = "api_key"
    TaintCredential   TaintLabel = "credential"
    TaintFinancial    TaintLabel = "financial"
    TaintPII          TaintLabel = "pii"
)

type TaintPolicy struct {
    LogOutput     string // block | sanitize
    NetworkOutput string // block | sanitize
    CrossApp      string // block
    CloudAPI      string // sanitize
}

type TaintTracker struct {
    policy TaintPolicy
}

func (t *TaintTracker) Mark(data []byte, label TaintLabel) *TaintedData {
    return &TaintedData{data: data, label: label}
}

func (t *TaintTracker) SanitizeForCloud(input string) string {
    // 替换检测到的敏感数据:
    // API 密钥 → [REDACTED_API_KEY]
    // 邮箱 → u***@***.com
    // 电话 → ***-****-1234
    // 金融数据 → [REDACTED_FINANCIAL]
}

func (t *TaintTracker) CheckLogSafe(data string) bool {
    // 扫描数据中是否包含已标记的敏感模式
    // 如果包含 → 阻止写入日志 → 写入脱敏版本
}
```

---

## 10 安装脚本与首次上手 (MVP)

### 10.1 一行命令安装

```bash
# 用户执行
curl -fsSL https://bitengine.io/install.sh | bash
```

```go
// scripts/install.go (编译为 install.sh 内嵌二进制)

type Installer struct {
    minRAM      int // 16GB 最低, 32GB 推荐
    minDisk     int // 50GB 最低
}

func (i *Installer) Run() error {
    // 1. 环境检测
    fmt.Println("🔍 检测系统环境...")
    arch := runtime.GOARCH               // amd64 | arm64
    ram := getSystemRAM()                 // 系统内存
    disk := getAvailableDisk("/")         // 可用磁盘
    
    if ram < 16*GB {
        return fmt.Errorf("内存不足: %dGB (最低 16GB, 推荐 32GB)", ram/GB)
    }
    
    // 2. 安装依赖
    if !isDockerInstalled() {
        fmt.Println("📦 安装 Docker...")
        installDocker()
    }
    
    // 3. 下载 BitEngine 单二进制
    fmt.Println("⬇️  下载 BitEngine...")
    binary := downloadBinary(arch) // bitengined-linux-{arch}
    installBinary(binary, "/usr/local/bin/bitengined")
    
    // 4. 创建数据目录
    createDirectories()
    
    // 5. 拉取基础镜像
    fmt.Println("🐳 拉取平台镜像...")
    pullImages([]string{"postgres:16-alpine", "redis:7-alpine"})
    
    // 6. 初始化数据库
    fmt.Println("🗄  初始化数据库...")
    initDatabase()
    
    // 7. 启动平台
    fmt.Println("🚀 启动 BitEngine...")
    startPlatform()
    
    // 8. 输出访问信息
    ip := getLocalIP()
    fmt.Printf("\n✅ BitEngine 安装完成!\n")
    fmt.Printf("   访问地址: http://%s:8080\n", ip)
    fmt.Printf("   首次设置向导将引导你完成配置\n")
    
    return nil
}
```

### 10.2 首次设置向导 (Web UI)

```go
// internal/foundation/setup/wizard.go

type SetupWizard struct {
    store  *SetupStore
    vault  *Vault
}

type SetupState struct {
    Step       int    `json:"step"`       // 当前步骤 1-5
    Completed  bool   `json:"completed"`
}

// 向导步骤
// Step 1: 创建管理员账户 (用户名 + 密码 + 可选 Passkey)
// Step 2: 配置云端 AI (选择 Provider + 输入 API Key → 存入 Vault)
// Step 3: 网络设置 (自定义域名 / Tailscale / 跳过)
// Step 4: 通知渠道 (微信 / Telegram / 邮件 / 跳过)
// Step 5: 体验引导 (创建第一个应用 / 浏览模板)

func (w *SetupWizard) CompleteStep(ctx context.Context, step int, data map[string]interface{}) error {
    switch step {
    case 1: // 创建管理员
        return w.createOwner(ctx, data["username"].(string), data["password"].(string))
    case 2: // 配置 AI
        provider := data["provider"].(string)
        apiKey := data["api_key"].(string)
        return w.vault.StoreSecret(ctx, "cloud_ai_"+provider, []byte(apiKey))
    case 3: // 网络
        if domain, ok := data["domain"]; ok {
            return w.configureCustomDomain(ctx, domain.(string))
        }
    case 4: // 通知
        return w.configureNotification(ctx, data)
    case 5: // 完成
        return w.store.MarkComplete(ctx)
    }
    return nil
}

// 中间件: 未完成设置时强制跳转向导
func SetupGuard() gin.HandlerFunc {
    return func(c *gin.Context) {
        if !isSetupComplete() && !strings.HasPrefix(c.Request.URL.Path, "/api/v1/setup") {
            c.JSON(http.StatusPreconditionRequired, gin.H{"redirect": "/setup"})
            c.Abort()
            return
        }
        c.Next()
    }
}
```

---

## 11 应用共享与公开链接 (v1.0)

```go
// internal/foundation/sharing/sharing.go

type SharedLink struct {
    ID              string     `json:"id"`
    AppID           string     `json:"app_id"`
    Token           string     `json:"token"`           // 随机 URL token
    PasswordHash    string     `json:"-"`               // bcrypt, 可选
    ExpiresAt       *time.Time `json:"expires_at"`      // 可选过期时间
    Permissions     string     `json:"permissions"`     // read_only | read_write
    CreatedBy       string     `json:"created_by"`
    CreatedAt       time.Time  `json:"created_at"`
}

type SharingService struct {
    pool *pgxpool.Pool
}

func (s *SharingService) CreateLink(ctx context.Context, req CreateLinkRequest) (*SharedLink, error) {
    link := &SharedLink{
        ID:          genULID(),
        AppID:       req.AppID,
        Token:       generateSecureToken(32),
        Permissions: req.Permissions,
        ExpiresAt:   req.ExpiresAt,
        CreatedBy:   req.UserID,
    }
    
    if req.Password != "" {
        link.PasswordHash, _ = bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
    }
    
    _, err := s.pool.Exec(ctx,
        `INSERT INTO foundation.shared_links (id, app_id, token, password_hash, expires_at, permissions, created_by)
         VALUES ($1, $2, $3, $4, $5, $6, $7)`,
        link.ID, link.AppID, link.Token, link.PasswordHash, link.ExpiresAt, link.Permissions, link.CreatedBy)
    
    return link, err
}

// Caddy 动态路由: /shared/{token} → 应用容器 (只读代理)
func (s *SharingService) ValidateAccess(ctx context.Context, token, password string) (*SharedLink, error) {
    var link SharedLink
    err := s.pool.QueryRow(ctx,
        `SELECT id, app_id, token, password_hash, expires_at, permissions FROM foundation.shared_links WHERE token=$1`, token).
        Scan(&link.ID, &link.AppID, &link.Token, &link.PasswordHash, &link.ExpiresAt, &link.Permissions)
    if err != nil {
        return nil, ErrShareLinkNotFound
    }
    
    // 检查过期
    if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
        return nil, ErrShareLinkExpired
    }
    
    // 检查密码
    if link.PasswordHash != "" {
        if err := bcrypt.CompareHashAndPassword([]byte(link.PasswordHash), []byte(password)); err != nil {
            return nil, ErrSharePasswordInvalid
        }
    }
    
    return &link, nil
}
```

```sql
-- foundation schema 补充
CREATE TABLE foundation.shared_links (
    id             TEXT PRIMARY KEY DEFAULT gen_ulid(),
    app_id         TEXT NOT NULL,
    token          VARCHAR(64) NOT NULL UNIQUE,
    password_hash  TEXT,
    expires_at     TIMESTAMPTZ,
    permissions    VARCHAR(20) NOT NULL DEFAULT 'read_only',
    created_by     TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_shared_token ON foundation.shared_links(token);
```

---

## 12 API 端点

### 12.1 设置向导 API

| 方法 | 端点 | 说明 | 阶段 |
|------|------|------|------|
| GET | `/api/v1/setup/status` | 向导状态 (step/completed) | MVP |
| POST | `/api/v1/setup/step/:n` | 完成向导步骤 | MVP |
| POST | `/api/v1/setup/skip` | 跳过可选步骤 | MVP |

### 12.2 认证 API

| 方法 | 端点 | 说明 | 阶段 |
|------|------|------|------|
| POST | `/api/v1/auth/login` | 登录获取 JWT | MVP |
| POST | `/api/v1/auth/refresh` | 刷新 token | MVP |
| POST | `/api/v1/auth/logout` | 注销 | MVP |
| GET | `/api/v1/auth/users` | 用户列表 | v1.0 |
| POST | `/api/v1/auth/users` | 创建用户 | v1.0 |
| PUT | `/api/v1/auth/users/:id/role` | 修改角色 | v1.0 |
| DELETE | `/api/v1/auth/users/:id` | 删除用户 | v1.0 |
| POST | `/api/v1/auth/invite` | 生成邀请链接 | v1.0 |
| POST | `/api/v1/sharing/links` | 创建共享链接 | v1.0 |
| DELETE | `/api/v1/sharing/links/:id` | 撤销共享链接 | v1.0 |
| GET | `/api/v1/auth/sso/providers` | SSO 提供商列表 | v2.0 |
| POST | `/api/v1/auth/sso/init` | 初始化 SSO | v2.0 |
| GET | `/api/v1/auth/sso/callback` | SSO 回调 | v2.0 |

### 12.3 共享 API

| 方法 | 端点 | 说明 | 阶段 |
|------|------|------|------|
| POST | `/api/v1/sharing/links` | 创建共享链接 | v1.0 |
| GET | `/api/v1/sharing/links` | 应用的共享链接列表 | v1.0 |
| DELETE | `/api/v1/sharing/links/:id` | 撤销共享链接 | v1.0 |
| GET | `/shared/:token` | 访问共享应用 | v1.0 |
| POST | `/shared/:token/auth` | 密码验证共享链接 | v1.0 |

### 12.4 系统 API

| 方法 | 端点 | 说明 | 阶段 |
|------|------|------|------|
| GET | `/api/v1/system/status` | 系统状态 | MVP |
| GET | `/api/v1/system/metrics` | 实时指标 | MVP |
| POST | `/api/v1/system/backup` | 手动备份 | MVP |
| GET | `/api/v1/system/backups` | 备份列表 | MVP |
| POST | `/api/v1/system/restore/:id` | 恢复 | MVP |
| GET | `/api/v1/system/alerts` | 告警列表 | MVP |
| PUT | `/api/v1/system/alerts/:id` | 修改告警规则 | MVP |
| GET | `/api/v1/system/audit` | 审计日志查询 | MVP |
| POST | `/api/v1/system/audit/export` | 导出审计 | MVP |
| GET | `/api/v1/remote/status` | 远程访问状态 | v1.0 |
| POST | `/api/v1/remote/connect` | 连接 Tailscale | v1.0 |
| POST | `/api/v1/remote/disconnect` | 断开 | v1.0 |
| GET | `/api/v1/system/firewall` | 防火墙规则 | v1.0 |
| PUT | `/api/v1/system/firewall` | 更新防火墙 | v1.0 |
| GET | `/api/v1/system/offsite` | 异地备份目标 | v2.0 |
| POST | `/api/v1/system/offsite` | 添加异地目标 | v2.0 |
| POST | `/api/v1/system/offsite/sync` | 手动同步 | v2.0 |

### 12.5 MCP Server Registry API（v7 新增）

| 方法 | 端点 | 说明 | 阶段 |
|------|------|------|------|
| GET | `/api/v1/mcp/registry` | MCP Server 清单 | v1.0 |
| POST | `/api/v1/mcp/registry` | 注册 MCP Server（手动添加 HA/EdgeX 等外部提供者） | v1.0 |
| PUT | `/api/v1/mcp/registry/:id` | 更新 MCP Server 配置 | v1.0 |
| DELETE | `/api/v1/mcp/registry/:id` | 注销 MCP Server | v1.0 |
| GET | `/api/v1/mcp/registry/:id/health` | MCP Server 健康状态 | v1.0 |

### 12.6 MQTT Broker API（v7 新增）

| 方法 | 端点 | 说明 | 阶段 |
|------|------|------|------|
| GET | `/api/v1/mqtt/status` | MQTT Broker 状态（连接数、消息吞吐） | v1.0 |
| GET | `/api/v1/mqtt/clients` | 已连接 MQTT 客户端列表 | v1.0 |

---

## 13 错误码

| 错误码 | 说明 | 阶段 |
|--------|------|------|
| `AUTH_INVALID_TOKEN` | JWT 无效或过期 | MVP |
| `AUTH_INVALID_CREDENTIALS` | 用户名/密码错误 | MVP |
| `AUTH_FORBIDDEN` | 权限不足 | MVP |
| `AUTH_USER_EXISTS` | 用户名已存在 | v1.0 |
| `AUTH_INVITE_EXPIRED` | 邀请链接已过期 | v1.0 |
| `AUTH_SSO_CONFIG_INVALID` | SSO 配置无效 | v2.0 |
| `BACKUP_FAILED` | 备份执行失败 | MVP |
| `BACKUP_RESTORE_FAILED` | 恢复失败 | MVP |
| `BACKUP_OFFSITE_UNREACHABLE` | 异地目标不可达 | v2.0 |
| `MONITOR_THRESHOLD_EXCEEDED` | 资源超限 | MVP |
| `AUDIT_CHAIN_BROKEN` | 审计链完整性校验失败 | MVP |
| `NETWORK_TAILSCALE_FAILED` | Tailscale 连接失败 | v1.0 |
| `NETWORK_DOMAIN_INVALID` | 自定义域名验证失败 | v1.0 |
| `SHARE_LINK_EXPIRED` | 共享链接已过期 | v1.0 |
| `SHARE_LINK_PASSWORD_WRONG` | 共享链接密码错误 | v1.0 |
| `FIREWALL_RULE_CONFLICT` | 防火墙规则冲突 | v1.0 |
| `MQTT_BROKER_START_FAILED` | MQTT 5.0 Broker 启动失败（v7 新增） | v1.0 |
| `MQTT_BROKER_UNAVAILABLE` | MQTT 5.0 Broker 不可用（v7 新增） | v1.0 |
| `MCP_REGISTRY_ENTRY_NOT_FOUND` | MCP Server 注册条目不存在（v7 新增） | v1.0 |
| `MCP_REGISTRY_DUPLICATE` | MCP Server ID 已存在（v7 新增） | v1.0 |
| `MEDIA_STORAGE_FULL` | 媒体存储空间已满（v7 新增） | v1.0 |
| `SETUP_NOT_COMPLETE` | 平台未完成初始设置 | MVP |
| `SETUP_STEP_INVALID` | 向导步骤无效 | MVP |

---

## 14 测试策略

| 类型 | 覆盖 | 工具 |
|------|------|------|
| 单元测试 | JWT 签发/验证、RBAC 权限矩阵、备份压缩/加密、Merkle 哈希链、污点正则匹配 | `testing` + `testify` |
| 集成测试 | Caddy 路由添加/删除、PostgreSQL Schema 创建/隔离、备份→恢复全链路 | `testcontainers-go` |
| 集成测试 | MQTT 5.0 Broker 启动 → 客户端连接 → 发布/订阅 → User Properties 传递（v7 新增） | `testcontainers-go` (Mosquitto 2.0+) |
| 集成测试 | MCP Server Registry 注册 → 查询 → 健康检查（v7 新增） | `testcontainers-go` (PostgreSQL) |
| 集成测试 | 媒体文件存储 → 哈希去重 → 存储上限检查（v7 新增） | 文件系统测试 |
| 安全测试 | JWT 篡改检测、SQL 注入防护、密钥零化验证、防火墙规则有效性 | 自定义 |
| E2E | 登录→创建用户→分配权限→共享链接→访问验证 | Playwright |

---

# 应用运行层（原 DD-02，v7 合并入 DD-01）

> 模块路径：`internal/runtime/` | 以下 §15-27 原为独立的 DD-02 应用运行层文档，v7 合并入核心平台。内容无修改，仅重编号。

---

## 15 应用运行层模块职责

应用运行层管理所有用户应用和能力模块的生命周期，提供双层隔离（Docker + WASM）。

| 子系统 | 职责 | 阶段 |
|--------|------|------|
| docker | Docker 容器生命周期管理 | MVP |
| network | 每应用隔离网络 | MVP |
| resources | cgroups 资源配额 | MVP |
| bluegreen | 蓝绿部署与回滚 | v1.0 |
| wasm | WASM 沙箱 (Wazero) | v1.0 |
| cron | 定时任务调度器 | v1.0 |
| wasmai | WASM+AI (WASI-NN) 推理 | v2.0 |

---

## 16 Docker 容器生命周期 (MVP)

### 16.1 状态机

```mermaid
stateDiagram-v2
    [*] --> Created: Create()
    Created --> Building: 构建镜像
    Building --> Running: Start()
    Running --> Stopped: Stop()
    Stopped --> Running: Start()
    Running --> Updating: IterateApp() [v1.0]
    Updating --> Running: 蓝绿切换成功
    Updating --> Running: 回滚 [失败时]
    Stopped --> Removed: Remove()
    Running --> Removed: Remove()
    Removed --> [*]
```

### 16.2 DockerManager

```go
// internal/runtime/docker/manager.go

type DockerManager struct {
    client   *docker.Client
    network  *NetworkPool
    db       *DBManager
    caddy    *CaddyManager
    eventBus EventBus
}

func (d *DockerManager) Create(ctx context.Context, spec AppSpec) (*AppInstance, error) {
    // 1. 构建 Docker 镜像 (ImageBuilder.Build)
    // 2. 创建隔离网络 (NetworkPool.CreateAppNetwork)
    // 3. 创建应用数据库 Schema (DBManager.CreateAppSchema)
    // 4. 创建并启动容器
    // 5. 注册 Caddy 路由 ({slug}.bit.local → container:3000)
    // 6. 发布事件 app.status.started
    // 7. 返回 AppInstance
}

func (d *DockerManager) Start(ctx context.Context, appID string) error {
    // docker start + 注册 Caddy 路由 + 发布事件
}

func (d *DockerManager) Stop(ctx context.Context, appID string) error {
    // 移除 Caddy 路由 + docker stop + 发布事件
}

func (d *DockerManager) Remove(ctx context.Context, appID string) error {
    // 1. Stop (如果运行中)
    // 2. 删除容器
    // 3. 删除隔离网络
    // 4. 删除数据库 Schema
    // 5. 删除数据卷
    // 6. 发布事件 app.status.removed
}

func (d *DockerManager) Logs(ctx context.Context, appID string, opts LogOptions) (io.ReadCloser, error) {
    // docker logs --follow --tail={opts.Tail} --since={opts.Since}
}

func (d *DockerManager) GetStatus(ctx context.Context, appID string) (*AppStatus, error) {
    // docker inspect → 提取 State, Health, Resource usage
}
```

### 16.3 镜像构建

```go
// internal/runtime/docker/builder.go

type ImageBuilder struct {
    client *docker.Client
}

// 默认 Dockerfile 模板 (AI 生成的 Python 应用)
const DefaultDockerfile = `
FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
USER nobody
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s CMD curl -f http://localhost:3000/health || exit 1
CMD ["python", "app.py"]
`

func (b *ImageBuilder) Build(ctx context.Context, code []byte, appSlug string) (string, error) {
    // 1. 创建临时目录，写入代码文件
    // 2. 如果没有 Dockerfile，使用 DefaultDockerfile
    // 3. docker build -t bitengine-app-{slug}:{version}
    // 4. 返回 image tag
}
```

### 16.4 容器环境变量

每个应用容器注入以下环境变量：

| 变量 | 说明 | 示例 |
|------|------|------|
| `DATABASE_URL` | 应用独立 schema 的连接串 | `postgres://...?search_path=app_xxx` |
| `BITENGINE_APP_ID` | 应用 ID | `01HZ...` |
| `BITENGINE_API_URL` | 平台 API 地址 | `http://bitengined:9000/api/v1` |
| `BITENGINE_API_TOKEN` | 内部 HMAC Token | `hmac-xxx` |
| `PORT` | 监听端口 | `3000` |
| `BITENGINE_MCP_URL` | MCP Server 端点 | `http://bitengined:9001/mcp/v1` |

---

## 17 网络隔离 (MVP)

```mermaid
graph TB
    CADDY[":443 Caddy"]
    
    subgraph NET_BANK["ef-app-bank (隔离网络)"]
        APP_BANK["app-bank :3000"]
    end
    
    subgraph NET_TODO["ef-app-todo (隔离网络)"]
        APP_TODO["app-todo :3000"]
    end
    
    subgraph INTERNAL["ef-internal"]
        PG["PostgreSQL"]
    end
    
    CADDY --> APP_BANK
    CADDY --> APP_TODO
    APP_BANK --> PG
    APP_TODO --> PG
    APP_BANK -.->|"不可访问"| APP_TODO
```

```go
// internal/runtime/network/pool.go

type NetworkPool struct {
    client *docker.Client
}

func (p *NetworkPool) CreateAppNetwork(ctx context.Context, appID string) (string, error) {
    name := fmt.Sprintf("ef-app-%s", appID)
    // docker network create --internal --driver bridge <n>
    // 连接应用容器 + postgresql 容器到此网络
    // 应用容器之间完全隔离
}

func (p *NetworkPool) RemoveAppNetwork(ctx context.Context, appID string) error

// 外部网络访问控制
type NetworkPolicy struct {
    AllowOutbound []string `json:"allow_outbound"` // 域名白名单
    DenyAll       bool     `json:"deny_all"`       // 默认 true
}
```

---

## 18 资源管控 (MVP)

```go
// internal/runtime/resources/quota.go

type ResourceQuota struct {
    CPUPercent  int   `json:"cpu_percent"`   // 默认 25, 最大 50
    Memory      int64 `json:"memory_mb"`     // 默认 256MB, 最大 1GB
    MemorySwap  int64 `json:"memory_swap"`   // 禁用 swap (-1)
    Storage     int64 `json:"storage_mb"`    // 默认 500MB, 最大 5GB
    PidsLimit   int64 `json:"pids_limit"`    // 默认 256, 最大 512
    ReadonlyRoot bool `json:"readonly_root"` // 默认 true
}

var DefaultQuota = ResourceQuota{
    CPUPercent:  25,
    Memory:      256,
    MemorySwap:  -1,
    Storage:     500,
    PidsLimit:   256,
    ReadonlyRoot: true,
}

func (q *ResourceQuota) ToHostConfig() *container.HostConfig {
    return &container.HostConfig{
        Resources: container.Resources{
            CPUPercent: int64(q.CPUPercent),
            Memory:     q.Memory * 1024 * 1024,
            MemorySwap: q.MemorySwap,
            PidsLimit:  &q.PidsLimit,
        },
        ReadonlyRootfs: q.ReadonlyRoot,
        SecurityOpt:    []string{"no-new-privileges"},
    }
}
```

---

## 19 蓝绿部署 (v1.0)

```mermaid
sequenceDiagram
    participant User as 用户
    participant AC as 应用中心
    participant RT as Runtime
    participant Caddy as Caddy
    participant Old as Blue 容器 (旧版)
    participant New as Green 容器 (新版)

    User->>AC: "给请假系统加个功能"
    AC->>RT: CreateSnapshot(appID)
    RT->>Old: docker commit → 快照
    AC->>RT: BlueGreenDeploy(appID, newSpec)
    RT->>New: 构建新镜像 + 启动 Green 容器
    RT->>New: 健康检查 (3次, 间隔5s)
    alt 健康检查通过
        RT->>Caddy: 切换路由 → Green
        RT->>Old: 停止 Blue (保留30分钟)
        RT-->>User: ✅ 更新成功
    else 健康检查失败
        RT->>New: 删除 Green 容器
        RT-->>User: ❌ 更新失败，保留旧版本
    end
```

```go
// internal/runtime/docker/bluegreen.go

type BlueGreenDeployer struct {
    docker *DockerManager
    caddy  *CaddyManager
    backup *BackupService
}

func (bg *BlueGreenDeployer) Deploy(ctx context.Context, appID string, newSpec AppSpec) error {
    // 1. 创建快照 (trigger=before_update)
    app, _ := bg.docker.GetApp(ctx, appID)
    
    // 2. 构建新镜像
    newImage, _ := bg.docker.builder.Build(ctx, newSpec.Code, app.Slug)
    
    // 3. 启动 Green 容器 (同一网络, 不同端口)
    greenID, _ := bg.docker.createContainer(ctx, newImage, app.NetworkID, 3001)
    
    // 4. 健康检查 Green
    if !bg.healthCheck(ctx, greenID, 3, 5*time.Second) {
        bg.docker.removeContainer(ctx, greenID)
        return fmt.Errorf("runtime: green container health check failed")
    }
    
    // 5. 切换 Caddy 路由
    bg.caddy.UpdateRoute(ctx, app.Domain, fmt.Sprintf("green-%s:3001", app.Slug))
    
    // 6. 停止 Blue 容器 (延迟 30 分钟删除, 供回滚)
    bg.docker.stopContainer(ctx, app.ContainerID)
    time.AfterFunc(30*time.Minute, func() {
        bg.docker.removeContainer(context.Background(), app.ContainerID)
    })
    
    // 7. 更新应用记录
    // 8. 发布事件 app.iteration.done
    return nil
}

func (bg *BlueGreenDeployer) Rollback(ctx context.Context, appID string) error {
    // 1. 获取最近快照
    // 2. 恢复镜像和数据卷
    // 3. 切换 Caddy 路由回 Blue
    // 4. 删除 Green 容器
}
```

---

## 20 WASM 沙箱 (v1.0)

### 20.1 Wazero 运行时

```go
// internal/runtime/wasm/sandbox.go

type WasmConfig struct {
    FuelLimit    uint64        `json:"fuel_limit"`    // 默认 1_000_000
    MemoryLimit  uint32        `json:"memory_mb"`     // 默认 10MB
    Timeout      time.Duration `json:"timeout"`       // 默认 5s
    HostFuncs    []string      `json:"host_funcs"`    // 白名单
}

var DefaultWasmConfig = WasmConfig{
    FuelLimit:   1_000_000,
    MemoryLimit: 10,
    Timeout:     5 * time.Second,
    HostFuncs:   []string{"env.log", "env.http_fetch", "env.db_query"},
}

type WasmSandbox struct {
    runtime wazero.Runtime
    modules map[string]wazero.CompiledModule
    mu      sync.RWMutex
}

func NewWasmSandbox() (*WasmSandbox, error) {
    ctx := context.Background()
    rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigCompiler().
        WithCompilationCache(wazero.NewCompilationCache()))
    return &WasmSandbox{runtime: rt, modules: make(map[string]wazero.CompiledModule)}, nil
}

func (s *WasmSandbox) LoadModule(ctx context.Context, wasmPath string, config WasmConfig) (string, error) {
    data, _ := os.ReadFile(wasmPath)
    compiled, _ := s.runtime.CompileModule(ctx, data)
    moduleID := uuid.NewV7().String()
    s.mu.Lock()
    s.modules[moduleID] = compiled
    s.mu.Unlock()
    return moduleID, nil
}

func (s *WasmSandbox) Execute(ctx context.Context, moduleID string, input []byte, config WasmConfig) ([]byte, error) {
    s.mu.RLock()
    compiled, ok := s.modules[moduleID]
    s.mu.RUnlock()
    if !ok {
        return nil, fmt.Errorf("runtime/wasm: module %s not found", moduleID)
    }
    
    execCtx, cancel := context.WithTimeout(ctx, config.Timeout)
    defer cancel()
    
    modConfig := wazero.NewModuleConfig().
        WithStdin(bytes.NewReader(input)).
        WithStdout(&outputBuf).
        WithFSConfig(wazero.NewFSConfig())
    
    hostModule := s.runtime.NewHostModuleBuilder("env")
    for _, fn := range config.HostFuncs {
        switch fn {
        case "env.log":
            hostModule.NewFunctionBuilder().
                WithGoFunction(api.GoFunc(hostLog), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, nil).
                Export("log")
        case "env.http_fetch":
            hostModule.NewFunctionBuilder().
                WithGoFunction(api.GoFunc(hostHTTPFetch), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).
                Export("http_fetch")
        }
    }
    hostModule.Instantiate(execCtx)
    
    mod, _ := s.runtime.InstantiateModule(execCtx, compiled, modConfig)
    defer mod.Close(execCtx)
    
    return outputBuf.Bytes(), nil
}
```

### 20.2 运行时选择逻辑

```go
// internal/runtime/selector.go

func SelectRuntime(moduleType string) string {
    switch moduleType {
    case "App", "DockerModule":
        return "docker"
    case "WasmModule":
        return "wasm"
    case "WasmAIModule":
        return "wasmai" // v2.0
    default:
        return "docker"
    }
}
```

---

## 21 定时调度器 (v1.0)

```go
// internal/runtime/cron/scheduler.go

type CronTask struct {
    ID        string    `json:"id" db:"id"`
    AppID     string    `json:"app_id" db:"app_id"`
    Name      string    `json:"name" db:"name"`
    Schedule  string    `json:"schedule" db:"schedule"`
    Action    string    `json:"action" db:"action"`     // http_call | script | event
    Config    TaskConfig `json:"config" db:"config"`
    Enabled   bool      `json:"enabled" db:"enabled"`
    LastRun   time.Time `json:"last_run" db:"last_run"`
    NextRun   time.Time `json:"next_run" db:"next_run"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type TaskConfig struct {
    URL     string            `json:"url,omitempty"`
    Method  string            `json:"method,omitempty"`
    Headers map[string]string `json:"headers,omitempty"`
    Body    string            `json:"body,omitempty"`
    Command string            `json:"command,omitempty"`
    EventTopic string         `json:"event_topic,omitempty"`
    EventData  map[string]interface{} `json:"event_data,omitempty"`
}

type Scheduler struct {
    cron     *cron.Cron
    tasks    map[string]cron.EntryID
    eventBus EventBus
    mu       sync.RWMutex
}

func (s *Scheduler) Schedule(ctx context.Context, task CronTask) (string, error) {
    entryID, err := s.cron.AddFunc(task.Schedule, func() {
        s.execute(context.Background(), task)
    })
    // 记录 task → entryID 映射
}

func (s *Scheduler) execute(ctx context.Context, task CronTask) {
    switch task.Action {
    case "http_call":
        resp, _ := http.NewRequest(task.Config.Method, task.Config.URL, strings.NewReader(task.Config.Body))
    case "script":
        // docker exec 在应用容器内执行命令
    case "event":
        s.eventBus.Publish(ctx, Event{Topic: task.Config.EventTopic, Data: task.Config.EventData})
    }
}
```

用例：自主应用包（SOP 调度），如竞品监控每天 8:00 采集 → 9:00 分析 → 每周一生成周报。

---

## 22 WASM + AI 推理 (v2.0)

```go
// internal/runtime/wasm/wasmai.go

type WasmAIModule struct {
    sandbox *WasmSandbox
    ollama  *OllamaClient
}

func (m *WasmAIModule) Execute(ctx context.Context, moduleID string, input []byte, model string) ([]byte, error) {
    // 1. WASM 模块执行预处理
    preprocessed, _ := m.sandbox.Execute(ctx, moduleID, input, DefaultWasmConfig)
    
    // 2. 调用本地 AI 模型 (WASI-NN 标准接口)
    aiResult, _ := m.ollama.Generate(ctx, model, string(preprocessed))
    
    // 3. WASM 模块后处理
    return m.sandbox.Execute(ctx, moduleID, []byte(aiResult), DefaultWasmConfig)
}

// WebLLM 浏览器端推理 (v2.0)
// 不在 Go 后端实现，而是前端通过 WebGPU 直接运行 Gemma3-1B
// 见 DD-08 前端设计
```

---

## 23 数据持久化目录结构

所有平台数据统一存储在 `/data/bitengine/` 下，由安装脚本创建。

```
/data/bitengine/
├── apps/                         # 应用中心数据
│   ├── {app-name}/               # 每个应用的数据卷 (Docker volume mount)
│   │   ├── db/                   # SQLite / 数据文件
│   │   ├── uploads/              # 用户上传
│   │   └── config.json           # 应用配置
│   └── ...
├── data-hub/                     # 数据中心数据
│   ├── knowledge-base/           # RAG 知识库
│   │   ├── vectors/              # 向量存储 (AES-256-GCM 加密)
│   │   └── documents/            # 原始文档备份
│   ├── pipelines/                # 数据管道配置
│   └── exports/                  # 数据导出缓存 (自动过期)
├── media/                        # v7 新增：媒体文件存储
│   └── {year}/{month}/{hash}/    # 按时间+哈希组织
├── iot-hub/                      # 物联网中心数据
│   ├── device-registry/          # 设备注册信息
│   ├── rules/                    # 自动化规则
│   ├── sensor-data/              # 传感器原始数据 (90天滚动)
│   ├── vision/                   # 视觉检测截图 (v2.0)
│   └── mosquitto/                # MQTT 5.0 Broker 数据
├── modules/                      # 能力模块数据
├── platform/                     # 平台服务数据
│   ├── postgres/                 # PostgreSQL 数据
│   ├── redis/                    # Redis 持久化
│   ├── vault.db                  # 密钥保险库 (AES-256-GCM 加密)
│   └── audit/                    # Merkle 审计链 (append-only)
├── backups/                      # 自动备份
│   ├── daily/                    # 每日备份 (保留7天)
│   ├── weekly/                   # 每周备份 (保留4周)
│   └── snapshots/                # 应用级快照
├── registry/                     # 本地模块缓存
└── config/                       # 平台配置
    ├── bitengine.yaml            # 主配置
    └── mcp-registry.yaml         # v7 新增：MCP Server 注册配置
```

```go
// internal/runtime/paths/paths.go

const DataRoot = "/data/bitengine"

func AppDataDir(appName string) string    { return filepath.Join(DataRoot, "apps", appName) }
func MediaDir() string                    { return filepath.Join(DataRoot, "media") }  // v7 新增
func KBVectorDir() string                 { return filepath.Join(DataRoot, "data-hub", "knowledge-base", "vectors") }
func SensorDataDir() string               { return filepath.Join(DataRoot, "iot-hub", "sensor-data") }
func BackupDir(kind string) string         { return filepath.Join(DataRoot, "backups", kind) }
func PlatformDir(service string) string    { return filepath.Join(DataRoot, "platform", service) }

func EnsureDirectories() error {
    dirs := []string{
        "apps", "data-hub/knowledge-base/vectors", "data-hub/knowledge-base/documents",
        "data-hub/pipelines", "data-hub/exports",
        "media",  // v7 新增
        "iot-hub/device-registry", "iot-hub/rules", "iot-hub/sensor-data", "iot-hub/mosquitto",
        "modules", "platform/postgres", "platform/redis", "platform/audit",
        "backups/daily", "backups/weekly", "backups/snapshots",
        "registry", "config",
    }
    for _, d := range dirs {
        if err := os.MkdirAll(filepath.Join(DataRoot, d), 0750); err != nil {
            return err
        }
    }
    return nil
}
```

---

## 24 应用运行层数据库 Schema

```sql
-- MVP
CREATE TABLE apps (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL,
    slug           TEXT UNIQUE NOT NULL,
    status         TEXT NOT NULL DEFAULT 'created',
    runtime_type   TEXT NOT NULL DEFAULT 'docker',
    container_id   TEXT,
    image_tag      TEXT,
    network_id     TEXT,
    domain         TEXT,
    port           INTEGER DEFAULT 3000,
    version        INTEGER DEFAULT 1,
    resources      JSONB DEFAULT '{}',
    health_check   JSONB DEFAULT '{}',
    network_policy JSONB DEFAULT '{"deny_all": true}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- v1.0
CREATE TABLE app_versions (
    id         TEXT PRIMARY KEY,
    app_id     TEXT REFERENCES apps(id) ON DELETE CASCADE,
    version    INTEGER NOT NULL,
    image_tag  TEXT NOT NULL,
    code_hash  TEXT NOT NULL,
    changelog  TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE cron_tasks (
    id         TEXT PRIMARY KEY,
    app_id     TEXT REFERENCES apps(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    schedule   TEXT NOT NULL,
    action     TEXT NOT NULL,
    config     JSONB NOT NULL DEFAULT '{}',
    enabled    BOOLEAN DEFAULT true,
    last_run   TIMESTAMPTZ,
    next_run   TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE wasm_modules (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    wasm_path    TEXT NOT NULL,
    config       JSONB NOT NULL DEFAULT '{}',
    module_type  TEXT NOT NULL DEFAULT 'wasm',
    loaded       BOOLEAN DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## 25 应用运行层 API 端点

| 方法 | 端点 | 说明 | 阶段 |
|------|------|------|------|
| GET | `/api/v1/runtime/containers` | 容器列表与状态 | MVP |
| GET | `/api/v1/runtime/containers/:id/stats` | 容器资源统计 | MVP |
| PUT | `/api/v1/runtime/containers/:id/resources` | 更新资源配额 | MVP |
| GET | `/api/v1/runtime/wasm` | WASM 模块列表 | v1.0 |
| POST | `/api/v1/runtime/wasm/load` | 加载 WASM 模块 | v1.0 |
| POST | `/api/v1/runtime/wasm/:id/execute` | 执行 WASM | v1.0 |
| DELETE | `/api/v1/runtime/wasm/:id` | 卸载 WASM | v1.0 |
| GET | `/api/v1/runtime/cron` | 定时任务列表 | v1.0 |
| POST | `/api/v1/runtime/cron` | 创建定时任务 | v1.0 |
| PUT | `/api/v1/runtime/cron/:id` | 更新任务 | v1.0 |
| DELETE | `/api/v1/runtime/cron/:id` | 删除任务 | v1.0 |
| POST | `/api/v1/runtime/cron/:id/trigger` | 手动触发 | v1.0 |

---

## 26 应用运行层错误码

| 错误码 | 说明 | 阶段 |
|--------|------|------|
| `RUNTIME_BUILD_FAILED` | Docker 镜像构建失败 | MVP |
| `RUNTIME_START_FAILED` | 容器启动失败 | MVP |
| `RUNTIME_QUOTA_EXCEEDED` | 资源配额超限 | MVP |
| `RUNTIME_NETWORK_FAILED` | 网络创建失败 | MVP |
| `RUNTIME_HEALTH_FAILED` | 健康检查失败 | MVP |
| `RUNTIME_BLUEGREEN_FAILED` | 蓝绿部署失败 | v1.0 |
| `RUNTIME_ROLLBACK_NO_SNAPSHOT` | 无可用快照 | v1.0 |
| `RUNTIME_WASM_LOAD_FAILED` | WASM 模块加载失败 | v1.0 |
| `RUNTIME_WASM_FUEL_EXHAUSTED` | WASM 燃料耗尽 | v1.0 |
| `RUNTIME_WASM_TIMEOUT` | WASM 执行超时 | v1.0 |
| `RUNTIME_CRON_INVALID` | Cron 表达式无效 | v1.0 |

---

## 27 应用运行层测试策略

| 类型 | 覆盖 | 工具 |
|------|------|------|
| 单元测试 | 资源配额计算、Cron 表达式解析、网络策略匹配 | `testing` |
| 集成测试 | Docker 创建/启动/停止/删除全链路、蓝绿部署 | `testcontainers-go` |
| 沙箱测试 | WASM 燃料耗尽、内存超限、超时中断、宿主函数白名单 | Wazero test |
| 性能测试 | WASM 启动延迟 <10ms、Docker 启动 <3s | benchmark |
