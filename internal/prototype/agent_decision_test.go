package prototype_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/difyz9/ytb2bili/pkg/prompts"
)

// ============================================================================
// 1. JSON Schema 校验与解析测试 (Schema Validation)
// ============================================================================

func TestDecisionAgent_SchemaValidation(t *testing.T) {
	ctx := &prompts.VideoDecisionContext{
		SourcePlatform:  "youtube",
		SourceVideoID:   "v_abc123",
		OriginalTitle:   "Building a High-Performance Distributed Database in Rust",
		OriginalChannel: "RustTechLabs",
		DefaultTags:     []string{"Rust", "编程", "分布式"},
		CategoryID:      188,
	}

	sanitizer := prompts.NewDefaultSanitizer()

	t.Run("标准合法 JSON 解析成功", func(t *testing.T) {
		validJSON := `{
			"title": "【精翻】用 Rust 从零手写高并发分布式数据库，架构设计全解析",
			"desc": "本期视频深入拆解分布式数据库的核心架构与 Paxos 共识算法实现。\n\n原频道：RustTechLabs\n出处：https://youtube.com/watch?v=v_abc123\n本视频由 AI 协同汉化并获得搬运指引，仅作交流学习。",
			"tags": ["Rust", "数据库", "分布式", "后端开发", "架构设计", "高并发"],
			"tid": 188,
			"dynamic": "深度硬核！用 Rust 从零构筑分布式数据库，架构师必看干货更新啦～",
			"part_titles": ["P1 存储引擎与 LSM-Tree 设计", "P2 Raft/Paxos 分布式共识集成"],
			"reasoning": "根据原视频技术深度与受众画像，强化硬核架构与干货标签，符合科技区 B 站用户偏好。"
		}`

		res, err := prompts.ValidateAndNormalizeDecision(validJSON, ctx, sanitizer)
		require.NoError(t, err)
		require.NotNil(t, res)

		assert.Equal(t, "【精翻】用 Rust 从零手写高并发分布式数据库，架构设计全解析", res.Title)
		assert.Equal(t, 188, res.TID)
		assert.Contains(t, res.Tags, "Rust")
		assert.Contains(t, res.Tags, "分布式")
		assert.Len(t, res.PartTitles, 2)
		assert.False(t, res.Degraded)
		assert.Empty(t, res.Warnings)
	})

	t.Run("Markdown 代码块包裹的 JSON 能够被正确提取解析", func(t *testing.T) {
		markdownJSON := "```json\n" + `{
			"title": "用 Rust 编写分布式数据库全记录",
			"desc": "原频道：RustTechLabs",
			"tags": ["Rust", "数据库"],
			"tid": 188
		}` + "\n```"

		res, err := prompts.ValidateAndNormalizeDecision(markdownJSON, ctx, sanitizer)
		require.NoError(t, err)
		assert.Equal(t, "用 Rust 编写分布式数据库全记录", res.Title)
	})

	t.Run("缺少必填字段 title 时报错拒绝", func(t *testing.T) {
		missingTitleJSON := `{
			"desc": "只有简介没有标题",
			"tags": ["测试"],
			"tid": 188
		}`

		_, err := prompts.ValidateAndNormalizeDecision(missingTitleJSON, ctx, sanitizer)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "title 不能为空")
	})

	t.Run("格式错乱非合法 JSON 报错拒绝", func(t *testing.T) {
		brokenJSON := `这里是 LLM 的一段闲聊文本，没有输出合法 JSON: {"title": "未闭合`
		_, err := prompts.ValidateAndNormalizeDecision(brokenJSON, ctx, sanitizer)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "JSON 反序列化失败")
	})
}

// ============================================================================
// 2. 标题 80 字符硬性截断测试 (80-Char Title Clamp)
// ============================================================================

func TestDecisionAgent_TitleClamp80Chars(t *testing.T) {
	ctx := &prompts.VideoDecisionContext{
		SourcePlatform: "youtube",
		SourceVideoID:  "v_long_title",
		CategoryID:     188,
	}

	sanitizer := prompts.NewDefaultSanitizer()

	// 构建一个长度达到 95 个中文字符的超长标题
	superLongTitle := "【深度解构极度震撼全网首发】这可能是你今年看过的最硬核的计算机体系架构全流程全景解析，从底层硅晶圆制造到光刻机原理再到操作系统内核指令集与微架构设计全方位无死角深度拆解指南超强干货合辑收藏必备！"
	initialRuneLen := utf8.RuneCountInString(superLongTitle)
	require.Greater(t, initialRuneLen, 80, "原始测试标题应大于 80 个字符")

	inputJSON := `{
		"title": "` + superLongTitle + `",
		"desc": "超长标题测试简介",
		"tags": ["计算机", "芯片", "科普"],
		"tid": 188
	}`

	res, err := prompts.ValidateAndNormalizeDecision(inputJSON, ctx, sanitizer)
	require.NoError(t, err)
	require.NotNil(t, res)

	clampedRuneLen := utf8.RuneCountInString(res.Title)
	assert.Equal(t, prompts.MaxBiliTitleRunes, clampedRuneLen, "标题必须被精准截断至 80 字符")
	assert.True(t, strings.HasPrefix(superLongTitle, res.Title), "截断后的内容必须与原标题前缀严格一致")

	// 校验警告信息被记入
	hasClampWarning := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "超出 B 站上限(80字)") {
			hasClampWarning = true
			break
		}
	}
	assert.True(t, hasClampWarning, "必须记录超长标题截断的审计警告")
}

// ============================================================================
// 3. 标签清洗、去重与数量上限测试 (Tag Limit & Normalization)
// ============================================================================

func TestDecisionAgent_TagNormalizationAndLimit(t *testing.T) {
	ctx := &prompts.VideoDecisionContext{
		SourcePlatform: "youtube",
		SourceVideoID:  "v_tags",
		DefaultTags:    []string{"科技", "数码"},
		CategoryID:     188,
	}
	sanitizer := prompts.NewDefaultSanitizer()

	// 输入超过 12 个标签，包含重复项（大小写不同）、带空格与逗号、超长单标签（>20 字符）
	inputJSON := `{
		"title": "标签清洗与裁剪综合测试",
		"desc": "简介内容",
		"tags": [
			"rust", "RUST", "  rust  ", 
			"Go,lang", "Python，教程",
			"这是一个超级长长长长长长长长长长长长长长长长长长长长的单标签", 
			"标签1", "标签2", "标签3", "标签4", "标签5", 
			"标签6", "标签7", "标签8", "标签9", "标签10"
		],
		"tid": 188
	}`

	res, err := prompts.ValidateAndNormalizeDecision(inputJSON, ctx, sanitizer)
	require.NoError(t, err)

	// 1. 数量上限不得超过 12 个
	assert.LessOrEqual(t, len(res.Tags), prompts.MaxBiliTags, "最终标签数量不得超过 12 个")
	assert.Equal(t, prompts.MaxBiliTags, len(res.Tags), "超量标签应被截断至 12 个")

	// 2. 去重校验：不能同时出现大小写重复的 rust
	rustCount := 0
	for _, tag := range res.Tags {
		if strings.ToLower(tag) == "rust" {
			rustCount++
		}
	}
	assert.Equal(t, 1, rustCount, "标签应大小写不敏感去重")

	// 3. 单标签字符长度不能超过 20
	for _, tag := range res.Tags {
		assert.LessOrEqual(t, utf8.RuneCountInString(tag), prompts.MaxTagRuneLength, "单标签长度不得超过 20 字符")
		assert.False(t, strings.Contains(tag, " "), "标签中不得包含空格")
		assert.False(t, strings.Contains(tag, ","), "标签中不得包含英文逗号")
		assert.False(t, strings.Contains(tag, "，"), "标签中不得包含中文逗号")
	}
}

// ============================================================================
// 4. 敏感词过滤与合规初筛测试 (Sensitive Word Filter)
// ============================================================================

func TestDecisionAgent_SensitiveWordFilter(t *testing.T) {
	sanitizer := prompts.NewDefaultSanitizer()

	t.Run("单文本清洗打码验证", func(t *testing.T) {
		raw := "推荐一款科学上网工具，免翻直接看，加微进群领最新资料"
		cleaned, count, matched := sanitizer.SanitizeText(raw)

		assert.Greater(t, count, 0)
		assert.Contains(t, matched, "科学上网")
		assert.Contains(t, matched, "免翻")
		assert.Contains(t, matched, "加微")
		assert.NotContains(t, cleaned, "科学上网")
		assert.NotContains(t, cleaned, "免翻")
		assert.NotContains(t, cleaned, "加微")
		assert.Contains(t, cleaned, "****工具") // 4个中文字符替换为 4 个 *
	})

	t.Run("决策结果整体安全过滤与警告注入", func(t *testing.T) {
		ctx := &prompts.VideoDecisionContext{
			SourcePlatform: "youtube",
			SourceVideoID:  "v_sensitive",
			CategoryID:     188,
		}

		inputJSON := `{
			"title": "海外网络连接指南：教你如何科学上网与配置外网节点",
			"desc": "本期带来翻墙软件使用技巧，更有博彩推广专区，扫码加微信领独家福利！",
			"tags": ["网络技术", "翻墙", "科技科普", "赌博"],
			"dynamic": "最新节点分享，加微进群领资料～",
			"tid": 188
		}`

		res, err := prompts.ValidateAndNormalizeDecision(inputJSON, ctx, sanitizer)
		require.NoError(t, err)

		// 标题打码
		assert.NotContains(t, res.Title, "科学上网")
		assert.NotContains(t, res.Title, "外网节点")

		// 简介打码
		assert.NotContains(t, res.Desc, "翻墙软件")
		assert.NotContains(t, res.Desc, "博彩")
		assert.NotContains(t, res.Desc, "微信")

		// 动态打码
		assert.NotContains(t, res.Dynamic, "加微")

		// 违规标签直接剔除
		assert.NotContains(t, res.Tags, "翻墙")
		assert.NotContains(t, res.Tags, "赌博")
		assert.Contains(t, res.Tags, "网络技术")
		assert.Contains(t, res.Tags, "科技科普")

		// 验证警告记录
		assert.NotEmpty(t, res.Warnings)
		hasTitleWarn := false
		hasTagWarn := false
		for _, w := range res.Warnings {
			if strings.Contains(w, "标题包含违禁敏感词") {
				hasTitleWarn = true
			}
			if strings.Contains(w, "包含违禁词【翻墙】已被移除") {
				hasTagWarn = true
			}
		}
		assert.True(t, hasTitleWarn, "应记录标题违禁词警告")
		assert.True(t, hasTagWarn, "应记录标签剔除警告")
	})
}

// ============================================================================
// 5. 确定性降级保底测试 (Fallback Degradation)
// ============================================================================

func TestDecisionAgent_FallbackDegradation(t *testing.T) {
	ctx := &prompts.VideoDecisionContext{
		SourcePlatform:       "youtube",
		SourceVideoID:        "yt_fallback_999",
		OriginalTitle:        "Advanced Concurrency Patterns in Go 1.24",
		OriginalDesc:         "Complete walkthrough of go channels, select, and sync primitives.",
		OriginalChannel:      "GopherDaily",
		OriginalAuthor:       "Rob Pike Fan",
		SourceURL:            "https://youtube.com/watch?v=yt_fallback_999",
		OriginalTags:         []string{"Golang", "Concurrency", "Backend"},
		TranscriptSummary:    "探讨了 Go 并发模型从 CSP 到现代协程池的演进路线。",
		TargetAccountName:    "Go语言技术精选",
		DynamicTitleTemplate: "【搬运精选】{title}",
		DescTemplate:         "原视频：{original_title}\n频道：{channel}\n出处：{source_url}\n本内容由搬运工厂自动化同步。",
		DefaultTags:          []string{"Go语言", "后端", "编程教学"},
		CategoryID:           188,
	}

	t.Run("模拟 Agent 超时触发降级保底", func(t *testing.T) {
		res := prompts.GenerateFallbackDecision(ctx, "Pi Agent 子进程超时(120s)")
		require.NotNil(t, res)

		assert.True(t, res.Degraded, "必须标记为 degraded 降级状态")
		assert.Equal(t, "【搬运精选】Advanced Concurrency Patterns in Go 1.24", res.Title)
		assert.Contains(t, res.Desc, "Advanced Concurrency Patterns in Go 1.24")
		assert.Contains(t, res.Desc, "GopherDaily")
		assert.Contains(t, res.Desc, "https://youtube.com/watch?v=yt_fallback_999")
		assert.Equal(t, 188, res.TID)
		assert.Contains(t, res.Tags, "Go语言")
		assert.Contains(t, res.Tags, "Golang")
		assert.NotEmpty(t, res.Dynamic)
		assert.Contains(t, res.Warnings[0], "Agent 决策降级回退: Pi Agent 子进程超时(120s)")
	})

	t.Run("极简上下文降级依然产出合法完整结构", func(t *testing.T) {
		minimalCtx := &prompts.VideoDecisionContext{
			SourcePlatform: "twitch",
			SourceVideoID:  "tw_stream_01",
		}

		res := prompts.GenerateFallbackDecision(minimalCtx, "LLM Key 欠费 402")
		require.NotNil(t, res)

		assert.True(t, res.Degraded)
		assert.NotEmpty(t, res.Title, "即使极简上下文也应生成保底标题")
		assert.NotEmpty(t, res.Desc)
		assert.NotEmpty(t, res.Tags)
		assert.Equal(t, prompts.DefaultCategoryID, res.TID)
	})
}

// ============================================================================
// 6. 提示词生成器测试 (Prompt Template Rendering)
// ============================================================================

func TestDecisionAgent_PromptTemplates(t *testing.T) {
	ctx := &prompts.VideoDecisionContext{
		SourcePlatform:       "youtube",
		SourceVideoID:        "abc_test",
		OriginalTitle:        "How Linux Works",
		OriginalChannel:      "KernelMaster",
		DurationSeconds:      642,
		SourceLanguage:       "en",
		HasSubtitles:         true,
		TranscriptSummary:    "讲解了虚拟内存与文件描述符的核心机制。",
		TargetAccountName:    "Linux极客总舵",
		TargetAccountPersona: "严谨求实的底层操作系统专家",
		DynamicTitleTemplate: "【硬核底层】{title}",
		CategoryID:           188,
		Copyright:            2,
		DefaultTags:          []string{"Linux", "操作系统"},
	}

	sysPrompt := prompts.BuildDecisionSystemPrompt(ctx.TargetAccountPersona)
	assert.Contains(t, sysPrompt, "严谨求实的底层操作系统专家")
	assert.Contains(t, sysPrompt, "80 个字符")
	assert.Contains(t, sysPrompt, "JSON Schema")

	userPrompt := prompts.BuildDecisionUserPrompt(ctx)
	assert.Contains(t, userPrompt, "How Linux Works")
	assert.Contains(t, userPrompt, "KernelMaster")
	assert.Contains(t, userPrompt, "10:42")
	assert.Contains(t, userPrompt, "虚拟内存与文件描述符")
	assert.Contains(t, userPrompt, "【硬核底层】{title}")
	assert.Contains(t, userPrompt, prompts.DecisionJSONSchema)
}
