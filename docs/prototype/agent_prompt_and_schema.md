# Specification: Stage-5 Content Decision Agent Prompt & Schema Contract

> **议题上下文**：Issue #7 (`wayfinder:prototype`)  
> **所属阶段**：流水线第 5 阶段：Pi Agent 内容决策 (`agent_decision`)  
> **代码实现**：
>
> - 核心契约与降级引擎：`pkg/prompts/decision_agent.go`
> - 自动化测试套件：`internal/prototype/agent_decision_test.go`
> - 关联模型：`pkg/store/model/strategy_matrix.go` (`StrategyRule`, `SourceChannel`)

---

## 1. 架构定位与职责边界

在 Agent 视频搬运工厂的 7 阶段门禁架构中，**阶段 5（Pi Agent 内容决策）**是确定性音视频处理（下载、提取、Whisper、翻译）与最终发布之间的**内容智能化中枢**。

```text
[1. Source Fetch] ──> [2. Audio Extract] ──> [3. Whisper] ──> [4. Translate]
                                                                        │
[7. Biliup Upload] <── [6. Remux] <── [5. Pi Agent Decision] <─────────┘
                                      ├── 输入: 原始元数据 + 字幕摘要 + 策略规则
                                      ├── 约束: JSON Schema + B站发布规范
                                      ├── 安全: 敏感词清洗 + 80字符截断
                                      └── 保底: 确定性降级引擎 (Fallback)
```

### 1.1 核心设计原则 (Ponytail Rules)

1. **轻量短会话**：Go 编排器以 `pi -p --mode json` 方式启动无头子进程，仅传入纯文本上下文，剥离 bash 与文件工具（`--no-builtin-tools`），杜绝外部副作用。
2. **严格结构化**：通过系统提示词与输入 Schema 强约束，要求 Agent 仅输出单一纯 JSON 对象，严禁闲聊。
3. **确定性防线**：不将发布合规寄托于大模型的概率输出。Go 编排器在反序列化后执行二次硬性校验：
   - 标题 UTF-8 字符数严格裁剪至 80 字符以内；
   - 标签数组强制清洗、去重并截断至最多 12 个；
   - 违禁词（翻墙、涉赌、色情、引流等）通过正则词库自动打码并移除违规标签；
   - 发生超时（默认 120s）或格式彻底错乱时，自动触发确定性规则降级引擎（`GenerateFallbackDecision`），保障工作流绝不挂死。

---

## 2. 输入上下文规范：`VideoDecisionContext`

Go 编排器在调用 Pi Agent 前，聚合阶段 1-4 的产物及策略矩阵规则，构筑完整的输入上下文：

```go
type VideoDecisionContext struct {
    // 原始媒体元数据 (阶段 1 raw_meta.json 提取)
    SourcePlatform  string   `json:"source_platform"`           // youtube, twitch
    SourceVideoID   string   `json:"source_video_id"`            // 原视频唯一 ID (如 dQw4w9WgXcQ)
    OriginalTitle   string   `json:"original_title"`            // 原始视频标题
    OriginalDesc    string   `json:"original_desc,omitempty"`   // 原始视频简介
    OriginalChannel string   `json:"original_channel"`          // 来源频道名称
    OriginalAuthor  string   `json:"original_author,omitempty"` // 原始作者
    DurationSeconds int      `json:"duration_seconds"`          // 视频时长（秒）
    OriginalTags    []string `json:"original_tags,omitempty"`   // 原平台标签
    SourceURL       string   `json:"source_url,omitempty"`      // 原视频来源 URL

    // 字幕与内容摘要 (阶段 3/4 产物)
    TranscriptSummary string `json:"transcript_summary,omitempty"` // 提取出的字幕核心要点/前缀摘要
    SourceLanguage    string `json:"source_language,omitempty"`    // 识别出的语种代码 (en, ja 等)
    HasSubtitles      bool   `json:"has_subtitles"`              // 是否就绪中文字幕

    // 策略矩阵与目标账号要求 (StrategyRule 注入)
    TargetAccountName    string   `json:"target_account_name,omitempty"`    // 目标 B 站投稿账号
    TargetAccountPersona string   `json:"target_account_persona,omitempty"` // 账号运营人设风格
    DynamicTitleTemplate string   `json:"dynamic_title_template,omitempty"` // 标题生成偏好/修饰前缀
    DescTemplate         string   `json:"desc_template,omitempty"`          // 简介版权与补充说明模板
    DefaultTags          []string `json:"default_tags,omitempty"`           // 策略设定的必填/缺省标签池
    CategoryID           int      `json:"category_id,omitempty"`            // 目标分区 TID
    Copyright            int      `json:"copyright,omitempty"`              // 投稿类型: 1=自制, 2=转载
    SourceOrigin         string   `json:"source_origin,omitempty"`          // 转载出处说明
    AutoPublish          bool     `json:"auto_publish"`                     // 是否全自动发布
}
```

---

## 3. 提示词工程架构 (Prompt Engineering)

系统提示词采用“角色人设 + 硬性平台规范 + 输出契约”三段式设计。

### 3.1 系统提示词模板 (`BuildDecisionSystemPrompt`)

根据策略中配置的 `TargetAccountPersona` 动态调整运营人设，例如：

- 默认人设：“你是一位精通 Bilibili 社区生态、深谙年轻观众偏好与平台审核规则的资深内容运营专家。”
- 科技账号人设：“你是一位专精于 Bilibili 社区生态的内容运营专家，本次执行的运营人设风格为【严谨硬核的科技数码极客】。”
- 游戏切片人设：“你是一位专精于 Bilibili 社区生态的内容运营专家，本次执行的运营人设风格为【轻松幽默的高能游戏实况解说】。”

系统提示词中固化的硬性规则：

1. **标题 (title)**：UTF-8 字符长度严格限制在 1-80 之间，推荐 20-45 字符，禁止低俗标题党，优先融入策略模板前缀。
2. **简介 (desc)**：结构化呈现，必须包含中文核心看点、原作者信息与出处链接、版权与免责声明。
3. **标签 (tags)**：3-12 个核心标签，单标签 <= 20 字符，严禁空格与特殊符号，必须融合策略默认标签。
4. **分区 (tid)**：优先采用策略指定的 category_id。
5. **合规红线**：严禁出现翻墙/梯子/博彩/色情/微商引流等违禁内容。
6. **输出格式**：仅输出单个纯 JSON 对象，不要附加代码块外的任何闲聊。

### 3.2 用户提示词模板 (`BuildDecisionUserPrompt`)

清晰组织 Markdown 分块，展示原视频信息、时长格式化（`MM:SS`）、字幕摘要截断、策略发布约束，并在文末嵌入完整的 JSON Schema 规范定义。

---

## 4. 输出契约：JSON Schema 规范

Stage-5 决策 Agent 输出必须严格匹配如下 Draft-07 JSON Schema：

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "VideoDecisionResult",
  "type": "object",
  "required": ["title", "desc", "tags", "tid"],
  "properties": {
    "title": {
      "type": "string",
      "description": "Bilibili 视频标题，UTF-8 字符长度必须介于 1 到 80 个字符之间",
      "minLength": 1,
      "maxLength": 80
    },
    "desc": {
      "type": "string",
      "description": "Bilibili 视频简介，包含中文核心看点与原作者版权声明",
      "minLength": 1
    },
    "tags": {
      "type": "array",
      "description": "视频标签数组，数量必须在 1 到 12 个之间，单标签不超过 20 字符",
      "items": { "type": "string" },
      "minItems": 1,
      "maxItems": 12
    },
    "tid": {
      "type": "integer",
      "description": "Bilibili 分区 TID，必须为正整数",
      "minimum": 1
    },
    "dynamic": {
      "type": "string",
      "description": "Bilibili 动态发布文案（可选）"
    },
    "part_titles": {
      "type": "array",
      "description": "多 P 稿件各分段小标题（可选）",
      "items": { "type": "string" }
    },
    "cover_prompt": {
      "type": "string",
      "description": "封面视觉要点建议（可选）"
    },
    "reasoning": {
      "type": "string",
      "description": "Agent 决策思考过程与重点依据（可选）"
    }
  }
}
```

---

## 5. 校验、清洗与合规防线实现

Go 编排层在接收到 Agent 输出后，通过 `ValidateAndNormalizeDecision` 管道执行多道确定性过滤：

### 5.1 Markdown 包裹剥离 (`CleanJSONString`)

部分 LLM 习惯输出带有 ````json ...```` 的 Markdown 标记。解析器首先提取最内层的合法 JSON 文本，避免由于反引号导致的反序列化报错。

### 5.2 字符精准截断 (`TruncateRunes`)

在 Go 语言中，一个中文字符在 `len()`（字节长度）中占 3 个字节，而在 B 站的 80 字标题限制中，**1 个中文字符与 1 个英文字符均计为 1 个字符**。  
因此实现采用基于 `[]rune` 的 Unicode 字符计量方式：

- 若字符数超过 80，使用 `TruncateRunes(res.Title, 80)` 精确切断；
- 并在 `Warnings` 列表中添加审计警告：`"标题长度(X字)超出 B 站上限(80字)，已自动截断至 80 字符"`。

### 5.3 标签规范化与上限约束 (`normalizeTags`)

- **清洗**：去除标签内的所有空格、半角逗号、全角逗号；
- **单标签截断**：单个标签字符数上限为 20（`MaxTagRuneLength`）；
- **大小写去重**：使用 `map[string]struct{}` 按照全小写去重，避免重复提交相同标签；
- **数量截断**：总标签数截断至最多 12 个（`MaxBiliTags`）。

### 5.4 敏感词过滤与合规打码 (`SensitiveWordSanitizer`)

针对 B 站平台审核的高频拦截风险，内置开箱即用的敏感词清洗器：

- **涉赌/灰产**：`赌博`、`博彩`、`百家乐`、`六合彩`、`代开发票`、`套现`、`刷单`等；
- **色情低俗**：`色情`、`黄色网站`、`约炮`、`成人网`等；
- **翻墙/违规代理**：`翻墙`、`科学上网`、`梯子推荐`、`免翻`、`vpn推荐`、`外网节点`等；
- **恶意引流**：`加微`、`加v`、`扫码领资料`、`私聊领`等。

**处理策略**：

- 标题、简介、动态文本中出现的敏感词，通过正则不区分大小写匹配，替换为**等长 Unicode 星号**（如`科学上网` -> `****`），不破坏文本排版；
- 标签列表中若命中违规词，则**整条标签直接剔除**；
- 每次命中均向 `res.Warnings` 追加记录（如`"标题包含违禁敏感词并已自动打码: 科学上网"`），供前端控制台溯源。

---

## 6. 确定性降级引擎 (`GenerateFallbackDecision`)

当流水线遭遇以下极端场景时，绝不抛出未捕获异常，直接转入保底模式：

1. Pi Agent 子进程执行超时（默认 120 秒中断）；
2. 第三方 LLM 余额耗尽（HTTP 402 Insufficient Quota）或服务宕机（HTTP 503）；
3. Agent 返回无法解析的残缺文本且 2 次重试均失败；
4. Agent 决策结果由于模型安全对齐原因拒答（Refusal）。

### 6.1 保底生成规则

| 目标字段 | 保底算法 |
| :--- | :--- |
| **`title`** | 优先使用策略的 `DynamicTitleTemplate`（如替换 `{title}`）；若无模板则使用原始标题；最后截取前 80 字符。 |
| **`desc`** | 优先渲染策略的 `DescTemplate`；若无则自动拼装“原标题 + 原频道 + 原出处 URL + 转载学习交流免责声明”。 |
| **`tags`** | 优先使用策略配置的 `DefaultTags`；若不足则补充原视频标签，清洗后裁剪至 12 个；若仍为空则兜底 `["搬运", "视频搬运", 平台名]`。 |
| **`tid`** | 采用策略指定的 `CategoryID`；若为 0 则回退至默认单机游戏区 `17`。 |
| **`dynamic`** | 自动生成：“搬运分享了来自 {Channel} 的新视频：{Title}”。 |
| **`degraded`** | **显式置为 `true`**。 |
| **`warnings`** | 记录详细的降级原因（如：`"Agent 决策降级回退: Pi Agent 子进程超时(120s)"`）。 |

---

## 7. 自动化测试验证矩阵

在 `internal/prototype/agent_decision_test.go` 中，已建立全套针对 Stage-5 决策契约的自动化测试用例：

| 测试用例名 | 验证目标 | 测试场景 |
| :--- | :--- | :--- |
| `TestDecisionAgent_SchemaValidation` | JSON 格式与字段解析 | 验证标准合法 JSON 解析、Markdown 代码块剥离解析、缺少必填 title 报错拒绝、语法错乱报错拒绝。 |
| `TestDecisionAgent_TitleClamp80Chars` | 标题 80 字符硬截断 | 传入 95 字超长标题，验证被精准裁剪至 80 字符，且前缀完全一致，审计记录警告。 |
| `TestDecisionAgent_TagNormalizationAndLimit` | 标签清洗与限额 | 传入 18 个包含重复大小写、空格逗号及超长字符的标签，验证截断至 12 个，大小写去重，无空格逗号。 |
| `TestDecisionAgent_SensitiveWordFilter` | 敏感词初筛与替换 | 传入含“科学上网”、“翻墙”、“博彩”、“加微”的文本，验证文本自动打码替换为 `*`，违规标签直接剔除，警告信息完备。 |
| `TestDecisionAgent_FallbackDegradation` | 确定性保底回退 | 模拟超时及空上下文，验证保底引擎自动生成合法完整的元数据，`degraded == true` 且包含排查原因。 |
| `TestDecisionAgent_PromptTemplates` | 提示词渲染引擎 | 验证人设注入、时长转换（`MM:SS`）、字幕摘要展示及 JSON Schema 嵌合正确。 |

本规范完全契合 `CONTEXT.md` 中关于“内容决策 Agent”在“搬运策略”与“投稿防线”约束下的自治边界。
