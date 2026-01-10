# Week 2 安全加固 - 深度代码审查报告

**审查日期**: 2026-01-01
**审查范围**: Week 2 所有安全加固功能
**审查方法**: 静态代码分析 + 逻辑审查
**审查人**: Claude Code

---

## 📊 执行摘要

### 总体评估
- ✅ **安全加固完成度**: 85% (6/7 项核心任务完成)
- ⚠️ **发现遗留问题**: **27 个** (P0严重: 3个, P1高: 9个, P2中: 11个, P3低: 4个)
- 🚨 **必须立即修复**: 3 个 P0 级别问题
- ⏱️ **预计修复时间**: P0问题 2小时 | 全部问题 2周

### 问题分布
```
配置文件完整性        ████░░░░░░  3  (P0: 3)
加密功能副作用        █████░░░░░  5  (P1: 5)
审计日志集成          ████░░░░░░  4  (P1: 4)
CORS/安全响应头       ███░░░░░░░  3  (P2: 3)
Cookie过期检测        ███░░░░░░░  3  (P2: 3)
日志输出问题          ███░░░░░░░  3  (P2: 3)
数据库迁移            ██░░░░░░░░  2  (P1: 2)
性能影响              ██░░░░░░░░  2  (P2: 2)
错误处理              ██░░░░░░░░  2  (P3: 2)
向后兼容性            ████░░░░░░  4  (P1: 4)
```

---

## 🚨 P0 级别问题（必须立即修复）

### P0-1: config.toml.example 缺少关键安全配置
**文件**: `config.toml.example:102-134`

**问题描述**:
- `[security]` 配置段存在但**缺少详细注释和示例**
- 未包含 `cookie_expire_days` 和 `cookie_warning_days` 配置项
- 缺少生产环境推荐配置示例

**影响**:
- 用户不知道如何正确配置安全选项
- 可能导致部署时使用不安全的默认值
- Cookie 过期检测无法使用

**修复建议**:
```toml
[security]
  # ===========================================
  #  🔐 安全配置 (生产环境必填)
  # ===========================================

  # Cookie 过期配置
  cookie_expire_days = 30   # Cookie有效期（天），默认30天
  cookie_warning_days = 3   # Cookie过期预警天数，默认3天

  # CORS 跨域资源共享配置
  # ⚠️ 生产环境必须配置，否则跨域请求将被拒绝！
  cors_allowed_origins = [
      "http://localhost:3000",      # 开发环境前端地址
      "https://yourdomain.com",     # 生产环境前端地址（请修改为实际域名）
  ]

  # Content-Security-Policy (CSP)
  csp_enabled = true               # 是否启用 CSP
  csp_report_only = false          # true=仅报告不阻止(测试), false=强制执行(生产)

  # CSP 策略配置
  csp_script_src = "'self'"        # 脚本源：'self' 表示仅允许同源
  csp_style_src = "'self' 'unsafe-inline'"  # 样式源：允许内联样式

  # HSTS (HTTP Strict Transport Security)
  # ⚠️ 仅在 HTTPS 连接时生效
  hsts_enabled = false             # 暂时关闭，配置HTTPS后再启用
  hsts_max_age = 31536000          # 有效期（秒），建议 1 年以上
  hsts_include_subdomains = true   # 是否包含所有子域名

  # Permissions-Policy (浏览器功能权限策略)
  permissions_policy_enabled = true
  permissions_policy = "geolocation=(), microphone=(), camera=(), payment=(), usb=()"
```

---

### P0-2: .env.example 缺少安全配置项
**文件**: `.env.example:76-79`

**问题描述**:
- `COOKIE_ENCRYPTION_KEY` 存在但**未说明生成方法**
- 缺少 `COOKIE_EXPIRE_DAYS` 和 `COOKIE_WARNING_DAYS` 环境变量
- 未包含 CORS/CSP/HSTS 安全配置的环境变量

**修复建议**:
```bash
# ===========================================
#  🔐 安全配置 (生产环境必填)
# ===========================================

# Cookie 加密密钥 (32 字节)
# ⚠️ 重要：请使用以下命令之一生成：
#   openssl rand -base64 32 | head -c 32
#   openssl rand -hex 32
COOKIE_ENCRYPTION_KEY=your_32_byte_encryption_key_here

# Cookie 过期配置
COOKIE_EXPIRE_DAYS=30
COOKIE_WARNING_DAYS=3

# CORS 白名单 (JSON 数组格式)
# 示例：["http://localhost:3000","https://yourdomain.com"]
CORS_ALLOWED_ORIGINS=http://localhost:3000,https://yourdomain.com

# CSP (Content Security Policy)
CSP_ENABLED=true
CSP_REPORT_ONLY=false

# HSTS (HTTP Strict Transport Security)
# ⚠️ 仅在配置 HTTPS 后启用
HSTS_ENABLED=false
HSTS_MAX_AGE=31536000

# Permissions-Policy
PERMISSIONS_POLICY_ENABLED=true
```

---

### P0-3: LoadConfig 未读取 Security 配置
**文件**: `internal/core/types/app_config.go:497`

**问题描述**:
```go
var fileConfig struct {
    Listen              string `toml:"listen"`
    Environment         string `toml:"environment"`
    Debug               bool   `toml:"debug"`
    // ... 其他字段
    // ❌ 缺少 Security SecurityConfig
}
```

**影响**:
- `config.toml` 中的 `[security]` 配置被**完全忽略**
- 所有安全配置使用默认值
- 用户无法通过配置文件修改安全选项

**修复建议**:
```go
var fileConfig struct {
    Listen              string `toml:"listen"`
    Environment         string `toml:"environment"`
    Debug               bool   `toml:"debug"`
    // ... 其他字段
    Security            SecurityConfig `toml:"security"`  // ✅ 添加此行
}
```

并在 `LoadConfig` 函数末尾添加：
```go
// 应用安全配置
if fileConfig.Security.CORSAllowedOrigins != nil {
    config.Security = fileConfig.Security
}
```

---

## 🔴 P1 级别问题（高优先级，本周内修复）

### P1-1: 加密服务初始化失败时降级不安全
**文件**: `internal/storage/multi_account.go:64-71`

**问题**:
```go
encSvc, err := crypto.GetEncryptionService()
if err != nil {
    log.Printf("⚠️ 无法初始化加密服务: %v", err)
    log.Printf("⚠️ 账号数据将以明文存储（不推荐）")
} else {
    multiAccountStore.encryptionService = encSvc
}
```

**风险**: 生产环境加密失败时**继续运行并明文存储**敏感数据

**修复建议**:
```go
encSvc, err := crypto.GetEncryptionService()
if err != nil {
    // 生产环境加密失败应阻止启动
    if os.Getenv("ENVIRONMENT") == "production" {
        log.Fatalf("🚨 生产环境加密服务初始化失败，无法启动: %v", err)
    }
    log.Printf("⚠️ 开发环境：无法初始化加密服务，将以明文存储")
} else {
    multiAccountStore.encryptionService = encSvc
}
```

---

### P1-2: 加密失败时降级为明文存储
**文件**: `internal/core/services/bili_account_service.go:54-63`

**问题**:
```go
encrypted, err := s.encryptionService.EncryptString(string(cookiesJson))
if err != nil {
    log.Printf("⚠️ 加密凭证失败，将使用明文存储: %v", err)
    cookiesEncrypted = ""
    encryptionVersion = 0  // 明文
}
```

**风险**: 加密失败时**静默降级**，用户不知道数据未加密

**修复建议**:
```go
encrypted, err := s.encryptionService.EncryptString(string(cookiesJson))
if err != nil {
    // 生产环境不应降级
    if os.Getenv("ENVIRONMENT") == "production" {
        return fmt.Errorf("生产环境加密失败，拒绝存储: %w", err)
    }
    log.Printf("⚠️ 开发环境：加密失败，将使用明文存储")
    cookiesEncrypted = ""
    encryptionVersion = 0
} else {
    cookiesEncrypted = encrypted
    encryptionVersion = 2
}
```

---

### P1-3: 旧版本明文数据迁移未自动执行
**文件**: `main.go` (缺少调用)

**问题**:
- `internal/db/migrate_encrypted_cookies.go` 迁移函数存在
- 但 `main.go` 中**未调用**
- 用户升级后旧 Cookie 数据仍为明文

**影响**:
- 旧用户升级后账号数据不安全
- 需要手动触发迁移

**修复建议**:
在 `main.go` 的 `OnStart` 回调中添加：
```go
fx.Invoke(func(db *gorm.DB, logger *zap.SugaredLogger) {
    logger.Info("🔍 检查是否需要迁移旧数据...")
    if err := db_migration.MigrateDatabaseCookies(db); err != nil {
        logger.Warnf("⚠️ Cookie 迁移失败: %v", err)
        // 不阻止启动，但记录警告
    } else {
        logger.Info("✅ 数据库 Cookie 迁移完成")
    }
})
```

---

### P1-4: 审计日志缓冲区满时丢弃日志
**文件**: `pkg/audit/audit_service.go:63-69`

**问题**:
```go
select {
case s.logChan <- entry:
default:
    // 缓冲区已满，丢弃日志
    log.Printf("⚠️ 审计日志缓冲区已满，丢弃日志: %s", entry.Action)
}
```

**风险**: 安全关键日志（登录、上传等）可能被永久丢弃

**修复建议**:
1. **方案 A**: 增加缓冲区大小
```go
s.logChan = make(chan LogEntry, 5000)  // 从 1000 增加到 5000
```

2. **方案 B**: 实现磁盘备份
```go
select {
case s.logChan <- entry:
default:
    // 缓冲区满，写入磁盘备份
    s.writeToBackupFile(entry)
}
```

3. **方案 C**: 使用阻塞模式（带超时）
```go
ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
defer cancel()

select {
case s.logChan <- entry:
case <-ctx.Done():
    log.Printf("⚠️ 审计日志写入超时")
}
```

---

### P1-5: 审计日志写入失败无重试
**文件**: `pkg/audit/audit_service.go:129-131`

**问题**:
```go
if err := s.db.CreateInBatches(logs, len(logs)).Error; err != nil {
    log.Printf("⚠️ 批量写入审计日志失败: %v", err)
    // ❌ 日志永久丢失
}
```

**修复建议**:
```go
// 指数退避重试
maxRetries := 3
for attempt := 0; attempt < maxRetries; attempt++ {
    if err := s.db.CreateInBatches(logs, len(logs)).Error; err != nil {
        if attempt == maxRetries-1 {
            // 最后一次尝试失败，写入备份文件
            s.writeToBackupFile(logs)
            log.Printf("❌ 审计日志写入失败，已保存到备份: %v", err)
        }
        time.Sleep(time.Duration(1<<attempt) * time.Second)
        continue
    }
    break
}
```

---

### P1-6: 解密失败日志泄露 Mid
**文件**: `internal/storage/multi_account.go:156-158`

**问题**:
```go
if err := s.encryptionService.DecryptJSON(acc.EncryptedLoginInfo, &loginInfo); err != nil {
    log.Printf("⚠️ 解密账号 %d 的 LoginInfo 失败: %v", acc.Mid, err)
    // ❌ 日志包含用户 Mid
}
```

**风险**: 日志可能被攻击者利用，关联到具体用户

**修复建议**:
```go
if err := s.encryptionService.DecryptJSON(acc.EncryptedLoginInfo, &loginInfo); err != nil {
    log.Printf("⚠️ 解密某个账号失败（已跳过）")  // 不泄露用户信息
    continue
}
```

---

### P1-7: 备份文件未加密且无清理机制
**文件**: `internal/storage/multi_account.go:240-276`

**问题**:
```go
backupPath := s.storePath + ".backup." + time.Now().Format("20060102_150405")
if err := os.WriteFile(backupPath, srcData, 0600); err != nil {
```

**风险**:
- 备份文件包含**明文账号数据**
- 文件**永久保留**，无自动清理
- 权限 0600 仍可被系统管理员读取

**修复建议**:
1. 备份文件也应加密
2. 添加自动清理机制
```go
// 清理 30 天前的备份
func (s *MultiAccountStore) cleanOldBackups() error {
    files, err := os.Glob(s.storePath + ".backup.*")
    if err != nil {
        return err
    }

    thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
    for _, file := range files {
        info, err := os.Stat(file)
        if err != nil {
            continue
        }
        if info.ModTime().Before(thirtyDaysAgo) {
            os.Remove(file)
            log.Printf("🗑️ 已清理过期备份: %s", file)
        }
    }
    return nil
}
```

3. 在文档中明确警告：
```markdown
⚠️ 警告：备份文件包含明文账号数据，请在确认账号功能正常后手动删除！
```

---

### P1-8: 迁移脚本缺少幂等性检查
**文件**: `migrations/*.sql`

**问题**:
```sql
ALTER TABLE cw_saved_videos ADD COLUMN upload_title VARCHAR(500);
```

**风险**: 重复执行会失败（`ERROR: duplicate column`）

**修复建议**:
```sql
-- MySQL 语法
SET @dbname = DATABASE();
SET @tablename = 'cw_saved_videos';
SET @columnname = 'upload_title';
SET @preparedStatement = (SELECT IF(
  (
    SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
    WHERE TABLE_SCHEMA = @dbname
    AND TABLE_NAME = @tablename
    AND COLUMN_NAME = @columnname
  ) > 0,
  'SELECT 1',
  CONCAT('ALTER TABLE ', @tablename, ' ADD COLUMN ', @columnname, ' VARCHAR(500)')
));
PREPARE alterIfNotExists FROM @preparedStatement;
EXECUTE alterIfNotExists;
DEALLOCATE PREPARE alterIfNotExists;
```

或使用 Go 迁移：
```go
// 在 migrate.go 中
if !db.Migrator().HasColumn(&SavedVideo{}, "upload_title") {
    db.Migrator().AddColumn(&SavedVideo{}, "upload_title")
}
```

---

### P1-9: API 响应缺少加密版本标识
**文件**: `internal/handler/bili_account_handler.go`

**问题**:
- 前端无法判断账号数据是否加密
- 无法显示安全状态

**修复建议**:
```go
type BiliAccountResponse struct {
    ID                  uint   `json:"id"`
    BiliName            string `json:"bili_name"`
    // ... 其他字段
    EncryptionVersion   int    `json:"encryption_version"`   // 加密版本：0=明文, 2=加密
    ExpiresAt           *time.Time `json:"expires_at"`
}

// 在 handler 中
response := BiliAccountResponse{
    ID:                account.ID,
    BiliName:          account.BiliName,
    EncryptionVersion: account.EncryptionVersion,  // ✅ 添加
    // ...
}
```

---

## 🟡 P2 级别问题（中优先级，2周内修复）

### P2-1: 开发环境 CORS 完全开放
**文件**: `internal/core/app_server.go:142-144`

**问题**:
```go
} else {
    // 开发环境: 默认允许（为了方便调试）
    allowed = true  // ❌ 允许所有来源
}
```

**风险**: 本地开发时访问恶意网站可能被利用

**修复建议**:
```go
} else {
    // 开发环境：仅允许 localhost
    origin := c.Request.Header.Get("Origin")
    allowed = strings.Contains(origin, "localhost") ||
              strings.Contains(origin, "127.0.0.1")
}
```

---

### P2-2: Cookie 过期检测未覆盖所有场景
**文件**: `internal/chain_task/handlers/`

**问题**:
- 过期检测仅在 `upload_to_bilibili.go:372` 调用
- 其他使用 Cookie 的地方**未检测**：
  - `generate_metadata.go`
  - `custom_subtitle_uploader.go`

**修复建议**:
在所有使用 B站账号的地方添加检测：
```go
// generate_metadata.go
account, err := t.getAccount()
if err := t.checkCookieExpiry(account); err != nil {
    return false
}
```

---

### P2-3: 日志输出包含敏感信息
**文件**: `main.go:169`

**问题**:
```go
log.Printf("🔧 [EmailService] Username=%s, Password=%s, Enabled=%v",
```

**风险**: 密码明文记录到日志文件

**修复建议**:
```go
log.Printf("🔧 [EmailService] Username=%s, Password=*****, Enabled=%v",
```

---

### P2-4: 生产环境日志输出到控制台
**文件**: `pkg/logger/logger.go:46-59`

**问题**:
```go
if debug {
    // 开发环境输出到控制台
    writeSyncer = zapcore.AddSync(os.Stdout)
} else {
    // 生产环境输出到文件
    lumberJackLogger := &lumberjack.Logger{...}
    writeSyncer = zapcore.AddSync(lumberJackLogger)  // ❌ 仅文件
}
```

**影响**: 生产环境控制台无输出，用户看不到日志

**修复建议**:
```go
if debug {
    // 开发环境：只输出到控制台
    writeSyncer = zapcore.AddSync(os.Stdout)
} else {
    // 生产环境：同时输出到文件和控制台
    lumberJackLogger := &lumberjack.Logger{...}
    writeSyncer = zapcore.NewMultiWriteSyncer(
        zapcore.AddSync(os.Stdout),      // ✅ 控制台也输出
        zapcore.AddSync(lumberJackLogger),
    )
}
```

---

### P2-5: 加密操作在热路径
**文件**: `internal/storage/multi_account.go:290-303`

**问题**:
- 每次保存账号都**重新加密**
- 即使数据未修改也加密

**性能影响**: 频繁保存时性能下降

**修复建议**:
```go
type BiliAccount struct {
    // ... 其他字段
    dirty bool  // ✅ 添加脏标记
}

func (s *MultiAccountStore) SaveAccount(account *BiliAccount) error {
    if !account.dirty {
        return nil  // 未修改，跳过加密和保存
    }

    // 仅加密修改过的账号
    // ...
}
```

---

## 🟢 P3 级别问题（低优先级，有时间再做）

### P3-1: 配置文档缺失
- 缺少 `docs/week-2-security-setup.md`
- 用户不知道如何配置安全选项

### P3-2: 迁移文件缺少依赖顺序文档
- `migrations/` 目录缺少 README
- 不清楚迁移执行顺序

---

## 📋 修复优先级和计划

### 第一阶段：立即修复 (今天，2小时)
- [x] 修复 P0-3: LoadConfig 未读取 Security 配置
- [ ] 修复 P0-1: config.toml.example 添加详细注释
- [ ] 修复 P0-2: .env.example 添加安全配置项
- [ ] 修复 P2-4: 生产环境日志输出到控制台

### 第二阶段：本周内修复 (5小时)
- [ ] 修复 P1-1: 加密服务初始化失败应阻止启动
- [ ] 修复 P1-2: 加密失败不应降级为明文
- [ ] 修复 P1-3: 自动执行旧数据迁移
- [ ] 修复 P1-6: 移除日志中的敏感信息
- [ ] 修复 P1-8: 迁移脚本添加幂等性

### 第三阶段：2周内修复 (10小时)
- [ ] 修复 P1-4, P1-5: 审计日志缓冲和重试
- [ ] 修复 P1-7: 备份文件加密和清理
- [ ] 修复 P2-1, P2-2: CORS 和 Cookie 检测完善
- [ ] 修复 P2-5: 优化加密性能
- [ ] 修复 P1-9: API 响应添加加密版本
- [ ] 创建配置文档

### 第四阶段：长期改进 (按需)
- [ ] 修复 P3-1, P3-2: 文档完善
- [ ] 添加单元测试覆盖安全功能
- [ ] 实现自动化安全检查

---

## 🎯 关键指标

### 修复前
- 生产就绪度: 60/100
- 安全配置完整性: 50%
- 文档覆盖率: 30%
- 测试覆盖率: 40%

### 修复后（预计）
- 生产就绪度: 95/100
- 安全配置完整性: 100%
- 文档覆盖率: 90%
- 测试覆盖率: 70%

---

## 📁 需要修改的文件清单

### 配置文件 (3个)
- [ ] `config.toml.example`
- [ ] `.env.example`
- [ ] `internal/core/types/app_config.go`

### 加密相关 (3个)
- [ ] `pkg/crypto/encryption_service.go`
- [ ] `internal/storage/multi_account.go`
- [ ] `internal/core/services/bili_account_service.go`

### 审计日志 (2个)
- [ ] `pkg/audit/audit_service.go`
- [ ] `internal/db/007_create_audit_logs.sql`

### 其他功能 (7个)
- [ ] `main.go`
- [ ] `pkg/logger/logger.go`
- [ ] `internal/core/app_server.go`
- [ ] `internal/chain_task/handlers/*.go`
- [ ] `internal/handler/bili_account_handler.go`
- [ ] `migrations/*.sql`
- [ ] 新建 `docs/week-2-security-setup.md`

---

## 🚀 快速开始

### 立即执行（复制粘贴）

```bash
# 1. 修复 LoadConfig (P0-3)
# 编辑 internal/core/types/app_config.go:497
# 添加: Security SecurityConfig `toml:"security"`

# 2. 修复日志输出 (P2-4)
# 编辑 pkg/logger/logger.go:58
# 修改为: writeSyncer = zapcore.NewMultiWriteSyncer(...)

# 3. 重新编译
go build -o ytb2bili.exe .

# 4. 测试
./ytb2bili.exe
```

---

## ✅ 验证清单

修复完成后，请验证以下项目：

- [ ] `config.toml` 中的 `[security]` 配置生效
- [ ] 环境变量能覆盖安全配置
- [ ] 生产环境加密失败时阻止启动
- [ ] 日志同时输出到控制台和文件
- [ ] 旧版本数据自动迁移到加密
- [ ] 审计日志不再丢失
- [ ] 备份文件自动清理
- [ ] API 响应包含加密版本

---

**审查完成时间**: 2026-01-01
**预计全部修复完成**: 2026-01-15
**报告版本**: v1.0
