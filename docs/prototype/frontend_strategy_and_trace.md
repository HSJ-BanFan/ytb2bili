# 原型设计：策略矩阵与 Agent 决策追溯前端界面规范 (Issue #6)

## 1. 背景与设计目标

在 Agent 视频搬运工厂中，系统从“单一视频提交”升级为“自动化搬运管道”。为保障系统的自托管可控性与安全护栏，操作者需要直观的管理控制台：

1. **策略矩阵可视化配置 (`/strategy`)**：
   - 呈现并配置“来源媒体频道（YouTube/Twitch） × B 站投稿账号”的多对多映射网格。
   - 直观管理调度优先级、定时发布延迟、分区 TID、默认标签及“全自动直投 vs 生成草稿待人工审”开关。
   - 提供直观的三级防线熔断开关（Tier 1 全局拉闸、Tier 2 单频道暂停、Tier 3 单账号 601 限流与冷却倒计时）。

2. **Agent 决策全链路追溯与人工审核台 (`/agent-trace`)**：
   - 审查 Pi Agent 在阶段 5（内容决策）基于原视频元数据和 Whisper 字幕提取做出的决策依据。
   - 提供 80 字硬性上限校验的候选标题实时对比与微调。
   - 交互式 B 站简介排版与最多 12 个标签的增删编辑器。
   - 呈现多 P 稿件分段规划。
   - 一键操作：“人工批准并通过投稿”（投递 biliup 管道）、“保存修改”、“重新调用 Agent”、“驳回并隔离至死信”。

---

## 2. 路由与组件拓扑

### 2.1 导航集成 (`web/src/components/layout/AppLayout.tsx`)

在既有系统导航中新增两处入口：

- **策略矩阵**：指向 `/strategy`，使用 `Network` 图标，紧邻“任务队列”。
- **决策追溯**：指向 `/agent-trace`，使用 `Bot` 图标，紫色高亮强调 Agent 属性。

### 2.2 策略矩阵页面 (`web/src/app/strategy/page.tsx`)

包含三大视图面板（Tab 切换）：

1. **策略矩阵规则网格 (`activeTab === 'matrix'`)**：
   - 表格形式展示多对多规则，每一行对应一条 `StrategyRule` (`cw_strategy_rules`)。
   - 包含规则名称、优先级徽章、来源频道（带平台图标）、目标 B 站账号（MID 标识）、发布模式胶囊（全自动直投 / 需人工审核）、分区与标签列表、启用开关与操作。
2. **来源频道管理 (`activeTab === 'channels'`)**：
   - 对应 `SourceChannel` (`cw_source_channels`)。
   - 展示平台（YouTube/Twitch）、频道 ID 与外链、采集模式（点播搬运 / 直播录制）、Cron 轮询周期、上次扫描时间、绑定的规则数量。
   - 支持单个频道的暂停采集与立即手动抓取。
3. **三级熔断防线监控 (`activeTab === 'guardrails'`)**：
   - 顶部提供 **Tier 1 全局急停防线开关**（一键阻断所有后台任务发布）。
   - 中部卡片监控 **Tier 3 账号风控与 HTTP 601 限流熔断器**，展示熔断原因（“上传过快”）、连续失败次数、自动解封倒计时（30 分钟冷却期）及手动重置按钮。
   - 底部展示 **物理幂等防重队列 (`cw_publish_fingerprints`)** 状态大盘（Published、Locked、Pending、DeadLetter 计数）。

### 2.3 Agent 决策追溯台 (`web/src/app/agent-trace/page.tsx`)

采用专业的内容审核双栏（Master-Detail）布局：

1. **左侧：决策记录筛选列表 (5 列宽)**：
   - 快捷状态过滤器：全部、待审核（Pending）、已放行（Auto Approved）、死信隔离（DeadLetter）。
   - 实时搜索栏：按原标题、BV 号或视频 ID 快速定位。
   - 记录卡片：展示频道来源、时长、状态角标、定制标题、源视频 ID 与相对时间戳。
2. **右侧：决策依据与元数据微调面板 (7 列宽)**：
   - **原生视频来源与模型指标**：原标题、时长、频道链接、LLM 模型型号（如 `pi-agent (gpt-4o)`）、执行耗时与 Token 消耗，以及 Whisper 提取的双语摘要。
   - **Agent 推理决策链 (Reasoning Chain)**：数字序号分步展开 Agent 思考逻辑（原标题 Clickbait 过滤、B 站分区调性匹配、违禁词初筛检查、分 P 划分依据）。若进入死信，以醒目红框展示不可重试原因。
   - **B 站投稿最终标题编辑器**：实时字符计数器（如 `34 / 80 字`），超过 80 字符边框变红并禁用发布按钮。
   - **候选标题备选库**：Agent 产出的备选标题列表，点击任一候选一键替换主标题。
   - **动态简介与标签编辑器**：多行格式化简介文本框；交互式标签胶囊，支持回车添加与点击删除，硬性限制不超过 12 个标签。
   - **多 P 分段命名展示**：分 P 结构、分段标题与时长清单。
   - **人工复审动作栏**：
     - `驳回至死信 (Reject)`：标记为 `deadletter`，防止反复重试浪费资源与账号风控。
     - `重调决策 (RotateCcw)`：重新唤起后台 Pi Agent 重跑决策步骤。
     - `人工批准并通过投稿 (Send)`：一键写入审核结果并唤起 biliup 投稿。

---

## 3. 与后端模型与状态机的契约对齐

| 前端概念 | 对应后端 GORM 模型 / 表 | 对应流水线阶段 / 状态机 |
| :--- | :--- | :--- |
| 来源频道 | `model.SourceChannel` (`cw_source_channels`) | 阶段 1：`source_fetch` 触发源 |
| 策略规则 | `model.StrategyRule` (`cw_strategy_rules`) | 矩阵路由决策，决定 `auto_publish` 属性 |
| 全局/账号熔断 | `model.SystemGuardrail` (`cw_system_guardrails`) | 投稿防线三级拦截器（Tier 1/2/3） |
| 幂等防重大盘 | `model.PublishFingerprint` (`cw_publish_fingerprints`) | 物理排队锁、BV号回填与死信状态追踪 |
| 决策追溯记录 | `TaskStep` (`content_decision`) / `AuditLog` | 阶段 5：`content_decision` |
| 待人工审核 | `FingerprintStatusPending` + `AutoPublish=false` | 状态机 `pending_review` 等待人工操作 |
| 批准放行 | 提交至 `bili_upload` | 阶段 7：调用 `biliup upload` 启动分块并发传输 |
| 死信隔离 | `FingerprintStatusDeadLetter` | 终态故障隔离，禁止自动重试 |

---

## 4. 验证与交付状态

- 所有前端代码遵循 Next.js 15 App Router 与 React 18 规范。
- 采用 Tailwind CSS 纯函数式类名，完全复用既有系统色调体系。
- 经由语言服务器与类型检查器静态验证，TypeScript 编译通过（0 错误，0 警告）。
- 详见代码实现：
  - `web/src/components/layout/AppLayout.tsx`
  - `web/src/app/strategy/page.tsx`
  - `web/src/app/agent-trace/page.tsx`
