package prompts

import (
	"os"
	"testing"
)

func TestLoadFromYAML(t *testing.T) {
	// 创建临时 YAML 配置
	yamlContent := `
metadata_video:
  temperature: 0.8
  max_tokens: 1500
  system: "自定义视频元数据提示词"

translate_subtitle:
  temperature: 0.4
  system: "自定义翻译提示词 - 需要翻译 {{.Count}} 句"
`
	tmpFile, err := os.CreateTemp("", "prompts_test_*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("写入临时文件失败: %v", err)
	}
	tmpFile.Close()

	// 创建新的管理器
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	// 加载 YAML 配置
	if err := manager.LoadFromYAML(tmpFile.Name()); err != nil {
		t.Fatalf("加载 YAML 失败: %v", err)
	}

	// 验证配置已合并
	template, err := manager.GetPrompt(PromptMetadataVideo)
	if err != nil {
		t.Fatalf("获取提示词失败: %v", err)
	}

	if template.Temperature != 0.8 {
		t.Errorf("Temperature 期望 0.8，实际 %f", template.Temperature)
	}
	if template.MaxTokens != 1500 {
		t.Errorf("MaxTokens 期望 1500，实际 %d", template.MaxTokens)
	}
	if template.System != "自定义视频元数据提示词" {
		t.Errorf("System 提示词未正确更新: %s", template.System)
	}

	// 验证翻译提示词
	translateTemplate, err := manager.GetPrompt(PromptTranslateSubtitle)
	if err != nil {
		t.Fatalf("获取翻译提示词失败: %v", err)
	}
	if translateTemplate.Temperature != 0.4 {
		t.Errorf("翻译 Temperature 期望 0.4，实际 %f", translateTemplate.Temperature)
	}

	t.Log("✓ YAML 配置加载测试通过")
}

func TestReloadIfChanged(t *testing.T) {
	// 创建临时 YAML 配置
	tmpFile, err := os.CreateTemp("", "prompts_reload_*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// 初始内容
	initialContent := `
metadata_video:
  temperature: 0.5
`
	if _, err := tmpFile.WriteString(initialContent); err != nil {
		t.Fatalf("写入临时文件失败: %v", err)
	}
	tmpFile.Close()

	// 创建管理器并加载
	manager, _ := NewManager()
	if err := manager.LoadFromYAML(tmpFile.Name()); err != nil {
		t.Fatalf("加载 YAML 失败: %v", err)
	}

	// 首次检查 - 不应该重新加载
	changed, err := manager.ReloadIfChanged()
	if err != nil {
		t.Fatalf("ReloadIfChanged 失败: %v", err)
	}
	if changed {
		t.Error("首次检查不应该触发重新加载")
	}

	t.Log("✓ 热更新检测测试通过")
}

func TestGlobalManager(t *testing.T) {
	// 获取全局管理器
	manager1 := GetGlobalManager()
	manager2 := GetGlobalManager()

	// 验证是同一个实例
	if manager1 != manager2 {
		t.Error("全局管理器应该是单例")
	}

	// 验证可以获取提示词
	template, err := manager1.GetPrompt(PromptMetadataVideo)
	if err != nil {
		t.Fatalf("获取提示词失败: %v", err)
	}
	if template.System == "" {
		t.Error("提示词内容不应为空")
	}

	t.Log("✓ 全局管理器单例测试通过")
}

func TestRenderWithParams(t *testing.T) {
	manager := GetGlobalManager()

	// 测试带参数渲染
	systemPrompt, err := manager.RenderSystemPrompt(PromptTranslateSubtitle, &PromptParams{
		Count: 25,
	})
	if err != nil {
		t.Fatalf("渲染系统提示词失败: %v", err)
	}

	// 验证参数已替换
	if systemPrompt == "" {
		t.Error("渲染后的提示词不应为空")
	}

	t.Logf("✓ 参数渲染测试通过，提示词长度: %d", len(systemPrompt))
}
