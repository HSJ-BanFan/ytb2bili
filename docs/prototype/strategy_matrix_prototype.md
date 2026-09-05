# 原型设计：策略矩阵与投稿防线数据模型 (Issue #4)

## 1. 背景与目标

在 Agent 视频搬运工厂中，为满足多源头（YouTube 频道、Twitch 直播间等）向多个 B 站账号自主分发并保证确定性安全底线，需要建立两套核心机制：

1. **策略矩阵 (Strategy Matrix)**：以多对多网格模型替代传统的单账号单配置，灵活支持“一源多投（主号全自动、副号待审）”和“多源汇聚”。
2. **投稿防线 (Publishing Guardrails)**：提供物理级幂等防重机制、Worker 并发排队锁、以及三级（全局/频道/账号）熔断暂停开关，防止重复投稿和风控扩散。

---

## 2. 数据实体与表结构设计 (GORM)

### 2.1 来源媒体频道：`SourceChannel` (`cw_source_channels`)

| 字段名 | 类型 | 说明 |
| :--- | :--- | :--- |
| `id` | `INTEGER PRIMARY KEY` | 自增主键 (`BaseModel`) |
| `created_at` / `updated_at` | `DATETIME` | 审计时间戳 |
| `platform` | `VARCHAR(50)` | 来源平台 (`youtube`, `twitch`)，带索引 |
| `channel_id` | `VARCHAR(150)` | 原生平台标识（如 YouTube 频道 ID `UCxxxx` 或 Handle），带索引 |
| `channel_name` | `VARCHAR(255)` | 频道友好显示名称 |
| `channel_url` | `VARCHAR(500)` | 来源主页链接 |
| `fetch_type` | `VARCHAR(50)` | 采集模式：`channel_video` (点播搬运), `live_stream` (直播录制), `playlist` |
| `is_enabled` | `BOOLEAN` | 频道级开关（三级熔断第 2 级），缺省 `true` |
| `status` | `VARCHAR(30)` | 状态：`active`, `paused`, `error` |
| `cron_expression` | `VARCHAR(100)` | 轮询周期表达式，缺省 `@every 30m` |
| `last_checked_at` | `DATETIME` | 最后轮询扫描时间 |
| `extra_config` | `TEXT` | JSON 扩展字段（分辨率偏好、关键字黑白名单等） |

### 2.2 搬运策略规则：`StrategyRule` (`cw_strategy_rules`)

连接 `SourceChannel` 与 `UserBiliAccount` 的多对多关系实体，定义具体加工与发布规则：

| 字段名 | 类型 | 说明 |
| :--- | :--- | :--- |
| `id` | `INTEGER PRIMARY KEY` | 自增主键 |
| `source_channel_id` | `INTEGER` | 关联 `SourceChannel.ID`，外键索引 |
| `bili_account_id` | `INTEGER` | 关联 `UserBiliAccount.ID`，外键索引 |
| `rule_name` | `VARCHAR(150)` | 规则名称（如“科技主号全自动搬运”） |
| `is_enabled` | `BOOLEAN` | 规则启用状态 |
| `priority` | `INTEGER` | 调度优先级（数值越大越先执行） |
| `auto_publish` | `BOOLEAN` | 是否全自动投稿；`false` 时仅生成草稿并进入待人工确认队列 |
| `dynamic_title_template` | `VARCHAR(500)` | 标题生成模板或 Agent 提示词前缀 |
| `desc_template` | `TEXT` | 简介模板（如保留原作者声明与链接） |
| `default_tags` | `VARCHAR(500)` | 缺省标签列表（逗号分隔） |
| `category_id` | `INTEGER` | B 站投稿分区 TID (例如 188 为科技区，17 为单机游戏) |
| `copyright` | `INTEGER` | 1=自制, 2=转载（搬运默认为 2） |
| `source_origin` | `VARCHAR(500)` | 转载来源地址声明 |
| `dtime_delay_minutes` | `INTEGER` | 定时发布延迟分钟数（0 为立即发布） |
| `extra_fields` | `TEXT` | JSON 扩展配置（分 P 聚合偏好、水印位置等） |

### 2.3 投稿指纹与并发排队锁：`PublishFingerprint` (`cw_publish_fingerprints`)

投稿防线的第一道核心锁，物理级杜绝重复投稿：

| 字段名 | 类型 | 说明 |
| :--- | :--- | :--- |
| `id` | `INTEGER PRIMARY KEY` | 自增主键 |
| `source_platform` | `VARCHAR(50)` | 来源平台 (`youtube`, `twitch`) |
| `source_video_id` | `VARCHAR(150)` | 原视频唯一 ID |
| `source_segment_hash` | `VARCHAR(64)` | 媒体分段 SHA-256 哈希 |
| `bili_account_id` | `INTEGER` | 目标 B 站账号 ID |
| `strategy_rule_id` | `INTEGER` | 匹配的策略规则 ID |
| `fingerprint_hash` | `VARCHAR(64)` | **唯一组合哈希**：`SHA256(Platform:VideoID:BiliAccountID)`，唯一索引 |
| `publish_status` | `VARCHAR(30)` | 状态：`pending`, `locked`, `published`, `failed`, `deadletter` |
| `bili_bvid` | `VARCHAR(50)` | 发布成功后的 B 站 BV 号 |
| `bili_aid` | `BIGINT` | 发布成功后的 B 站 AID |
| `published_at` | `DATETIME` | 成功发布时间戳 |
| `retry_count` | `INTEGER` | 当前失败重试次数（缺省 0） |
| `max_retries` | `INTEGER` | 最大允许重试次数（缺省 3） |
| `lock_expires_at` | `DATETIME` | **执行排队锁到期时间**，索引 |
| `dead_letter_reason` | `TEXT` | 超过最大重试次数或严重违规进入死信的原因 |

### 2.4 系统投稿防线与熔断开关：`SystemGuardrail` (`cw_system_guardrails`)

| 字段名 | 类型 | 说明 |
| :--- | :--- | :--- |
| `id` | `INTEGER PRIMARY KEY` | 自增主键 |
| `scope` | `VARCHAR(30)` | 熔断层级：`global`, `channel`, `account`，带索引 |
| `target_id` | `VARCHAR(100)` | 目标标识：全局为 `"0"`, 频道为 `ChannelID`, 账号为 `AccountID` |
| `is_paused` | `BOOLEAN` | 是否处于暂停/熔断状态 |
| `pause_reason` | `VARCHAR(255)` | 触发原因（如 `manual_kill_switch`, `http_601_rate_limit`） |
| `consecutive_failures` | `INTEGER` | 当前连续失败次数 |
| `failure_threshold` | `INTEGER` | 触发自动熔断的失败阈值（缺省 3） |
| `auto_resume_at` | `DATETIME` | 冷却期结束时间，超时后自动恢复放行 |
| `last_triggered_at` | `DATETIME` | 最后触发时间 |

---

## 3. 核心机制原型与验证逻辑

我们在 `internal/prototype/strategy_matrix_test.go` 中编写了完备的自动化测试用例，涵盖以下三项核心能力的闭环验证：

### 3.1 多对多网格路由 (Multi-to-Multi Routing)

- **场景**：
  - 频道 A（科技）通过两条规则分别投递给账号 1（全自动精翻）与账号 2（备份归档待人工审）。
  - 频道 B（游戏）投递给账号 2（切片速发）。
- **验证结论**：
  - 调度器扫描到新视频时，能够按 `priority DESC` 正确拉取到该频道的全部投稿目标及差异化模板。
  - 支持从账号视角反向查询接收的所有来源频道。

### 3.2 投稿幂等防重与原子排队锁 (Idempotency & Concurrent Locks)

- **确定性哈希**：`FingerprintHash = SHA256(Platform + ":" + VideoID + ":" + AccountID)`。
- **物理唯一性**：数据库 `uniqueIndex` 强制拦截任何重复插入，消除竞态。
- **原子行锁占有 (Atomic Claim)**：

  ```sql
  UPDATE cw_publish_fingerprints
  SET publish_status = 'locked', lock_expires_at = ?
  WHERE fingerprint_hash = ?
    AND (publish_status = 'pending' OR (publish_status = 'locked' AND lock_expires_at < ?))
  ```

- **崩溃自动接管 (Crash Recovery)**：Worker 异常退出导致锁未释放时，后续 Worker 在 `lock_expires_at < now` 时自动安全抢占接管。
- **终态不可逆**：一旦更新为 `published` 并填入 `bili_bvid`，任何重复扫描均立即短路跳过。

### 3.3 三级熔断暂停开关 (3-Tier Pause Switch)

统一执行前防御评估：

```text
[任务发起]
   │
   ├─► 检查 Tier 1 (Global): 全局开关是否拉闸？
   │      └─ 是 ──► 阻断所有投稿 (Emergency Stop)
   │
   ├─► 检查 Tier 2 (Channel): 来源频道是否被禁用或熔断？
   │      └─ 是 ──► 阻断该频道的视频，其余频道正常流转
   │
   ├─► 检查 Tier 3 (Account): 目标 B 站账号是否被停用或触发 HTTP 601 限流？
   │      └─ 是 ──► 阻断投往该账号的任务，其余账号不受影响
   │
   └─► 全部通过 ──► 放行进入 biliup 投稿管道
```

- **冷却自动恢复**：当账号触发 HTTP 601 限流自动熔断并设置 30 分钟冷却后，评估逻辑在检测到 `time.Now() > AutoResumeAt` 时自动解除暂停并清零连续失败计数。

---

## 4. 架构契约与演进结论

1. **与现有系统的无缝兼容**：
   - 引用现有的 `BaseModel` 与 `UserBiliAccount` 模型。
   - 已注册进 `pkg/store/migrate.go` 的 `MigrateDatabase` 自动建表流程。
2. **与 biliup (Issue #2) 的协作接口**：
   - 在进入 `biliup upload` 之前，必须先在 `PublishFingerprint` 中锁定记录并校验三级防线。
   - biliup 上传成功获取 BV 号后，在同事务中更新 `bili_bvid` 与 `published` 状态。
3. **与 Pi Agent (Issue #3) 的协作接口**：
   - `StrategyRule` 中的 `DynamicTitleTemplate`、`DescTemplate`、`DefaultTags` 作为上下文输入给 Pi Agent。
   - Agent 根据输入生成最终发布元数据，若 `StrategyRule.AutoPublish == false`，则状态置为 `pending_review` 等待人工放行。
