package prompts

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() 失败: %v", err)
	}
	if m == nil {
		t.Fatal("NewManager() 返回 nil")
	}
}

func TestGetPrompt(t *testing.T) {
	m, _ := NewManager()

	testCases := []PromptType{
		PromptMetadataVideo,
		PromptMetadataImage,
		PromptMetadataText,
		PromptMetadataFallback,
		PromptTranslateSubtitle,
		PromptTranslateContext,
		PromptFixSubtitle,
		PromptDetectLanguage,
	}

	for _, pt := range testCases {
		t.Run(string(pt), func(t *testing.T) {
			prompt, err := m.GetPrompt(pt)
			if err != nil {
				t.Errorf("GetPrompt(%s) 失败: %v", pt, err)
			}
			if prompt == nil {
				t.Errorf("GetPrompt(%s) 返回 nil", pt)
			}
			if prompt.System == "" {
				t.Errorf("GetPrompt(%s) 系统提示词为空", pt)
			}
		})
	}
}

func TestRenderSystemPrompt(t *testing.T) {
	m, _ := NewManager()

	// 测试不带参数的渲染
	system, err := m.RenderSystemPrompt(PromptMetadataVideo, nil)
	if err != nil {
		t.Errorf("RenderSystemPrompt() 失败: %v", err)
	}
	if system == "" {
		t.Error("RenderSystemPrompt() 返回空字符串")
	}

	// 测试带参数的渲染
	params := &PromptParams{
		Count: 5,
		Text:  "测试文本",
	}
	system2, err := m.RenderSystemPrompt(PromptTranslateSubtitle, params)
	if err != nil {
		t.Errorf("RenderSystemPrompt() with params 失败: %v", err)
	}
	if system2 == "" {
		t.Error("RenderSystemPrompt() with params 返回空字符串")
	}
}

func TestRenderUserPrompt(t *testing.T) {
	m, _ := NewManager()

	params := &PromptParams{
		SubtitleText: "这是一段测试字幕内容",
	}

	user, err := m.RenderUserPrompt(PromptMetadataFallback, params)
	if err != nil {
		t.Errorf("RenderUserPrompt() 失败: %v", err)
	}
	if user == "" {
		t.Error("RenderUserPrompt() 返回空字符串")
	}
	t.Logf("渲染后的用户提示词: %s", user[:50]+"...")
}

func TestSeparatorFunctions(t *testing.T) {
	texts := []string{"第一句", "第二句", "第三句"}

	// 测试连接
	joined := JoinWithSeparator(texts)
	if joined == "" {
		t.Error("JoinWithSeparator() 返回空字符串")
	}

	// 测试分割
	split := SplitBySeparator(joined)
	if len(split) != len(texts) {
		t.Errorf("SplitBySeparator() 返回 %d 个元素，期望 %d 个", len(split), len(texts))
	}

	for i, s := range split {
		if s != texts[i] {
			t.Errorf("SplitBySeparator()[%d] = %q, 期望 %q", i, s, texts[i])
		}
	}
}

func TestUpdatePrompt(t *testing.T) {
	m, _ := NewManager()

	// 获取原始值
	original, _ := m.GetPrompt(PromptDetectLanguage)
	originalSystem := original.System

	// 更新提示词
	newSystem := "新的系统提示词"
	err := m.UpdatePrompt(PromptDetectLanguage, newSystem, "", 0.5, 100)
	if err != nil {
		t.Errorf("UpdatePrompt() 失败: %v", err)
	}

	// 验证更新
	updated, _ := m.GetPrompt(PromptDetectLanguage)
	if updated.System != newSystem {
		t.Errorf("UpdatePrompt() 未更新系统提示词")
	}
	if updated.Temperature != 0.5 {
		t.Errorf("UpdatePrompt() 未更新温度: got %f, want 0.5", updated.Temperature)
	}

	// 恢复原始值
	m.UpdatePrompt(PromptDetectLanguage, originalSystem, "", 0.1, 50)
}

func TestExportToMap(t *testing.T) {
	m, _ := NewManager()

	exported := m.ExportToMap()
	if len(exported) == 0 {
		t.Error("ExportToMap() 返回空 map")
	}

	// 验证包含所有类型
	expectedTypes := []string{
		"metadata_video",
		"metadata_image",
		"metadata_text",
		"metadata_fallback",
		"translate_subtitle",
		"translate_context",
		"fix_subtitle",
		"detect_language",
	}

	for _, pt := range expectedTypes {
		if _, ok := exported[pt]; !ok {
			t.Errorf("ExportToMap() 缺少类型: %s", pt)
		}
	}
}
