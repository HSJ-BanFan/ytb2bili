package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/difyz9/ytb2bili/internal/chain_task/base"
	"github.com/difyz9/ytb2bili/internal/chain_task/manager"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/pkg/cos"
	"github.com/difyz9/ytb2bili/pkg/prompts"
	"github.com/difyz9/ytb2bili/pkg/utils"
	"gorm.io/gorm"
)

type TranslateSubtitle struct {
	base.BaseTask
	App          *core.AppServer
	DB           *gorm.DB
	APIKey       string
	GroupSize    int
	MaxWorkers   int // 最大并发数
	AIManager    *services.AIServiceManager
	LastProvider services.AIProvider // 记录最后使用的AI提供商
}

func NewTranslateSubtitle(name string, app *core.AppServer, stateManager *manager.StateManager, client *cos.CosClient, db *gorm.DB, apiKey string) *TranslateSubtitle {
	// 创建AI服务管理器
	aiManager := services.NewAIServiceManager(app.Config, app.Logger)

	return &TranslateSubtitle{
		BaseTask: base.BaseTask{
			Name:         name,
			StateManager: stateManager,
			Client:       client,
		},
		App:        app,
		DB:         db,
		APIKey:     "", // 不再固化API Key，运行时动态获取
		GroupSize:  25, // 每组25句，减少API调用次数
		MaxWorkers: 3,  // 最多3个并发，避免API限制
		AIManager:  aiManager,
	}
}

// getCurrentAIProvider 获取当前可用的AI服务提供商
func (t *TranslateSubtitle) getCurrentAIProvider() (services.AIProvider, error) {
	// 刷新配置
	t.AIManager.RefreshConfig(t.App.Config)

	// 获取首选提供商
	provider, err := t.AIManager.GetPreferredProvider()
	if err != nil {
		return "", fmt.Errorf("没有可用的AI服务: %v", err)
	}

	return provider, nil
}

// getCurrentAPIKey 获取当前的API Key（兼容旧代码）
func (t *TranslateSubtitle) getCurrentAPIKey() (string, error) {
	// 优先使用OpenAI兼容API
	if t.AIManager.IsOpenAICompatibleEnabled() {
		cfg := t.AIManager.GetOpenAICompatibleConfig()
		return cfg.ApiKey, nil
	}

	// 备选：DeepSeek
	if t.AIManager.IsDeepSeekEnabled() {
		cfg := t.AIManager.GetDeepSeekConfig()
		return cfg.ApiKey, nil
	}

	return "", fmt.Errorf("没有可用的AI服务，请先配置AI服务")
}

// SRTEntry SRT字幕条目
type SRTEntry struct {
	Index    int
	TimeCode string
	Text     string
}

func (t *TranslateSubtitle) Execute(context map[string]interface{}) bool {
	t.App.Logger.Info("========================================")
	t.App.Logger.Infof("开始翻译字幕: VideoID=%s", t.StateManager.VideoID)
	t.App.Logger.Info("========================================")

	// 0. 优先检查是否已存在 YouTube 官方翻译的中文字幕
	if existingZhSrt := t.findExistingChineseSubtitle(); existingZhSrt != "" {
		t.App.Logger.Infof("✅ 发现 YouTube 官方中文字幕: %s", filepath.Base(existingZhSrt))
		t.App.Logger.Info("✅ 跳过 AI 翻译，直接使用官方字幕")

		// 复制为 zh.srt 供后续任务使用
		zhSRTPath := filepath.Join(t.StateManager.CurrentDir, "zh.srt")
		if err := t.copyFile(existingZhSrt, zhSRTPath); err != nil {
			t.App.Logger.Warnf("⚠️ 复制字幕文件失败: %v，将继续使用原文件", err)
			zhSRTPath = existingZhSrt
		} else {
			t.App.Logger.Infof("✓ 已复制为: %s", filepath.Base(zhSRTPath))
		}

		// 清理冗余字幕文件
		t.cleanupRedundantSubtitles(existingZhSrt)

		context["zh_srt_path"] = zhSRTPath
		context["subtitle_source"] = "youtube_official"
		t.App.Logger.Info("========================================")
		return true
	}

	// 1. 刷新AI服务管理器配置并获取首选AI服务
	t.AIManager.RefreshConfig(t.App.Config)
	provider, err := t.getCurrentAIProvider()
	if err != nil {
		t.App.Logger.Errorf("❌ %v", err)
		context["error"] = t.getTranslationError(err)
		return false
	}

	// 记录使用的AI服务
	t.LastProvider = provider
	status := t.AIManager.GetStatus(provider)
	t.App.Logger.Infof("🤖 使用AI服务: %s (模型: %s)", status.Name, status.Model)

	// 1. 检查英文字幕文件是否存在（由 GenerateSubtitles 任务生成）
	// 优先查找 en.srt（yt-dlp 下载后复制的），然后是 {videoID}.srt
	enSRTPath := t.findEnglishSubtitle()
	if enSRTPath == "" {
		t.App.Logger.Warn("⚠️  英文字幕文件不存在，跳过翻译")
		return true // 没有字幕文件不算失败
	}
	t.App.Logger.Infof("  │ 📄 使用字幕: %s", filepath.Base(enSRTPath))

	// 2. 读取并解析英文字幕文件
	srtContent, err := os.ReadFile(enSRTPath)
	if err != nil {
		t.App.Logger.Errorf("❌ 读取英文字幕文件失败: %v", err)
		context["error"] = "字幕文件读取失败，请确认字幕生成步骤已完成"
		return false
	}

	srtEntries, err := t.parseSRTContent(string(srtContent))
	if err != nil {
		t.App.Logger.Errorf("❌ 解析SRT文件失败: %v", err)
		context["error"] = "字幕文件格式错误，无法解析SRT内容"
		return false
	}

	if len(srtEntries) == 0 {
		t.App.Logger.Warn("⚠️  字幕内容为空，跳过翻译")
		return true
	}

	t.App.Logger.Infof("📝 找到 %d 条字幕", len(srtEntries))

	// 3. 提取文本进行翻译
	var texts []string
	for _, entry := range srtEntries {
		texts = append(texts, entry.Text)
	}

	// 4. 执行并发翻译
	totalGroups := (len(texts) + t.GroupSize - 1) / t.GroupSize
	t.App.Logger.Infof("� 开始并发翻译，每组 %d 句，共 %d 组，并发数: %d", t.GroupSize, totalGroups, t.MaxWorkers)

	translatedTexts, err := t.translateTextsInGroupsConcurrent(texts)
	if err != nil {
		t.App.Logger.Errorf("❌ 翻译失败: %v", err)
		context["error"] = t.getTranslationError(err)
		return false
	}

	// 5. 生成中文字幕SRT
	translatedSRT := t.generateTranslatedSRTContent(srtEntries, translatedTexts)

	// 6. 保存中文字幕文件
	zhSRTPath := filepath.Join(t.StateManager.CurrentDir, "zh.srt")
	if err := os.WriteFile(zhSRTPath, []byte(translatedSRT), 0644); err != nil {
		t.App.Logger.Errorf("❌ 保存中文字幕失败: %v", err)
		context["error"] = "保存翻译字幕文件失败，请检查磁盘空间和文件权限"
		return false
	}

	// 7. 字幕质量校验和优化
	optimizedPath, validationResult, err := t.validateAndOptimizeSubtitles(enSRTPath, zhSRTPath)
	if err != nil {
		t.App.Logger.Warnf("⚠️  字幕校验失败，使用原始翻译: %v", err)
	} else {
		if validationResult.MissingEntries > 0 {
			t.App.Logger.Infof("🔧 检测到 %d 个问题条目，已尝试修复 %d 个",
				validationResult.MissingEntries, len(validationResult.FixedEntries))

			if optimizedPath != "" {
				// 使用优化后的文件替换原文件
				if err := os.Rename(optimizedPath, zhSRTPath); err == nil {
					t.App.Logger.Info("✨ 已应用字幕优化结果")
				}
			}
		}
	}

	// 8. 保存文件路径到 context
	context["en_srt_path"] = enSRTPath
	context["zh_srt_path"] = zhSRTPath
	context["translated_count"] = len(translatedTexts)

	// 添加校验结果信息
	if validationResult != nil {
		context["validation_result"] = map[string]interface{}{
			"total_entries":   validationResult.TotalEntries,
			"valid_entries":   validationResult.ValidEntries,
			"missing_entries": validationResult.MissingEntries,
			"fixed_entries":   len(validationResult.FixedEntries),
		}
	}

	t.App.Logger.Infof("✓ 中文字幕已保存: %s", zhSRTPath)
	t.App.Logger.Infof("✓ 翻译完成: %d/%d 条字幕", len(translatedTexts), len(texts))
	t.App.Logger.Info("========================================")

	return true
}

// parseSRTContent 解析SRT文件内容
func (t *TranslateSubtitle) parseSRTContent(content string) ([]SRTEntry, error) {
	lines := strings.Split(content, "\n")
	var entries []SRTEntry
	var currentEntry SRTEntry
	var textLines []string
	stage := 0 // 0=等待序号, 1=等待时间码, 2=读取文本

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			// 空行表示一个条目结束
			if stage == 2 && len(textLines) > 0 {
				currentEntry.Text = strings.Join(textLines, "\n")
				entries = append(entries, currentEntry)
				textLines = nil
				stage = 0
			}
			continue
		}

		switch stage {
		case 0: // 读取序号
			var index int
			if _, err := fmt.Sscanf(line, "%d", &index); err == nil {
				currentEntry = SRTEntry{Index: index}
				stage = 1
			}
		case 1: // 读取时间码
			if strings.Contains(line, "-->") {
				currentEntry.TimeCode = line
				stage = 2
			}
		case 2: // 读取文本
			textLines = append(textLines, line)
		}
	}

	// 处理最后一个条目（如果文件末尾没有空行）
	if stage == 2 && len(textLines) > 0 {
		currentEntry.Text = strings.Join(textLines, "\n")
		entries = append(entries, currentEntry)
	}

	return entries, nil
}

// generateTranslatedSRTContent 生成翻译后的SRT内容（保持原时间轴）
func (t *TranslateSubtitle) generateTranslatedSRTContent(entries []SRTEntry, translatedTexts []string) string {
	var builder strings.Builder

	for i, entry := range entries {
		builder.WriteString(fmt.Sprintf("%d\n", entry.Index))
		builder.WriteString(fmt.Sprintf("%s\n", entry.TimeCode))

		if i < len(translatedTexts) {
			builder.WriteString(fmt.Sprintf("%s\n\n", translatedTexts[i]))
		} else {
			builder.WriteString(fmt.Sprintf("%s\n\n", entry.Text))
		}
	}

	return builder.String()
}

// translateTextsInGroupsConcurrent 并发分组翻译文本
func (t *TranslateSubtitle) translateTextsInGroupsConcurrent(texts []string) ([]string, error) {
	totalGroups := (len(texts) + t.GroupSize - 1) / t.GroupSize
	results := make([][]string, totalGroups)

	// 创建工作池
	type translateTask struct {
		groupIndex int
		texts      []string
	}

	taskChannel := make(chan translateTask, totalGroups)
	resultChannel := make(chan struct {
		groupIndex int
		result     []string
		err        error
	}, totalGroups)

	// 启动工作者
	var wg sync.WaitGroup
	workerCount := t.MaxWorkers
	if workerCount > totalGroups {
		workerCount = totalGroups
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			t.App.Logger.Debugf("🔧 启动翻译工作者 %d", workerID)

			for task := range taskChannel {
				t.App.Logger.Infof("⏳ 工作者 %d 处理第 %d/%d 组 (%d句)",
					workerID, task.groupIndex+1, totalGroups, len(task.texts))

				// 使用简化的翻译方法
				translated, err := t.translateGroupSimple(task.texts)

				resultChannel <- struct {
					groupIndex int
					result     []string
					err        error
				}{
					groupIndex: task.groupIndex,
					result:     translated,
					err:        err,
				}
			}
		}(i)
	}

	// 分发任务
	go func() {
		for i := 0; i < len(texts); i += t.GroupSize {
			end := i + t.GroupSize
			if end > len(texts) {
				end = len(texts)
			}

			taskChannel <- translateTask{
				groupIndex: i / t.GroupSize,
				texts:      texts[i:end],
			}
		}
		close(taskChannel)
	}()

	// 收集结果
	go func() {
		wg.Wait()
		close(resultChannel)
	}()

	// 处理结果
	var lastErr error
	for result := range resultChannel {
		if result.err != nil {
			t.App.Logger.Errorf("❌ 第 %d 组翻译失败: %v", result.groupIndex+1, result.err)
			lastErr = result.err
			continue
		}
		results[result.groupIndex] = result.result
	}

	if lastErr != nil {
		return nil, lastErr
	}

	// 合并结果
	var allTranslated []string
	for _, groupResult := range results {
		allTranslated = append(allTranslated, groupResult...)
	}

	return allTranslated, nil
}

// translateGroupSimple 简化的组翻译（无上下文，更快速）
func (t *TranslateSubtitle) translateGroupSimple(texts []string) ([]string, error) {
	if len(texts) == 0 {
		return []string{}, nil
	}

	// 直接组合文本
	combinedText := strings.Join(texts, "\n###SENTENCE_BREAK###\n")

	// 从提示词管理器获取提示词
	promptManager := prompts.GetGlobalManager()
	systemPrompt, err := promptManager.RenderSystemPrompt(prompts.PromptTranslateSubtitle, &prompts.PromptParams{
		Count: len(texts),
	})
	if err != nil {
		// 回退到默认提示词
		systemPrompt = fmt.Sprintf(`你是一个专业的视频字幕翻译专家。将给出的 %d 句英文字幕翻译成中文。

翻译要求：
1. 自然流畅：使用口语化表达，符合中文字幕习惯
2. 准确传神：忠实原文含义，保持语气和情感
3. 简洁明了：字幕需要快速阅读，避免冗长
4. 数量严格：必须输出 %d 句翻译，不多不少
5. 分隔符：每句翻译用"###SENTENCE_BREAK###"分隔

注意：只返回翻译的中文文本，不要添加序号、解释或其他内容。`, len(texts), len(texts))
	}

	translatedText, err := t.callDeepSeekAPI(systemPrompt, combinedText)
	if err != nil {
		return nil, err
	}

	translatedSentences := strings.Split(translatedText, "###SENTENCE_BREAK###")

	// 清理和验证
	for i := range translatedSentences {
		translatedSentences[i] = strings.TrimSpace(translatedSentences[i])
	}

	// 确保数量匹配
	if len(translatedSentences) != len(texts) {
		t.App.Logger.Warnf("⚠️  翻译结果数量不匹配: 期望%d句，实际%d句，正在修正...", len(texts), len(translatedSentences))
		for len(translatedSentences) < len(texts) {
			translatedSentences = append(translatedSentences, "[翻译缺失]")
		}
		if len(translatedSentences) > len(texts) {
			translatedSentences = translatedSentences[:len(texts)]
		}
	}

	return translatedSentences, nil
}

// translateTextsInGroups 分组翻译文本（带上下文）- 保留原方法作为备用
func (t *TranslateSubtitle) translateTextsInGroups(texts []string) ([]string, error) {
	var translatedTexts []string
	totalGroups := (len(texts) + t.GroupSize - 1) / t.GroupSize

	for i := 0; i < len(texts); i += t.GroupSize {
		groupNum := (i / t.GroupSize) + 1
		end := i + t.GroupSize
		if end > len(texts) {
			end = len(texts)
		}

		currentGroup := texts[i:end]

		// 准备上下文窗口
		var prevContext, nextContext []string
		contextSize := 2 // 前后各取2句作为上下文

		// 获取前置上下文
		if i > 0 {
			prevStart := i - contextSize
			if prevStart < 0 {
				prevStart = 0
			}
			prevContext = texts[prevStart:i]
		}

		// 获取后置上下文
		if end < len(texts) {
			nextEnd := end + contextSize
			if nextEnd > len(texts) {
				nextEnd = len(texts)
			}
			nextContext = texts[end:nextEnd]
		}

		t.App.Logger.Infof("⏳ 翻译第 %d/%d 组 (上下文: 前%d句, 当前%d句, 后%d句)",
			groupNum, totalGroups, len(prevContext), len(currentGroup), len(nextContext))

		// 带上下文翻译
		groupTranslated, err := t.translateGroupWithContext(currentGroup, prevContext, nextContext)
		if err != nil {
			return nil, fmt.Errorf("翻译第 %d 组失败: %v", groupNum, err)
		}

		translatedTexts = append(translatedTexts, groupTranslated...)

		// 移除组间延迟，改为根据需要动态调整
		// 如果遇到API限制，可以在错误处理中添加重试和延迟
	}

	return translatedTexts, nil
}

// translateGroupWithContext 带上下文翻译一组文本
func (t *TranslateSubtitle) translateGroupWithContext(texts []string, prevContext []string, nextContext []string) ([]string, error) {
	// 构建包含上下文的完整文本
	var fullTexts []string
	targetStartIndex := 0

	// 添加前置上下文
	if len(prevContext) > 0 {
		fullTexts = append(fullTexts, prevContext...)
		targetStartIndex = len(fullTexts)
	}

	// 添加目标翻译文本
	fullTexts = append(fullTexts, texts...)
	targetEndIndex := len(fullTexts)

	// 添加后置上下文
	if len(nextContext) > 0 {
		fullTexts = append(fullTexts, nextContext...)
	}

	combinedText := strings.Join(fullTexts, "\n###SENTENCE_BREAK###\n")

	// 构建系统提示
	contextInfo := ""
	if len(prevContext) > 0 || len(nextContext) > 0 {
		contextInfo = fmt.Sprintf(`

上下文信息：
- 前置上下文：%d 句（仅供参考，不需要翻译）
- 目标翻译：%d 句（位于第 %d-%d 句，需要全部翻译）
- 后置上下文：%d 句（仅供参考，不需要翻译）

请只翻译目标部分（第 %d-%d 句），但要充分考虑前后文的连贯性。`,
			len(prevContext), len(texts), targetStartIndex+1, targetEndIndex,
			len(nextContext), targetStartIndex+1, targetEndIndex)
	}

	// 从提示词管理器获取提示词
	promptManager := prompts.GetGlobalManager()
	systemPrompt, err := promptManager.RenderSystemPrompt(prompts.PromptTranslateContext, &prompts.PromptParams{
		Count: len(texts),
	})
	if err != nil {
		// 回退到默认提示词
		systemPrompt = fmt.Sprintf(`你是一个专业的视频字幕翻译专家。我将给你一段连续的英文字幕，其中包含 %d 句需要翻译的内容。%s

翻译要求：
1. 自然流畅：使用口语化表达，符合中文字幕习惯
2. 上下文连贯：理解整体语境，确保翻译前后呼应
3. 准确传神：忠实原文含义，保持语气和情感
4. 简洁明了：字幕需要快速阅读，避免冗长
5. 数量严格：必须输出 %d 句翻译，不多不少
6. 分隔符：每句翻译用"###SENTENCE_BREAK###"分隔

注意：只返回翻译的中文文本，不要添加序号、解释或其他内容。`, len(texts), contextInfo, len(texts))
	} else {
		// 添加上下文信息到提示词
		systemPrompt = systemPrompt + contextInfo
	}

	translatedText, err := t.callDeepSeekAPI(systemPrompt, combinedText)
	if err != nil {
		return nil, err
	}

	translatedSentences := strings.Split(translatedText, "###SENTENCE_BREAK###")

	// 清理和验证
	for i := range translatedSentences {
		translatedSentences[i] = strings.TrimSpace(translatedSentences[i])
	}

	// 确保数量匹配
	if len(translatedSentences) != len(texts) {
		t.App.Logger.Warnf("⚠️  翻译结果数量不匹配: 期望%d句，实际%d句，正在修正...", len(texts), len(translatedSentences))
		for len(translatedSentences) < len(texts) {
			translatedSentences = append(translatedSentences, "[翻译缺失]")
		}
		if len(translatedSentences) > len(texts) {
			translatedSentences = translatedSentences[:len(texts)]
		}
	}

	return translatedSentences, nil
}

// callDeepSeekAPI 调用AI API（使用AI服务管理器，支持自动故障转移）
func (t *TranslateSubtitle) callDeepSeekAPI(systemPrompt, userPrompt string) (string, error) {
	// 使用AI服务管理器进行调用（自动选择首选服务，失败时自动切换）
	response, provider, err := t.AIManager.ChatCompletion(systemPrompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("AI服务调用失败: %v", err)
	}

	// 记录实际使用的提供商
	if provider != t.LastProvider {
		t.App.Logger.Infof("🔄 AI服务已切换: %s -> %s", t.LastProvider, provider)
		t.LastProvider = provider
	}

	return response, nil
}

// getTranslationError 将翻译错误转换为用户友好的错误信息
func (t *TranslateSubtitle) getTranslationError(err error) string {
	errorStr := err.Error()

	if strings.Contains(errorStr, "没有可用的AI服务") {
		return "翻译失败：没有可用的AI服务，请在设置中配置AI服务（首选OpenAI兼容API或DeepSeek）"
	}

	if strings.Contains(errorStr, "API Key 未配置") || strings.Contains(errorStr, "API Key未配置") {
		return "翻译失败：AI服务API Key未配置，请在设置中配置API Key"
	}

	if strings.Contains(errorStr, "401") || strings.Contains(errorStr, "unauthorized") {
		return "翻译失败：AI服务API Key无效或已过期，请检查API Key设置"
	}

	if strings.Contains(errorStr, "429") || strings.Contains(errorStr, "rate limit") {
		return "翻译失败：API调用频率过快，请稍后重试"
	}

	if strings.Contains(errorStr, "insufficient_quota") || strings.Contains(errorStr, "quota") {
		return "翻译失败：DeepSeek账户余额不足，请充值后重试"
	}

	if strings.Contains(errorStr, "timeout") || strings.Contains(errorStr, "deadline exceeded") {
		return "翻译失败：网络超时，请检查网络连接后重试"
	}

	if strings.Contains(errorStr, "connection") {
		return "翻译失败：网络连接异常，请检查网络状态"
	}

	if strings.Contains(errorStr, "max_tokens") {
		return "翻译失败：字幕内容过长，请尝试分段处理"
	}

	if strings.Contains(errorStr, "context_length_exceeded") {
		return "翻译失败：单次翻译内容过多，系统将自动分批重试"
	}

	if strings.Contains(errorStr, "API Key") {
		return "翻译失败：API Key配置问题，请检查设置"
	}

	// 通用翻译错误
	return "翻译失败：AI翻译服务暂时不可用，请稍后重试"
}

// maskAPIKey 隐藏API Key的敏感信息用于日志显示
func maskAPIKey(apiKey string) string {
	if apiKey == "" {
		return ""
	}
	if len(apiKey) > 10 {
		return apiKey[:6] + "..." + apiKey[len(apiKey)-4:]
	}
	return "***"
}

// validateAndOptimizeSubtitles 校验和优化字幕质量
func (t *TranslateSubtitle) validateAndOptimizeSubtitles(originalPath, translatedPath string) (string, *utils.ValidationResult, error) {
	// 获取当前API Key用于修复
	apiKey, err := t.getCurrentAPIKey()
	if err != nil {
		return "", nil, fmt.Errorf("无法获取API Key进行校验: %v", err)
	}

	// 创建校验器
	validator := utils.NewSubtitleValidator(t.App.Logger, apiKey)

	// 生成优化后的文件路径
	optimizedPath := filepath.Join(t.StateManager.CurrentDir, "zh_optimized.srt")

	// 执行校验和修复
	result, err := validator.ValidateAndFixSubtitles(originalPath, translatedPath, optimizedPath)
	if err != nil {
		return "", nil, err
	}

	// 如果有修复，返回优化文件路径
	if len(result.FixedEntries) > 0 {
		return optimizedPath, result, nil
	}

	// 没有问题或无法修复，返回空路径
	return "", result, nil
}

// findEnglishSubtitle 查找英文字幕文件
// 按优先级查找: en.srt -> {videoID}.srt -> {videoID}.en.srt -> {videoID}.en-*.srt
func (t *TranslateSubtitle) findEnglishSubtitle() string {
	currentDir := t.StateManager.CurrentDir
	videoID := t.StateManager.VideoID

	t.App.Logger.Debugf("🔍 查找字幕目录: %s, VideoID: %s", currentDir, videoID)

	// 优先级1: en.srt（由 GenerateSubtitles 任务复制）
	enPath := filepath.Join(currentDir, "en.srt")
	if _, err := os.Stat(enPath); err == nil {
		t.App.Logger.Debugf("✓ 找到 en.srt: %s", enPath)
		return enPath
	}
	t.App.Logger.Debugf("✗ en.srt 不存在: %s", enPath)

	// 优先级2: {videoID}.srt（数据库字幕生成）
	videoIDPath := filepath.Join(currentDir, fmt.Sprintf("%s.srt", videoID))
	if _, err := os.Stat(videoIDPath); err == nil {
		return videoIDPath
	}

	// 优先级3: 查找 yt-dlp 下载的英文字幕
	entries, err := os.ReadDir(currentDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// 查找 {videoID}.en.srt 或 {videoID}.en-*.srt
		if strings.HasPrefix(name, videoID+".") && strings.HasSuffix(name, ".srt") {
			if strings.Contains(name, ".en.") || strings.Contains(name, ".en-") {
				return filepath.Join(currentDir, name)
			}
		}
	}

	// 优先级4: 任意 srt 文件
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".srt") {
			return filepath.Join(currentDir, entry.Name())
		}
	}

	return ""
}

// findExistingChineseSubtitle 查找已存在的 YouTube 官方中文字幕
// 按优先级查找: {videoID}.zh-Hans.srt -> {videoID}.zh-CN.srt -> {videoID}.zh.srt
func (t *TranslateSubtitle) findExistingChineseSubtitle() string {
	currentDir := t.StateManager.CurrentDir
	videoID := t.StateManager.VideoID

	// 优先级列表：简体中文 > 繁体中文
	priorities := []string{
		fmt.Sprintf("%s.zh-Hans.srt", videoID), // 简体中文
		fmt.Sprintf("%s.zh-CN.srt", videoID),   // 简体中文（另一种格式）
		fmt.Sprintf("%s.zh.srt", videoID),      // 中文（通用）
	}

	for _, filename := range priorities {
		fullPath := filepath.Join(currentDir, filename)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			// 检查文件大小，确保不是空文件
			if info.Size() > 100 {
				return fullPath
			}
		}
	}

	return ""
}

// copyFile 复制文件
func (t *TranslateSubtitle) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = destFile.ReadFrom(sourceFile)
	return err
}

// cleanupRedundantSubtitles 清理冗余的字幕文件
// 保留: zh.srt, en.srt, {videoID}.zh-Hans.srt, {videoID}.en.srt
func (t *TranslateSubtitle) cleanupRedundantSubtitles(keepZhSrt string) {
	currentDir := t.StateManager.CurrentDir
	videoID := t.StateManager.VideoID

	entries, err := os.ReadDir(currentDir)
	if err != nil {
		t.App.Logger.Warnf("⚠️ 读取目录失败，跳过清理: %v", err)
		return
	}

	// 需要保留的文件
	keepFiles := map[string]bool{
		"zh.srt":                               true,
		"en.srt":                               true,
		filepath.Base(keepZhSrt):               true,
		fmt.Sprintf("%s.en.srt", videoID):      true,
		fmt.Sprintf("%s.zh-Hans.srt", videoID): true,
	}

	var deletedCount int
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()

		// 只处理字幕文件
		if !strings.HasSuffix(name, ".srt") && !strings.HasSuffix(name, ".vtt") {
			continue
		}

		// 检查是否需要保留
		if keepFiles[name] {
			continue
		}

		// 删除冗余字幕文件
		fullPath := filepath.Join(currentDir, name)
		if err := os.Remove(fullPath); err != nil {
			t.App.Logger.Warnf("⚠️ 删除冗余字幕失败: %s - %v", name, err)
		} else {
			deletedCount++
			t.App.Logger.Debugf("🗑️ 已删除冗余字幕: %s", name)
		}
	}

	if deletedCount > 0 {
		t.App.Logger.Infof("🧹 已清理 %d 个冗余字幕文件", deletedCount)
	}
}
