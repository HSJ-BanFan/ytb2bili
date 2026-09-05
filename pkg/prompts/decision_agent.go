package prompts

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// B 站平台投稿规范常量
const (
	MaxBiliTitleRunes = 80 // B 站标题硬性上限（最多 80 个字符/中英文字元）
	MaxBiliTags       = 12 // B 站标签数量硬性上限（最多 12 个）
	MaxTagRuneLength  = 20 // 单个标签字符上限（最多 20 个字符）
	DefaultCategoryID = 17 // 默认分区 ID（单机/综合游戏区 17，或科技区 188）
)

// VideoDecisionContext 阶段 5 Pi Agent 内容决策的输入上下文
type VideoDecisionContext struct {
	// 原始媒体元数据
	SourcePlatform  string   `json:"source_platform"`           // 来源平台: youtube, twitch
	SourceVideoID   string   `json:"source_video_id"`           // 原始平台视频 ID
	OriginalTitle   string   `json:"original_title"`            // 原始视频标题
	OriginalDesc    string   `json:"original_desc,omitempty"`   // 原始视频简介
	OriginalChannel string   `json:"original_channel"`          // 来源频道名称
	OriginalAuthor  string   `json:"original_author,omitempty"` // 原始作者名称
	DurationSeconds int      `json:"duration_seconds"`          // 视频时长（秒）
	OriginalTags    []string `json:"original_tags,omitempty"`   // 原视频标签
	SourceURL       string   `json:"source_url,omitempty"`      // 原视频来源 URL

	// 字幕与内容摘要
	TranscriptSummary string `json:"transcript_summary,omitempty"` // 提取出的关键摘要或前缀字幕
	SourceLanguage    string `json:"source_language,omitempty"`    // 识别语言代码（如 en, ja, zh）
	HasSubtitles      bool   `json:"has_subtitles"`                // 是否已生成或存在字幕

	// 策略矩阵与目标账号要求
	TargetAccountName    string   `json:"target_account_name,omitempty"`    // 目标 B 站账号名称
	TargetAccountPersona string   `json:"target_account_persona,omitempty"` // 账号人设偏好（如：硬核科技、轻松娱乐、干货速览）
	DynamicTitleTemplate string   `json:"dynamic_title_template,omitempty"` // 标题模板或修饰前缀（如：【科技前沿】{title}）
	DescTemplate         string   `json:"desc_template,omitempty"`          // 简介补充声明模板
	DefaultTags          []string `json:"default_tags,omitempty"`           // 策略配置的缺省标签池
	CategoryID           int      `json:"category_id,omitempty"`            // 目标分区 TID
	Copyright            int      `json:"copyright,omitempty"`              // 投稿类型: 1=自制, 2=转载
	SourceOrigin         string   `json:"source_origin,omitempty"`          // 转载出处说明
	AutoPublish          bool     `json:"auto_publish"`                     // 是否全自动发布
}

// VideoDecisionResult 阶段 5 决策 Agent 输出的结构化决策产物
type VideoDecisionResult struct {
	Title       string   `json:"title"`                  // B 站定制标题（<= 80 字符）
	Desc        string   `json:"desc"`                   // 本地化结构化简介（含作者版权声明）
	Tags        []string `json:"tags"`                   // 核心标签列表（1-12 个）
	TID         int      `json:"tid"`                    // B 站主分区 TID
	Dynamic     string   `json:"dynamic,omitempty"`      // B 站动态文案
	PartTitles  []string `json:"part_titles,omitempty"`  // 多 P 章节分段标题列表（若有多 P）
	CoverPrompt string   `json:"cover_prompt,omitempty"` // 封面构图或文本提示词
	Reasoning   string   `json:"reasoning,omitempty"`    // 决策依据与思路简述
	Degraded    bool     `json:"degraded"`               // 是否为确定性降级保底产物
	Warnings    []string `json:"warnings,omitempty"`     // 处理过程中的警告信息（如超长截断、敏感词替换等）
}

// DecisionJSONSchema 定义 Stage-5 决策的 JSON Schema 字符串
const DecisionJSONSchema = `{
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
}`

// BuildDecisionSystemPrompt 构建系统角色提示词
func BuildDecisionSystemPrompt(persona string) string {
	personaDesc := "你是一位精通 Bilibili 社区生态、深谙年轻观众偏好与平台审核规则的资深内容运营专家。"
	if strings.TrimSpace(persona) != "" {
		personaDesc = fmt.Sprintf("你是一位专精于 Bilibili 社区生态的内容运营专家，本次执行的运营人设风格为【%s】。", persona)
	}

	return fmt.Sprintf(`%s

你的核心职责是：分析海外平台（如 YouTube、Twitch）的原始视频元数据、字幕摘要与搬运策略，为该视频制定最符合 B 站受众习惯、高点击率且严谨合规的投稿元数据。

必须严格遵守以下硬性平台约束与质量准则：
1. 【标题（title）】
   - 字符长度限制：严格介于 1 到 80 个字符之间（中英文字符均计为 1 个字符，超出将被平台直接拒收）。
   - 质量建议：推荐 20 至 45 个字符，突出核心矛盾、技术要点或剧情亮点；严禁低俗恶俗、虚假夸大的标题党。
   - 若策略上下文中给出了标题模板或固定前缀，请优先融合。
2. 【简介（desc）】
   - 结构清晰，包含：① 中文本地化核心看点简介；② 原作者信息及来源链接；③ 版权与搬运声明（如："本视频仅作分享交流，版权归原作者所有"）。
3. 【标签（tags）】
   - 标签数量严格在 3 至 12 个之间；每个标签长度不得超过 20 字符。
   - 标签中严禁包含空格、英文逗号或非法特殊符号。
   - 必须优先包含策略中指定的默认标签，并补充与视频内容紧密契合的主题标签。
4. 【分区（tid）】
   - 填入最契合该视频内容的 B 站主分区 TID。若策略上下文中指定了 category_id，必须以该 TID 为准。
5. 【合规防线】
   - 严禁出现涉及翻墙、免翻、赌博、色情、引流加群加微信等任何违反法律法规或平台规则的词汇。
6. 【输出契约】
   - 必须输出且仅输出单个符合指定 JSON Schema 的纯 JSON 对象，不要附加任何 Markdown 格式前缀后缀、闲聊文字或代码块外部注释。`, personaDesc)
}

// BuildDecisionUserPrompt 构建输入上下文的用户提示词
func BuildDecisionUserPrompt(ctx *VideoDecisionContext) string {
	var sb strings.Builder
	sb.WriteString("请根据以下输入的视频元数据与策略规则，生成符合规范的 B 站投稿决策 JSON：\n\n")

	sb.WriteString("### 1. 原始媒体信息\n")
	sb.WriteString(fmt.Sprintf("- 来源平台: %s\n", ctx.SourcePlatform))
	sb.WriteString(fmt.Sprintf("- 视频 ID: %s\n", ctx.SourceVideoID))
	sb.WriteString(fmt.Sprintf("- 原始标题: %s\n", ctx.OriginalTitle))
	if ctx.OriginalChannel != "" {
		sb.WriteString(fmt.Sprintf("- 原始频道: %s\n", ctx.OriginalChannel))
	}
	if ctx.OriginalAuthor != "" {
		sb.WriteString(fmt.Sprintf("- 原始作者: %s\n", ctx.OriginalAuthor))
	}
	if ctx.DurationSeconds > 0 {
		sb.WriteString(fmt.Sprintf("- 时长: %d 秒 (%02d:%02d)\n", ctx.DurationSeconds, ctx.DurationSeconds/60, ctx.DurationSeconds%60))
	}
	if ctx.SourceURL != "" {
		sb.WriteString(fmt.Sprintf("- 视频来源 URL: %s\n", ctx.SourceURL))
	}
	if len(ctx.OriginalTags) > 0 {
		sb.WriteString(fmt.Sprintf("- 原平台标签: %s\n", strings.Join(ctx.OriginalTags, ", ")))
	}
	if ctx.OriginalDesc != "" {
		descTrunc := TruncateRunes(ctx.OriginalDesc, 300)
		sb.WriteString(fmt.Sprintf("- 原简介摘要: %s\n", descTrunc))
	}

	sb.WriteString("\n### 2. 字幕与内容摘要\n")
	sb.WriteString(fmt.Sprintf("- 原始语言: %s\n", ctx.SourceLanguage))
	sb.WriteString(fmt.Sprintf("- 中文字幕就绪: %t\n", ctx.HasSubtitles))
	if ctx.TranscriptSummary != "" {
		sb.WriteString(fmt.Sprintf("- 字幕文本核心摘要:\n%s\n", TruncateRunes(ctx.TranscriptSummary, 600)))
	} else {
		sb.WriteString("- 字幕文本核心摘要: (未提取到字幕摘要，请主要依据原标题与简介决策)\n")
	}

	sb.WriteString("\n### 3. 策略矩阵与发布约束\n")
	if ctx.TargetAccountName != "" {
		sb.WriteString(fmt.Sprintf("- 目标投稿账号: %s\n", ctx.TargetAccountName))
	}
	if ctx.DynamicTitleTemplate != "" {
		sb.WriteString(fmt.Sprintf("- 标题生成偏好/模板: %s\n", ctx.DynamicTitleTemplate))
	}
	if ctx.DescTemplate != "" {
		sb.WriteString(fmt.Sprintf("- 简介声明模板: %s\n", ctx.DescTemplate))
	}
	if len(ctx.DefaultTags) > 0 {
		sb.WriteString(fmt.Sprintf("- 策略必填/缺省标签: %s\n", strings.Join(ctx.DefaultTags, ", ")))
	}
	if ctx.CategoryID > 0 {
		sb.WriteString(fmt.Sprintf("- 指定分区 TID: %d\n", ctx.CategoryID))
	}
	sb.WriteString(fmt.Sprintf("- 投稿版权类型: %d (1=自制, 2=转载)\n", ctx.Copyright))
	if ctx.SourceOrigin != "" {
		sb.WriteString(fmt.Sprintf("- 转载来源声明: %s\n", ctx.SourceOrigin))
	}

	sb.WriteString("\n### 4. 输出 JSON Schema 约束\n")
	sb.WriteString("请严格按照以下 JSON Schema 格式输出单个 JSON 对象：\n")
	sb.WriteString(DecisionJSONSchema)
	sb.WriteString("\n\n直接输出合法 JSON，勿添加任何前后解释文本。")

	return sb.String()
}

// SensitiveWordSanitizer 敏感词过滤与合规初筛清洗器
type SensitiveWordSanitizer struct {
	patterns []*regexp.Regexp
	keywords []string
}

// defaultSensitiveKeywords B 站发稿常见受限高风险关键词
var defaultSensitiveKeywords = []string{
	"赌博", "博彩", "百家乐", "六合彩", "时时彩", "下注", "外围盘",
	"代开发票", "套现", "跑分", "刷单", "高利贷",
	"色情", "黄色网站", "约炮", "成人网", "裸聊",
	"翻墙", "科学上网", "梯子推荐", "免翻", "vpn推荐", "外网节点", "翻墙软件",
	"加微", "加v", "加群领", "微信+", "加我微信", "扫码领资料", "私聊领",
}

// NewDefaultSanitizer 创建带有默认敏感词库的清洗器
func NewDefaultSanitizer() *SensitiveWordSanitizer {
	return NewSanitizer(defaultSensitiveKeywords)
}

// NewSanitizer 创建自定义敏感词清洗器
func NewSanitizer(words []string) *SensitiveWordSanitizer {
	s := &SensitiveWordSanitizer{
		keywords: make([]string, 0, len(words)),
		patterns: make([]*regexp.Regexp, 0, len(words)),
	}

	for _, w := range words {
		trimmed := strings.TrimSpace(w)
		if trimmed == "" {
			continue
		}
		s.keywords = append(s.keywords, trimmed)
		// 构建不区分大小写的正则表达式
		pattern, err := regexp.Compile("(?i)" + regexp.QuoteMeta(trimmed))
		if err == nil {
			s.patterns = append(s.patterns, pattern)
		}
	}
	return s
}

// SanitizeText 对单段文本进行敏感词清洗，将敏感词替换为等长星号，并返回命中词
func (s *SensitiveWordSanitizer) SanitizeText(text string) (sanitized string, replacedCount int, matchedWords []string) {
	if text == "" {
		return "", 0, nil
	}

	result := text
	matchedMap := make(map[string]struct{})
	totalReplaced := 0

	for _, re := range s.patterns {
		matches := re.FindAllString(result, -1)
		if len(matches) > 0 {
			for _, m := range matches {
				matchedMap[m] = struct{}{}
				totalReplaced++
			}
			result = re.ReplaceAllStringFunc(result, func(match string) string {
				runeLen := utf8.RuneCountInString(match)
				return strings.Repeat("*", runeLen)
			})
		}
	}

	matchedList := make([]string, 0, len(matchedMap))
	for w := range matchedMap {
		matchedList = append(matchedList, w)
	}

	return result, totalReplaced, matchedList
}

// SanitizeResult 对完整的 VideoDecisionResult 进行深度清洗
func (s *SensitiveWordSanitizer) SanitizeResult(res *VideoDecisionResult) int {
	if res == nil {
		return 0
	}

	totalReplaced := 0

	// 1. 清洗标题
	cleanTitle, count, words := s.SanitizeText(res.Title)
	if count > 0 {
		res.Title = cleanTitle
		totalReplaced += count
		res.Warnings = append(res.Warnings, fmt.Sprintf("标题包含违禁敏感词并已自动打码: %s", strings.Join(words, ", ")))
	}

	// 2. 清洗简介
	cleanDesc, count, words := s.SanitizeText(res.Desc)
	if count > 0 {
		res.Desc = cleanDesc
		totalReplaced += count
		res.Warnings = append(res.Warnings, fmt.Sprintf("简介包含违禁敏感词并已自动打码: %s", strings.Join(words, ", ")))
	}

	// 3. 清洗动态
	if res.Dynamic != "" {
		cleanDyn, count, words := s.SanitizeText(res.Dynamic)
		if count > 0 {
			res.Dynamic = cleanDyn
			totalReplaced += count
			res.Warnings = append(res.Warnings, fmt.Sprintf("动态文案包含违禁敏感词并已自动打码: %s", strings.Join(words, ", ")))
		}
	}

	// 4. 清洗标签（命中敏感词的标签直接剔除）
	validTags := make([]string, 0, len(res.Tags))
	for _, tag := range res.Tags {
		_, count, words := s.SanitizeText(tag)
		if count > 0 {
			totalReplaced += count
			res.Warnings = append(res.Warnings, fmt.Sprintf("标签【%s】包含违禁词【%s】已被移除", tag, strings.Join(words, ", ")))
		} else {
			validTags = append(validTags, tag)
		}
	}
	res.Tags = validTags

	return totalReplaced
}

// CleanJSONString 从 LLM 输出中剥离可能存在的 Markdown 代码块包裹
func CleanJSONString(raw string) string {
	trimmed := strings.TrimSpace(raw)
	// 匹配 ```json ... ``` 或 ``` ... ```
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 2 {
			// 移除第一行 ```json
			startIdx := 1
			endIdx := len(lines)
			if strings.HasPrefix(lines[len(lines)-1], "```") {
				endIdx = len(lines) - 1
			}
			trimmed = strings.TrimSpace(strings.Join(lines[startIdx:endIdx], "\n"))
		}
	}
	return trimmed
}

// ValidateAndNormalizeDecision 解析并校验 Agent 产出的原始 JSON 字符串
func ValidateAndNormalizeDecision(rawJSON string, ctx *VideoDecisionContext, sanitizer *SensitiveWordSanitizer) (*VideoDecisionResult, error) {
	cleanJSON := CleanJSONString(rawJSON)
	if cleanJSON == "" {
		return nil, fmt.Errorf("agent 返回内容为空")
	}

	var res VideoDecisionResult
	if err := json.Unmarshal([]byte(cleanJSON), &res); err != nil {
		return nil, fmt.Errorf("JSON 反序列化失败: %w (原始返回片段: %s)", err, TruncateRunes(cleanJSON, 100))
	}

	if sanitizer == nil {
		sanitizer = NewDefaultSanitizer()
	}

	// 1. 校验与规范化标题
	res.Title = strings.TrimSpace(res.Title)
	if res.Title == "" {
		return nil, fmt.Errorf("决策结果缺少必填字段: title 不能为空")
	}

	titleRuneCount := utf8.RuneCountInString(res.Title)
	if titleRuneCount > MaxBiliTitleRunes {
		res.Title = TruncateRunes(res.Title, MaxBiliTitleRunes)
		res.Warnings = append(res.Warnings, fmt.Sprintf("标题长度(%d字)超出 B 站上限(80字)，已自动截断至 80 字符", titleRuneCount))
	}

	// 2. 校验与规范化简介
	res.Desc = strings.TrimSpace(res.Desc)
	if res.Desc == "" {
		// 若简介为空，从上下文保底生成
		res.Desc = generateDefaultDesc(ctx)
		res.Warnings = append(res.Warnings, "Agent 未生成简介，已使用策略保底简介")
	}

	// 3. 校验与规范化标签
	res.Tags = normalizeTags(res.Tags, ctx.DefaultTags)

	// 4. 校验分区 TID
	if res.TID <= 0 {
		if ctx.CategoryID > 0 {
			res.TID = ctx.CategoryID
		} else {
			res.TID = DefaultCategoryID
		}
		res.Warnings = append(res.Warnings, fmt.Sprintf("TID 无效，已回退至默认分区 %d", res.TID))
	}

	// 5. 敏感词清洗
	sanitizer.SanitizeResult(&res)

	// 清洗后再次确保标签不为空
	if len(res.Tags) == 0 {
		res.Tags = []string{"搬运", "视频搬运"}
		res.Warnings = append(res.Warnings, "标签池在清洗后为空，已补充基础保底标签")
	}

	return &res, nil
}

// GenerateFallbackDecision 阶段 5 确定性降级保底生成器
// 当 Agent 子进程不可用、执行超时或输出严重无法解析时调用，保证流水线不被挂死
func GenerateFallbackDecision(ctx *VideoDecisionContext, failureReason string) *VideoDecisionResult {
	if ctx == nil {
		ctx = &VideoDecisionContext{}
	}

	// 1. 生成保底标题
	title := strings.TrimSpace(ctx.OriginalTitle)
	if ctx.DynamicTitleTemplate != "" {
		if strings.Contains(ctx.DynamicTitleTemplate, "{title}") {
			title = strings.ReplaceAll(ctx.DynamicTitleTemplate, "{title}", title)
		} else {
			title = fmt.Sprintf("%s %s", ctx.DynamicTitleTemplate, title)
		}
	} else if title == "" {
		title = fmt.Sprintf("搬运视频 %s", ctx.SourceVideoID)
	}
	title = strings.TrimSpace(title)
	if utf8.RuneCountInString(title) > MaxBiliTitleRunes {
		title = TruncateRunes(title, MaxBiliTitleRunes)
	}

	// 2. 生成保底简介
	desc := generateDefaultDesc(ctx)

	// 3. 生成保底标签
	tags := normalizeTags(ctx.DefaultTags, ctx.OriginalTags)
	if len(tags) == 0 {
		platformTag := "YouTube"
		if strings.ToLower(ctx.SourcePlatform) == "twitch" {
			platformTag = "Twitch"
		}
		tags = []string{"搬运", "视频搬运", platformTag}
	}

	// 4. 分区 TID
	tid := ctx.CategoryID
	if tid <= 0 {
		tid = DefaultCategoryID
	}

	// 5. 动态发布文案
	dynamic := ""
	if ctx.OriginalChannel != "" {
		dynamic = fmt.Sprintf("搬运分享了来自 %s 的新视频：%s", ctx.OriginalChannel, TruncateRunes(title, 30))
	}

	res := &VideoDecisionResult{
		Title:     title,
		Desc:      desc,
		Tags:      tags,
		TID:       tid,
		Dynamic:   dynamic,
		Reasoning: "确定性规则降级引擎自动生成（LLM 阶段不可用或输出异常）",
		Degraded:  true,
		Warnings:  []string{fmt.Sprintf("Agent 决策降级回退: %s", failureReason)},
	}

	// 敏感词安全过滤
	sanitizer := NewDefaultSanitizer()
	sanitizer.SanitizeResult(res)

	return res
}

// generateDefaultDesc 构造结构化保底简介
func generateDefaultDesc(ctx *VideoDecisionContext) string {
	if ctx.DescTemplate != "" {
		d := ctx.DescTemplate
		d = strings.ReplaceAll(d, "{original_title}", ctx.OriginalTitle)
		d = strings.ReplaceAll(d, "{channel}", ctx.OriginalChannel)
		d = strings.ReplaceAll(d, "{author}", ctx.OriginalAuthor)
		d = strings.ReplaceAll(d, "{source_url}", ctx.SourceURL)
		return d
	}

	var sb strings.Builder
	sb.WriteString("【视频信息】\n")
	if ctx.OriginalTitle != "" {
		sb.WriteString(fmt.Sprintf("原标题：%s\n", ctx.OriginalTitle))
	}
	if ctx.OriginalChannel != "" {
		sb.WriteString(fmt.Sprintf("原频道：%s\n", ctx.OriginalChannel))
	}
	if ctx.SourceURL != "" {
		sb.WriteString(fmt.Sprintf("原视频出处：%s\n", ctx.SourceURL))
	}

	if ctx.TranscriptSummary != "" {
		sb.WriteString("\n【看点要点】\n")
		sb.WriteString(TruncateRunes(ctx.TranscriptSummary, 200))
		sb.WriteString("\n")
	}

	sb.WriteString("\n【声明】\n")
	sb.WriteString("本视频仅作分享与交流使用，视频版权与收益均归原作者所有。若有侵权请联系后台删除。")

	return sb.String()
}

// normalizeTags 标签清洗、去重、限长与裁剪
func normalizeTags(preferredTags, fallbackTags []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, MaxBiliTags)

	addTag := func(tag string) {
		t := strings.TrimSpace(tag)
		t = strings.ReplaceAll(t, " ", "")
		t = strings.ReplaceAll(t, ",", "")
		t = strings.ReplaceAll(t, "，", "")
		if t == "" {
			return
		}
		if utf8.RuneCountInString(t) > MaxTagRuneLength {
			t = TruncateRunes(t, MaxTagRuneLength)
		}
		lower := strings.ToLower(t)
		if _, exists := seen[lower]; !exists {
			seen[lower] = struct{}{}
			result = append(result, t)
		}
	}

	// 优先添加策略指定标签
	for _, t := range preferredTags {
		if len(result) >= MaxBiliTags {
			break
		}
		addTag(t)
	}

	// 补充兜底或原视频标签
	for _, t := range fallbackTags {
		if len(result) >= MaxBiliTags {
			break
		}
		addTag(t)
	}

	return result
}

// TruncateRunes 按 Unicode 字符/元（rune）精准截断字符串
func TruncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}
