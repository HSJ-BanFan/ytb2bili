# Specification: Stage-Gated Pipeline State Machine & Failure Recovery

> Context: [Issue #5](https://github.com/HSJ-BanFan/ytb2bili/issues/5) | Parent Map: [Issue #1](https://github.com/HSJ-BanFan/ytb2bili/issues/1)  
> Domain References: `CONTEXT.md`, `docs/research/biliup-capabilities.md`, `docs/research/pi-agent-orchestration.md`

---

## 1. 概述与架构意图 (Overview & Architectural Intent)

本规范为 **Agent 视频搬运工厂**（以 Go 编排器为控制中枢、`biliup` 为录制与投稿引擎、`Pi Agent` 为受控内容决策大脑）定义确定性阶段门禁（Stage-Gated）流水线的状态机流转模型、故障分类分级体系、退避重试策略以及死信队列（Dead-Letter Queue, DLQ）隔离规则。

### 核心架构原则

1. **身手脑清晰分工**：
   - **Go 编排守护进程（身躯）**：负责全局持久化、状态机原子步进、Worker 池并发配额、超时熔断与重试调度。
   - **biliup 执行引擎（手足）**：负责原始媒体流的高性能拉取（`stream-gears`/`yt-dlp`）与 Bilibili UPOS 分块并发直传（`biliup upload`）。
   - **Pi Agent 决策层（头脑）**：在阶段门禁约束下以无头子进程运行，基于受限沙箱与严格 JSON Schema 输出定制化 B 站标题、简介、标签及分 P 结构。
2. **确定性投稿防线**：坚决禁止将不确定的 LLM 概率决策直接绑定在发布动作上。状态机通过幂等锁、去重指纹、熔断开关和结构化校验提供硬性护栏。
3. **分层状态解耦**：将系统状态划分为 **稿件全局生命周期状态（Video Status）** 与 **单个处理阶段状态（TaskStep Status）**，保证粒度细化的重试与断点续传。

---

## 2. 状态机模型定义 (State Models & Lifecycle)

系统维护两级状态实体：

- **`SavedVideo.Status`**：宏观稿件生命周期状态（继承并规范现存三位状态码）。
- **`TaskStep.Status`**：微观单个流水线步骤状态机。

### 2.1 稿件生命周期状态 (`SavedVideo.Status`)

```text
      [001 待处理]
           │
           ▼ (获取 worker 槽位 & 原子 CAS 更新)
      [002 处理中] ───────────────────────────────────┐ (遇致命错误/死信)
           │                                          │
           ▼ (阶段 1~6 成功完成)                       ▼
      [200 准备就绪]                            [999 任务失败] (进入死信隔离)
           │                                          ▲
           ▼ (调度器获取上传锁)                        │
      [300 投稿发布中] ───────────────────────────────┘ (投稿终态失败)
           │
           ▼ (投稿成功并确认 BVID)
      [400 处理完成] (归档，进入生命周期自动清理)

      [998 已取消] (任何非终态均可由人工或全局熔断置入)
```

| 状态码 | 常量名 | 语义定义 | 允许的后续状态 | 触发条件 |
| :--- | :--- | :--- | :--- | :--- |
| `001` | `VideoStatusPending` | 待处理 | `002`, `998` | 初始创建、或服务重启恢复 |
| `002` | `VideoStatusProcessing` | 正在执行处理流水线 | `200`, `999`, `998` | Worker 抢占 slot 并成功将 DB 更新为 002 |
| `200` | `VideoStatusReady` | 本地处理完毕，视频准备就绪 | `300`, `998` | 阶段 1~6（获取到压制）全部成功完成 |
| `300` | `VideoStatusUploading` | 正在向 Bilibili 投稿 | `400`, `200` (重试), `999` | `UploadScheduler` 获取账号锁并调起 `biliup` |
| `400` | `VideoStatusCompleted` | 投稿成功并获取到 BVID | 终态（只读归档） | biliup 提交成功返回 aid/bvid |
| `998` | `VideoStatusCanceled` | 用户手动取消或熔断取消 | `001` (人工重置) | 用户取消、策略矩阵禁用频道 |
| `999` | `VideoStatusFailed` | 终态失败 / 死信隔离 | `001` (人工干预修复后) | 必需步骤超过最大重试次数或遭遇不可恢复错误 |

### 2.2 流水线步骤状态机 (`TaskStep.Status`)

每个视频任务由有序的 `TaskStep` 序列组成，每个步骤独立跟踪自身状态。

```text
              ┌────────────────────────────────────────────────────────┐
              │                                                        │
              ▼                                                        │
         [waiting]                                                     │
              │                                                        │
              ▼ (前置步骤完成)                                         │
         [pending] ◄────────────────────────────────────┐             │
              │                                          │             │
              ▼ (获取执行槽位)                            │ (可重试错误) │
         [running] ──────────────────────────────────────┤             │
         │   │   │                                       │ (到达退避时间)
         │   │   └───────────────┐                       │             │
         │   ▼ (处理成功)         ▼ (无需处理/降级跳过)   │             │
         │  [completed]          [skipped]               ▼ (超过重试上限)
         │                                         [failed_permanent]
         ▼ (致命故障/不可恢复)                               (死信隔离)
      [failed] (can_retry=false) ─────────────────────────────┘
```

| 步骤状态 | 对应代码常量 | 描述 |
| :--- | :--- | :--- |
| `waiting` | `TaskStepStatusWaiting` | 前置依赖步骤尚未完成，静默等待。 |
| `pending` | `TaskStepStatusPending` | 前置步骤已就绪，或失败退避时间已到，等待被调度执行。 |
| `running` | `TaskStepStatusRunning` | 步骤正在 Goroutine 或子进程中运行。 |
| `completed` | `TaskStepStatusCompleted` | 步骤执行成功，产物已校验并持久化。 |
| `skipped` | `TaskStepStatusSkipped` | 策略条件不满足或触发安全降级，该步骤被跳过且不影响主链继续。 |
| `failed` | `TaskStepStatusFailed` | 步骤遭遇错误，若 `can_retry=true` 则等待退避后转回 `pending`。 |
| `failed_permanent` | `TaskStepStatusFailedPermanent` | 重试次数超限或遭遇终态致命错误，停止自动重试，转入死信状态。 |

---

## 3. 阶段门禁流水线详述 (Stage-Gated Pipeline Steps)

流水线标准阶段分为严格有序的 7 个步骤：

```text
[1. Source Fetch] ──> [2. Audio Extract] ──> [3. Whisper] ──> [4. Translate]
                                                                    │
[7. Biliup Upload] <── [6. Remux] <── [5. Pi Agent Decision] <─────┘
```

---

### 阶段 1: 来源媒体获取 (`source_fetch`)

- **依赖前置**：无（视频记录进入 `002` 即触发）
- **必需程度**：**必需 (`Required = true`)**
- **执行方式**：Go 编排器驱动 `biliup download`（直播会话）或 `yt-dlp`（点播频道搬运）子进程。
- **输入产物**：来源 URL、代理配置、频道策略（分辨率上限、格式首选）。
- **输出产物**：`{videoID}.mp4`（原始视频）、`raw_meta.json`（原始平台元数据）、`cover.jpg`（原始封面）。
- **故障分类**：
  - *瞬态故障 (Transient)*：网络连接超时、代理连接重置、YouTube 429 速率限制、流媒体分块下载短暂断流。
  - *终态故障 (Terminal)*：视频被作者删除 (404)、视频设为私密/会员专享 (Private/Members Only)、地理版权屏蔽 (Geo-blocked 403)、URL 非法。
- **重试策略**：
  - 最大重试次数：**5 次**
  - 退避策略：指数退避带抖动，初始退避 10 秒，最大退避 300 秒（10s, 30s, 60s, 120s, 300s）。
- **降级路径**：无（无原始媒体则整链无法继续）。
- **死信条件**：检测到 404/Private/Copyright 关键字，或 5 次重试后仍无法拉取完整媒体。

---

### 阶段 2: 音频提取 (`audio_extract`)

- **依赖前置**：`source_fetch` (`completed`)
- **必需程度**：**必需 (`Required = true`)**
- **执行方式**：Go 调用本地 `ffmpeg` 二进制命令。
- **输入产物**：`{videoID}.mp4`
- **输出产物**：`{videoID}.wav`（16kHz, 单声道 16-bit PCM，最适 Whisper 识别）。
- **故障分类**：
  - *瞬态故障 (Transient)*：本地磁盘写入满 (ENOSPC)、系统线程资源耗尽。
  - *终态故障 (Terminal)*：原始视频文件损坏（Moov atom not found、Trun/Trak 解码致命错误）、视频无音频流（纯静音轨）。
- **重试策略**：
  - 最大重试次数：**2 次**
  - 退避策略：线性退避 5 秒。
- **降级路径**：
  - 若输入视频确系无声视频（通过 `ffprobe` 检测音频流为 0），将 `audio_extract`、`whisper_transcribe`、`subtitle_translate` 统一标记为 `skipped`，直接跳向 `agent_decision`。
- **死信条件**：视频文件损坏且 `ffmpeg` 无法修复。

---

### 阶段 3: 语音转写识别 (`whisper_transcribe`)

- **依赖前置**：`audio_extract` (`completed`)
- **必需程度**：**可选 (`Required = false`)**
- **执行方式**：Go 内部 Whisper 客户端调用本地 `whisper.cpp`/GPU 服务，或配置的 OpenAI-compatible Whisper API。
- **输入产物**：`{videoID}.wav`
- **输出产物**：`raw_transcript.json`、`source.srt`（原始语言时间戳字幕文件）、检测出的来源语言代码（`source_lang`）。
- **故障分类**：
  - *瞬态故障 (Transient)*：GPU 显存瞬时溢出 (CUDA OOM)、Whisper API 网关 502/504、单次 HTTP 响应超时。
  - *终态故障 (Terminal)*：音频格式不支持、Whisper 服务配置错误（非法模型名称/Token）。
- **重试策略**：
  - 最大重试次数：**3 次**
  - 退避策略：指数退避，初始 15 秒，最大 120 秒。
- **降级路径**：
  - **降级 1**：若原始平台自带外挂字幕（如 YouTube CC 字幕），降级提取平台原文字幕作为 `source.srt`。
  - **降级 2**：若既无 CC 字幕且 Whisper 3 次重试失败，步骤置为 `skipped`，降级为无字幕稿件继续往下流转，不阻断发稿。
- **死信条件**：仅在显式配置为“必须有字幕才发稿”且降级路径均失效时才置为死信。

---

### 阶段 4: 字幕本地化翻译 (`subtitle_translate`)

- **依赖前置**：`whisper_transcribe` (`completed`)
- **必需程度**：**可选 (`Required = false`)**
- **执行方式**：Go 调用百度翻译、DeepSeek、Gemini 或兼容 LLM 接口分批并发翻译。
- **输入产物**：`source.srt`、`source_lang`
- **输出产物**：`zh.srt`（双语对齐或纯中文中文字幕文件）。
- **故障分类**：
  - *瞬态故障 (Transient)*：第三方翻译平台 HTTP 429（超频限流）、网络短暂丢包、单批次超时。
  - *终态故障 (Terminal)*：翻译账户余额耗尽（Baidu 54004 / OpenAI 402 Insufficient Quota）、API Key 失效/封禁。
- **重试策略**：
  - 最大重试次数：**3 次**
  - 退避策略：指数退避 10s, 30s, 60s。
- **降级路径**：
  - **免处理跳过**：若 `source_lang` 本身已是中文（`zh`/`yue`），直接复制为 `zh.srt`，步骤标记为 `skipped`。
  - **优雅降级**：若翻译配额耗尽或连续重试失败，步骤标记为 `skipped`，记录降级审计日志，使用原始语言字幕或不压制中文字幕继续流转。
- **死信条件**：无（非必需步骤，失败一律降级跳过）。

---

### 阶段 5: Pi Agent 内容决策 (`agent_decision`)

- **依赖前置**：`source_fetch` (`completed`), `subtitle_translate` (`completed` 或 `skipped`)
- **必需程度**：**必需 (`Required = true`)**（必须产生符合 B 站规范的元数据）
- **执行方式**：Go 编排器调起 `pi -p --mode json` 无头子进程，传入严格 JSON 上下文与 Schema 提示词。
- **输入产物**：
  - 原始元数据（`raw_meta.json`：原标题、原简介、原作者、时长、播放量）。
  - 字幕文本摘要（提取 `zh.srt` 或 `source.srt` 关键段落）。
  - 策略矩阵配置规则（该频道的标题前缀要求、版权声明、目标分区 `tid`、默认标签列表）。
- **输出产物**：`decision.json`

  ```json
  {
    "title": "符合B站风格且<=80字符的标题",
    "desc": "本地化结构化简介\n\n原作者信息与授权声明",
    "tags": ["核心标签1", "标签2", "最多12个标签"],
    "tid": 174,
    "dynamic": "动态推送文案",
    "part_titles": ["P1 章节名", "P2 章节名"]
  }
  ```

- **故障分类**：
  - *瞬态故障 (Transient)*：LLM 服务 503/429、子进程标准流偶发超时（默认超时 120 秒）。
  - *终态故障 (Terminal)*：Agent 生成内容触发敏感词违规拒答、LLM API Key 彻底无效。
  - *契约异常 (Schema Mismatch)*：输出未按约定 JSON 格式返回，缺少 `title` 必填字段。
- **重试策略**：
  - 最大重试次数：**2 次**
  - 退避策略：指数退避 5 秒、15 秒。重试时自动在 Prompt 尾部追加修复约束指令。
- **降级路径（确定性规则保底）**：
  - 若 2 次 Agent 决策均失败或输出非法 JSON，**触发确定性降级模版引擎**：
    - `title`：截取原始标题前 80 字符（清洗常见特殊字符与表情包）。
    - `desc`：使用策略矩阵预设的静态简介模板 + 自动拼接原始源链接。
    - `tags`：使用策略矩阵为当前频道预设的静态标签池。
    - `tid`：使用当前频道策略设定的默认分区 ID。
  - 将步骤标记为 `completed`（附加 `degraded: true` 标记及审计警告）。
- **死信条件**：降级模板自身配置缺失且无法生成任何合法标题。

---

### 阶段 6: 视频封装与压制 (`video_remux`)

- **依赖前置**：`source_fetch` (`completed`), `agent_decision` (`completed`)
- **必需程度**：**必需 (`Required = true`)**
- **执行方式**：Go 调用 `ffmpeg` 执行流重封装、PTS 修复、可选硬字幕烧录或软字幕流合并、MP4 FastStart 优化。
- **输入产物**：`{videoID}.mp4`、`zh.srt`（若存在且策略要求烧录）、`decision.json`。
- **输出产物**：
  - 单 P 场景：`dist_{videoID}_p1.mp4`（经 PTS 修复和 faststart 处理后的可发布标准 MP4）。
  - 多 P 场景：`dist_{videoID}_p1.mp4`, `dist_{videoID}_p2.mp4`...（按章节或切片规则切分）。
- **故障分类**：
  - *瞬态故障 (Transient)*：磁盘空间临时报警。
  - *终态故障 (Terminal)*：编码格式与 B 站支持规格不兼容、视频 PTS 时间戳错乱无法重构。
- **重试策略**：
  - 最大重试次数：**2 次**
  - 退避策略：线性退避 10 秒。
- **降级路径**：
  - 若字幕烧录（Burn-in Subtitles）编码失败，降级为跳过烧录，仅做流拷贝与 remux，字幕转为独立投稿附件（走 B 站外挂字幕 API）。
- **死信条件**：`ffmpeg` 报不可恢复的致命编码错误。

---

### 阶段 7: Biliup 投稿发布 (`biliup_upload`)

- **依赖前置**：`video_remux` (`completed`), `agent_decision` (`completed`)
- **必需程度**：**必需 (`Required = true`)**
- **执行方式**：由独立的 `UploadScheduler` 定时触发，获取账号排队锁后，以标准子进程执行 `biliup upload`。
- **输入产物**：
  - 最终视频分 P 列表：`dist_{videoID}_p*.mp4`
  - 投稿元数据参数：`--title`, `--desc`, `--tag`, `--tid`, `--cover`, `--dtime`, `--copyright 2`, 账号 Cookies。
- **输出产物**：B 站稿件响应（`aid`, `bvid`），写入 `cw_saved_videos`。
- **故障分类**：
  - *瞬态故障 (Transient)*：
    - B 站网关限流 HTTP 601（"您上传视频过快，请稍作休息"）。
    - UPOS CDN 上传节点网络超时 / 断开。
    - B 站服务器返回 500/502/504 临时错误。
  - *终态故障 (Terminal)*：
    - 投稿账号 Cookies 失效、SESSDATA 过期、账号被 B 站封禁或未实名认证。
    - 标题包含平台绝对违禁词被审核接口拦截（Bilibili Code 21000~21100）。
    - 视频封面图片格式超大或违规。
- **重试策略**：
  - 遵循 biliup 原生断点恢复与智能退避机制（PR #1558）：
    - 遇到 HTTP 601：按 **1 分钟 -> 2 分钟 -> 3 分钟 -> 4 分钟** 递增等待，最多重试 5 次。
    - UPOS 分块失败：分块级原地重试 3 次，无需重新上传整个大视频。
  - Go 编排调度器级别重试：最多调度重试 **3 轮**。
- **降级路径**：若指定的海外线路 (`txa`/`alia`) 失败，自动切换为 `AUTO` 进行全网动态 CDN 探测重试。
- **死信条件**：账号凭据失效 (Cookie Expired)、内容审核硬性驳回、超过最大重试轮次。此时视频置为 `999`，发出告警通知。

---

## 4. 故障分类分级与死信机制 (Failure Classification & DLQ)

### 4.1 故障类型矩阵表

| 故障类别 | 典型错误特征 | 判定规则 | 处理策略 | 目标状态 |
| :--- | :--- | :--- | :--- | :--- |
| **网络瞬态抖动** | `connection reset`, `timeout`, `i/o timeout` | 正则匹配常见网络断连字符串 | 增量指数退避，原地或下次调度重试 | `failed` (`can_retry=true`) |
| **外部频控限流** | HTTP 429, Bilibili 601 | HTTP 状态码 429 / 错误码 601 | 专属退避时长（1~5分钟），锁定对应账号/源通道 | `pending` (带锁定冷却时间) |
| **依赖配额耗尽** | OpenAI 402, Baidu 54004, Disk Full | 第三方 API 配额错误码 | 激活降级策略（跳过字幕/使用模版）或触发系统告警 | `skipped` 或全局挂起 |
| **契约格式错误** | Pi Agent 输出非标准 JSON, 字段缺失 | `json.Unmarshal` 失败 | 触发提示词自愈重试 1 次，仍失败走确定性模版降级 | `completed` (`degraded=true`) |
| **账号与权限终态** | Cookie 过期, 账号封禁, 密码错误 | Bilibili -101 / 未登录 | 立即终止重试，锁定该投稿账号，阻断后续任务 | `failed_permanent` -> `999` |
| **内容违规阻断** | 标题/简介包含敏感词, 视频版权红线 | 平台特定业务错误码 | 记录审计原因，绝不盲目重试，等待人工审查 | `failed_permanent` -> `999` |

### 4.2 退避算法标准公式 (Exponential Backoff with Jitter)

为避免大量失败任务在同一时刻重试造成“惊群效应”打崩网络或触发风控，重试等待时间采用 **带全抖动的指数退避（Full Jitter Exponential Backoff）**：

$$\text{Sleep} = \text{random}(0, \; \min(\text{MaxBackoff}, \; \text{BaseBackoff} \times 2^{\text{RetryCount}}))$$

- **通用阶段参数**：$\text{BaseBackoff} = 10\text{s}$, $\text{MaxBackoff} = 300\text{s}$。
- **投稿发布阶段参数 (针对 601)**：$\text{BaseBackoff} = 60\text{s}$, $\text{MaxBackoff} = 600\text{s}$。

### 4.3 死信隔离与人工干预 (Dead-Letter Quarantine & Operator Triage)

当满足以下任一条件时，任务步骤转移至 `failed_permanent`，主任务置为 `999`，进入死信隔离区：

1. 步骤重试计数达到上限（默认必需步骤 3~5 次，或配置上限）。
2. 捕获到明确的“终态不可恢复”错误码（账号无效、视频不存在、硬性审核违规）。
3. 视频元数据与降级模版均完全不可用。

**死信处理规范**：

- **安全隔离**：进入死信的任务从每 5 秒的自动轮询调度队列中永久排除，不消耗 Worker 槽位。
- **现场保存**：保留当前的 `cw_task_steps` 步骤执行记录、`result_data`、`error_msg` 以及已经生成在磁盘上的中间分段文件（不被即刻清理），以便排查。
- **人工干预救赎流**：
  1. 操作员在 Web 控制台修复问题（如更新失效的 B 站 Cookie、修改违规标题）。
  2. 调用 `/api/v1/videos/:id/retry` 接口。
  3. 后端执行原子操作：重置该任务失败步骤的 `RetryCount = 0`, `Status = 'pending'`, `CanRetry = true`，并将主状态重置回 `001`，无缝重新加入调度链。

---

## 5. 并发控制、原子性与崩溃恢复 (Concurrency & Crash Recovery)

### 5.1 三池隔离并发模型 (Three-Pool Concurrency Model)

为防止单一瓶颈（如大文件下载把带宽占满，或 Whisper 把 CPU/GPU 占满导致接口卡死），系统在 Go 进程内划分三层独立并发池：

```text
                    ┌────────────────────────┐
                    │ 全局任务分发器 (Cron)   │
                    └───────────┬────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        ▼                       ▼                       ▼
┌──────────────┐        ┌──────────────┐        ┌──────────────┐
│ 下载并发池   │        │ 计算/Agent池 │        │ 投稿调度器   │
│ maxDownloads │        │ maxWorkers   │        │ (串行账号锁) │
│ (默认: 3)    │        │ (默认: 10)   │        │ (1 任务/账号)│
└──────────────┘        └──────────────┘        └──────────────┘
```

1. **下载专用池 (`downloadWorkerPool`)**：限制外部网络高吞吐操作并发度（默认 3），防止触发 YouTube 429 限流或击穿本地带宽。
2. **综合处理池 (`workerPool`)**：用于音频分离、Whisper、翻译、Agent 推理与 Remux（默认 10）。
3. **投稿串行细粒度锁 (`UploadScheduler with Lock`)**：
   - 同一 Bilibili 投稿账号在同一时间内**必须严格串行上传**，杜绝并发调用触发 B 站 HTTP 601 连环封禁。
   - 不同投稿账号之间支持并发上传。

### 5.2 守护进程崩溃恢复规范 (Crash Recovery Invariant)

系统可能因宿主机断电、更新重启或 OOM 突然终止。必须保证重启后状态自愈：

1. **启动自检与重置 (`resetRunningTasksOnStartup`)**：
   - 守护进程启动时，在事务中扫描全部 `Status == 'running'` 的 `cw_task_steps`。
   - 将其重置为 `TaskStepStatusPending`（`start_time` 清空，保留已有的 `RetryCount`）。
   - 将关联的 `cw_saved_videos` 从 `002`（处理中）原子重置为 `001`（待处理）。
2. **幂等性与产物复用 (Artifact Reuse)**：
   - 每个 Handler 在执行具体工作前，先检查目标产物文件是否存在且大小合法（例如 `StateManager.InputVideoPath` 已存在且大小 > 0，或 `zh.srt` 已存在）。
   - 若产物已有效存在，Handler 直接复用本地文件并更新状态为 `completed`，跳过重复计算与下载，实现近乎实时的断点恢复。

---

## 6. 数据模型强化建议 (Schema Enhancements)

为支撑上述退避与死信机制，建议对 `cw_task_steps` 表结构进行如下向后兼容扩展：

```sql
ALTER TABLE cw_task_steps ADD COLUMN next_retry_at DATETIME DEFAULT NULL;      -- 下次允许重试的最早时间（用于退避调度）
ALTER TABLE cw_task_steps ADD COLUMN failure_type VARCHAR(32) DEFAULT NULL;   -- 故障分类: transient, terminal, quota, schema
ALTER TABLE cw_task_steps ADD COLUMN is_degraded BOOLEAN DEFAULT FALSE;       -- 是否属于降级完成状态
```

Go GORM 模型定义扩展：

```go
type TaskStep struct {
    model.BaseModel
    VideoID       string     `gorm:"type:varchar(100);not null;index" json:"video_id"`
    StepName      string     `gorm:"type:varchar(100);not null" json:"step_name"`
    StepOrder     int        `gorm:"type:int;not null" json:"step_order"`
    Status        string     `gorm:"type:varchar(20);not null;index" json:"status"`
    StartTime     *time.Time `gorm:"type:datetime" json:"start_time"`
    EndTime       *time.Time `gorm:"type:datetime" json:"end_time"`
    Duration      int64      `gorm:"type:bigint" json:"duration"`
    ErrorMsg      string     `gorm:"type:text" json:"error_msg"`
    ResultData    string     `gorm:"type:longtext" json:"result_data"`
    RetryCount    int        `gorm:"type:int;default:0" json:"retry_count"`
    CanRetry      bool       `gorm:"type:boolean;default:true" json:"can_retry"`
    NextRetryAt   *time.Time `gorm:"type:datetime;index" json:"next_retry_at"`   // 退避调度时间戳
    FailureType   string     `gorm:"type:varchar(32)" json:"failure_type"`       // 故障类型标签
    IsDegraded    bool       `gorm:"type:boolean;default:false" json:"is_degraded"` // 降级标记
}
```

---

## 7. 结论与验收指标 (Acceptance Criteria for Issue #5)

1. **状态机全生命周期闭环**：`Source Fetch -> Audio Extract -> Whisper -> Translate -> Pi Agent Decision -> Remux -> Biliup Upload` 7 个阶段均具备严格的前置约束、输入输出定义与完成条件。
2. **故障隔离完备**：清晰划分瞬态、终态、降级与死信。非必需步骤（Whisper/翻译）遇到不可恢复错误时优雅降级；必需步骤遇到不可恢复错误时安全转移至死信区（`999` / `failed_permanent`），不再无意义空转。
3. **退避算法明确**：通用步骤实施带抖动指数退避，针对 B 站 601 限流实施 1~5 分钟分级惩罚式退避。
4. **自愈与断点能力**：支持进程崩溃重启后通过文件检验直接跳过已完成步骤，无需从头下载或转码。
