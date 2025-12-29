# Gemini 快速集成清单

## ✅ 需要修改的文件清单

### 1. 配置文件 (2个文件)

- [ ] `internal/core/types/app_config.go` - 添加 Gemini 配置结构
- [ ] `config.toml.example` - 添加配置示例

### 2. 新建文件 (2个文件)

- [ ] `internal/chain_task/handlers/gemini_client.go` - Gemini API 客户端
- [ ] `internal/chain_task/handlers/ai_client.go` - 统一 AI 接口

### 3. 修改现有文件 (3个文件)

- [ ] `internal/chain_task/handlers/translate_subtitle.go` - 使用统一接口
- [ ] `internal/chain_task/handlers/generate_metadata.go` - 使用统一接口
- [ ] `internal/handler/config_handler.go` - 添加 Gemini 配置 API

## 📝 修改要点

### app_config.go 需要添加的代码

```go
// 1. 添加 GeminiConfig 结构（在 DeepSeekTransConfig 后面）
type GeminiConfig struct {
	Enabled   bool   `toml:"enabled"`
	ApiKey    string `toml:"api_key"`
	Model     string `toml:"model"`
	Endpoint  string `toml:"endpoint"`
	Timeout   int    `toml:"timeout"`
	MaxTokens int    `toml:"max_tokens"`
}

// 2. 添加 AIProviderConfig 结构
type AIProviderConfig struct {
	DefaultProvider   string   `toml:"default_provider"`
	FallbackProviders []string `toml:"fallback_providers"`
	MaxRetries        int      `toml:"max_retries"`
}

// 3. 在 AppConfig 中添加字段
type AppConfig struct {
	// ... 现有字段 ...
	GeminiConfig     *GeminiConfig     `toml:"GeminiConfig"`
	AIProviderConfig *AIProviderConfig `toml:"AIProviderConfig"`
}

// 4. 在 NewDefaultConfig() 中添加默认值
GeminiConfig: &GeminiConfig{
	Enabled:   false,
	ApiKey:    "",
	Model:     "gemini-pro",
	Endpoint:  "https://generativelanguage.googleapis.com/v1beta",
	Timeout:   60,
	MaxTokens: 4000,
},
AIProviderConfig: &AIProviderConfig{
	DefaultProvider:   "deepseek",
	FallbackProviders: []string{"gemini"},
	MaxRetries:        3,
},
```

### translate_subtitle.go 需要修改的代码

找到创建 DeepSeek 客户端的地方，替换为：

```go
// 旧代码：
// deepseekClient := NewDeepSeekClient(t.App.Config.DeepSeekTransConfig.ApiKey)

// 新代码：
factory := NewAIClientFactory(
	t.App.Config.DeepSeekTransConfig,
	t.App.Config.GeminiConfig,
	t.App.Config.AIProviderConfig,
)

// 使用带回退的调用
result, err := factory.ChatCompletionWithFallback(systemPrompt, userPrompt)
```

### config.toml 配置示例

```toml
# DeepSeek 配置（现有）
[DeepSeekTransConfig]
  enabled = true
  api_key = "sk-your-deepseek-key"
  models = ""
  endpoint = "https://api.deepseek.com"
  timeout = 60
  max_tokens = 4000

# Gemini 配置（新增）
[GeminiConfig]
  enabled = true
  api_key = "your-gemini-api-key"
  model = "gemini-pro"
  endpoint = "https://generativelanguage.googleapis.com/v1beta"
  timeout = 60
  max_tokens = 4000

# AI 提供商配置（新增）
[AIProviderConfig]
  default_provider = "deepseek"     # 默认使用 deepseek
  fallback_providers = ["gemini"]   # 失败时回退到 gemini
  max_retries = 3
```

## 🔗 关键链接

### Gemini API 文档
- **获取 API Key**: https://makersuite.google.com/app/apikey
- **快速开始**: https://ai.google.dev/tutorials/get_started_web
- **API 参考**: https://ai.google.dev/api/rest/v1beta/models/generateContent
- **定价**: https://ai.google.dev/pricing

### 代码参考
- **详细实现指南**: `GEMINI_INTEGRATION_GUIDE.md`
- **完整代码示例**: 见指南中的各个步骤

## 🧪 测试步骤

### 1. 获取 Gemini API Key
```bash
# 访问 https://makersuite.google.com/app/apikey
# 登录 Google 账号
# 创建 API Key
```

### 2. 配置文件
```toml
[GeminiConfig]
  enabled = true
  api_key = "your-actual-api-key"
  model = "gemini-pro"
```

### 3. 测试连接
```go
// 在代码中测试
client := NewGeminiClient("your-api-key", "gemini-pro")
result, err := client.ChatCompletion(
	"You are a translator", 
	"Translate to Chinese: Hello World",
)
fmt.Println(result) // 应该输出：你好世界
```

### 4. 测试自动切换
```toml
# 故意让 DeepSeek 失败（错误的 API Key）
[DeepSeekTransConfig]
  enabled = true
  api_key = "wrong-key"

# Gemini 配置正确
[GeminiConfig]
  enabled = true
  api_key = "correct-gemini-key"

# 配置回退
[AIProviderConfig]
  default_provider = "deepseek"
  fallback_providers = ["gemini"]
```

系统应该自动从 DeepSeek 切换到 Gemini。

## 📊 实现优先级

### 高优先级（核心功能）
1. ✅ `gemini_client.go` - Gemini 客户端实现
2. ✅ `app_config.go` - 配置结构
3. ✅ `ai_client.go` - 统一接口

### 中优先级（集成）
4. ✅ `translate_subtitle.go` - 翻译功能集成
5. ✅ `generate_metadata.go` - 元数据生成集成

### 低优先级（增强）
6. ⭐ `config_handler.go` - API 配置接口
7. ⭐ 前端配置界面

## 💡 实现建议

### 方案 A：最小实现（推荐）
只实现核心功能，不修改前端：
- 添加 Gemini 客户端
- 修改配置文件
- 修改翻译逻辑
- 通过 `config.toml` 手动配置

**优点**：快速实现，风险小
**缺点**：需要手动编辑配置文件

### 方案 B：完整实现
包含前端配置界面：
- 所有方案 A 的内容
- 添加前端配置页面
- 添加 API 接口

**优点**：用户体验好
**缺点**：工作量大

## 🎯 预期效果

### 配置示例 1：只用 Gemini
```toml
[DeepSeekTransConfig]
  enabled = false

[GeminiConfig]
  enabled = true
  api_key = "your-key"

[AIProviderConfig]
  default_provider = "gemini"
```

### 配置示例 2：DeepSeek 主，Gemini 备
```toml
[DeepSeekTransConfig]
  enabled = true
  api_key = "deepseek-key"

[GeminiConfig]
  enabled = true
  api_key = "gemini-key"

[AIProviderConfig]
  default_provider = "deepseek"
  fallback_providers = ["gemini"]
```

### 配置示例 3：Gemini 主，DeepSeek 备
```toml
[AIProviderConfig]
  default_provider = "gemini"
  fallback_providers = ["deepseek"]
```

## ⏱️ 预估工作量

- **核心实现**：2-3 小时
- **测试调试**：1-2 小时
- **文档更新**：0.5 小时
- **总计**：3.5-5.5 小时

## 🚀 开始实现

按照 `GEMINI_INTEGRATION_GUIDE.md` 中的步骤逐步实现即可！
