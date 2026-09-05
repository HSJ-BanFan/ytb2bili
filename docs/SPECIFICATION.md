# Agent 视频搬运工厂产品与架构规格说明书

(Product & Architecture Specification: Agent Video Transshipment Factory)

- **版本**: v1.0.0
- **定位**: 个人自托管、多投稿账号、全流程自动化的视频采集、加工与 Bilibili 投稿工厂
- **父地图**: [Map: Agent 视频搬运工厂架构与产品规格 (#1)](https://github.com/HSJ-BanFan/ytb2bili/issues/1)

---

## 1. 项目愿景与设计哲学

### 1.1 核心定位

以 **biliup** 为高吞吐媒体采集与分块并发投稿的“执行手足”，以 **ytb2bili** 原生 Go 守护服务为状态机与任务编排的“核心身躯”，以 **Pi Agent**（阶段门禁短会话）为视频理解与元数据本地化规划的“受控大脑”，在 Next.js 控制台统一调度下，构建单机自托管的 YouTube / Twitch 到 Bilibili 的智能视频搬运工厂。

### 1.2 核心原则（Ponytail 准则）

1. **能用确定性代码解决的，坚决不塞进大模型**：大文件 I/O、音视频切片、Whisper 转录、分块上传完全由确定性管道执行；LLM 仅在“内容决策阶段”做受控文本规划。
2. **零多租户与零多余抽象**：保持单操作者自托管架构，不引入用户配额、计费、SaaS 权限等冗余层。
3. **物理级确定性投稿防线**：全自动化不等于失控。系统内置唯一指纹幂等去重、任务原子排队锁、三级熔断急停（全局/频道/账号 601 限流冷却）及死信队列（DLQ），确保任何异常均可控可查。

---

## 2. 整体系统拓扑架构

```text
+-----------------------------------------------------------------------------------------+
|                                    Next.js Web 控制台                                    |
|    - 策略矩阵配置 (/strategy)                       - Agent 决策追溯与人工审核 (/agent-trace)     |
+--------------------------------------------+--------------------------------------------+
                                             | REST / WebSocket
+--------------------------------------------v--------------------------------------------+
|                              Go 编排守护进程 (ytb2bili 核心)                                |
|                                                                                         |
|   +-----------------------+   +-----------------------+   +-------------------------+   |
|   |   Cron 调度器 (10频)   |   |  7阶段门禁状态机引擎    |   |    投稿防线与并发排队锁     |   |
|   +-----------+-----------+   +-----------+-----------+   +------------+------------+   |
+---------------|---------------------------|----------------------------|----------------+
                |                           |                            |
                v                           v                            v
+-------------------------------+ +--------------------+ +-------------------------------+
|      biliup CLI 执行引擎       | |  音视频处理管线    | |     Pi Agent 决策子进程        |
|  - 原始流录制 (stream-gears)   | |  - FFmpeg 音频提取 | |  - pi -p --mode json (无头)  |
|  - UPOS 动态线路探测 (AUTO)    | |  - Whisper 语音转录| |  - 80字标题/标签/简介规划      |
|  - HTTP 601 智能退避重试       | |  - AI 文本翻译     | |  - 敏感词清洗 + 确定性保底回退   |
|  - 多 P 稿件聚合并发提交       | |  - 软字幕/硬字幕压制| +-------------------------------+
+-------------------------------+ +--------------------+
```

---

## 3. 核心领域模型与存储规范 (SQLite / GORM)

代码落地于 `pkg/store/model/strategy_matrix.go`，并在 `pkg/store/migrate.go` 注册自动迁移：

### 3.1 来源媒体频道：`SourceChannel` (`cw_source_channels`)

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| `id` | `uint` | 主键 |
| `platform` | `varchar(50)` | 来源平台: `youtube`, `twitch` |
| `channel_id` | `varchar(150)` | 平台原生频道 ID 或 Handle (如 `UCxxxx`) |
| `channel_name` | `varchar(255)` | 频道名称 |
| `channel_url` | `varchar(500)` | 频道主页 URL |
| `fetch_type` | `varchar(50)` | 模式: `channel_video` (点播), `live_stream` (直播录制) |
| `cron_expression` | `varchar(100)` | 检查周期 (缺省 `@every 30m`) |
| `is_enabled` | `bool` | 频道级启用开关 (三级熔断第 2 级) |
| `status` | `varchar(30)` | 运行状态: `active`, `paused`, `error` |

### 3.2 搬运策略规则：`StrategyRule` (`cw_strategy_rules`)

定义频道与 B 站账号的多对多关系及定制化策略：

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| `id` | `uint` | 主键 |
| `source_channel_id` | `uint` | 关联 `SourceChannel.ID` |
| `bili_account_id` | `uint` | 关联 `UserBiliAccount.ID` |
| `rule_name` | `varchar(150)` | 规则名称 (如“科技主号全自动”) |
| `priority` | `int` | 调度优先级 (数值高优先) |
| `auto_publish` | `bool` | `true` 为全自动直发，`false` 为草稿待审 |
| `category_id` | `int` | B 站目标分区 TID (如 188 科技区) |
| `copyright` | `int` | 1=自制, 2=转载 (默认 2) |
| `dynamic_title_template` | `varchar(500)` | 标题模版或 Agent 提示词前缀 |
| `desc_template` | `text` | 简介模版 (保留原作者出处等) |
| `default_tags` | `varchar(500)` | 默认保底标签 |
| `dtime_delay_minutes` | `int` | 定时发布延迟分钟 (0 为立即) |

### 3.3 投稿指纹与并发排队锁：`PublishFingerprint` (`cw_publish_fingerprints`)

防线第一道核心锁，物理级杜绝同一视频被重复投递至同一 B 站账号：

- **复合哈希算法**: `SHA256(Platform + ":" + SourceVideoID + ":" + BiliAccountID)` 唯一索引。
- **状态流转**: `pending` -> `locked` -> `published` / `failed` -> `deadletter`。
- **防死锁机制**: 包含 `lock_expires_at`，当 Worker 意外崩溃时，锁自动超时过期释放，自愈恢复。

### 3.4 系统投稿防线：`SystemGuardrail` (`cw_system_guardrails`)

三级熔断机制保障平台安全：

1. **Global 级**: 全局紧急刹车 (`target_id = "0"`)，一键阻断所有发布。
2. **Channel 级**: 单频道爬取熔断，频发 404/反爬时单源暂停。
3. **Account 级**: B 站账号级限流保护。当检测到 B 站 HTTP 601（上传过快）或 Cookie 失效时，自动进入熔断状态，设置 30 分钟冷却窗口并提供倒计时自动恢复。

---

## 4. 7 阶段门禁流水线规范

完整的状态机规约落地于 `docs/design/pipeline-state-machine.md`：

| 阶段序号 | 阶段代码 (`task_step`) | 执行主体 | 成功判定与动作 | 失败恢复策略 |
| :--- | :--- | :--- | :--- | :--- |
| **阶段 1** | `source_fetch` | biliup / yt-dlp | 媒体完整下载，生成元数据快照 | 网络错误指数退避（上限 3 次）；404 视频被删直接隔离至死信 |
| **阶段 2** | `audio_extract` | FFmpeg | 提取 16kHz 单声道 WAV/AAC | 格式损坏重试 1 次；持续失败转死信 |
| **阶段 3** | `subtitle_transcribe` | Whisper (faster-whisper) | 生成带时间戳的 `source.srt` | 显存溢出重试；若源视频已内置外挂字幕则直接跳过 |
| **阶段 4** | `subtitle_translate` | DeepSeek / Gemini | 生成目标语言 `target.srt` | 接口配额耗尽指数退避；降级直接使用生肉字幕发布 |
| **阶段 5** | `content_decision` | **Pi Agent (阶段门禁)** | 产出合规 `VideoDecisionResult` | 超时（120s）或 JSON 破损时，自动调用 `GenerateFallbackDecision` 零阻塞降级 |
| **阶段 6** | `video_remux` | FFmpeg | 完成字幕压制或生成分 P 切片 | 硬压制失败降级为软字幕外挂直接输出 |
| **阶段 7** | `bili_upload` | **biliup CLI** | 返回 BV 号与 AID，写回指纹表 | 遇 HTTP 601 触发账号熔断（30m 冷却）；网络波动走 biliup 本地断点续传 |

---

## 5. 执行引擎与 Agent 集成契约

### 5.1 biliup CLI 调用契约 (`docs/research/biliup-capabilities.md`)

Go 编排器通过子进程标准方式调用：

```bash
biliup upload \
  --submit client \
  -l AUTO \
  --limit 3 \
  --copyright 2 \
  --tid {category_id} \
  --title "{decision_title}" \
  --desc "{decision_desc}" \
  --tag "{decision_tags}" \
  --cover "{cover_path}" \
  p1.mp4 p2.mp4
```

- **多 P 聚合**: 多个分段视频直接作为位置参数传递，biliup 自动聚合成多 P 稿件。
- **续传保障**: 利用 biliup 内置 lockfile，若传输被杀中断，重试启动自动跳过已上传的分块。

### 5.2 Pi Agent 提示词与 JSON 契约 (`pkg/prompts/decision_agent.go`)

Go 编排器通过无头子进程唤起：

```bash
pi -p --mode json --approve --no-builtin-tools "{constructed_prompt}"
```

- **输入契约 (`VideoDecisionContext`)**: 注入原视频标题、描述、时长、Whisper 摘要及目标策略配置。
- **输出契约 (`VideoDecisionResult`)**:

  ```json
  {
    "title": "80字以内的B站本地化标题",
    "desc": "包含出处声明的结构化简介",
    "tags": ["科技", "AI", "教程"],
    "category_id": 188,
    "part_titles": ["P1 章节名", "P2 章节名"],
    "reasoning": "决策逻辑说明..."
  }
  ```

- **安全前置清洗**: 强制执行 Unicode 80 字截断；内置 `SensitiveWordSanitizer` 自动过滤违禁引流词并替换为等长 `*`。
- **保底降级机制**: 若 Agent 未能在 120s 内返回或 JSON 语法校验失败，执行 `GenerateFallbackDecision`，根据原始元数据套用规则模版，确保工厂无阻断运转。

---

## 6. 前端人机协同与审查控制台 (`docs/prototype/frontend_strategy_and_trace.md`)

- **策略矩阵界面 (`/strategy`)**:
  - 来源频道管理：实时监控 YouTube / Twitch 抓取频次与连通状态。
  - 多对多规则表格：直观编辑频道与 B 站账号绑定、全自动/人工审核标志、优先级排序。
  - 三级熔断面板：包含全局急停开关、账号 601 限流冷却倒计时与手动复位。
- **Agent 决策追溯中心 (`/agent-trace`)**:
  - 全流程透明化：呈现 Agent 完整思考链（Reasoning）、原片与转录上下文。
  - 80 字符合规校验器：实时字数指示器，超出 80 字符标红并禁用发布按钮。
  - 人机审批动作：一键“人工批准并投稿”、“重新决策”、“驳回至死信”。

---

## 7. 分阶段实施路线图 (Implementation Roadmap)

本工程已具备完备的架构与产品规格，推荐按以下四阶段拆分实施：

- **阶段一：数据层与控制台 API 升级** (已完成模型设计与测试)
  - 在 `internal/db/` 固化 `StrategyMatrix` 与 `Guardrail` 服务。
  - 实现 `/api/v1/strategy/*` 与 `/api/v1/agent-trace/*` 接口，对接现有 Next.js 原型页面。
- **阶段二：Pi Agent 阶段门禁集成**
  - 在 `internal/chain_task/handlers/` 中新增 `AgentDecisionHandler`，利用 `pkg/prompts/decision_agent.go` 驱动 `pi` 无头子进程。
  - 接入敏感词过滤与超时自动保底降级逻辑。
- **阶段三：biliup 执行引擎子进程驱动**
  - 在 `internal/chain_task/handlers/` 中接入 `BiliupUploadHandler`，实现命令行参数装配、日志解析与断点续传管理。
  - 接入 HTTP 601 限流拦截，触发 Account 级熔断冷却。
- **阶段四：端到端闭环与自托管联调**
  - 接入多频道定时轮询检查（YouTube / Twitch）。
  - 全链路冒烟自测：从源视频监听、下载切片、Whisper 提取、Agent 决策、压制到 biliup 成功登台发布。
