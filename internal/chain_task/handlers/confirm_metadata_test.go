package handlers

import (
	"testing"

	"github.com/difyz9/ytb2bili/pkg/store/model"
)

// TestApplyTemplate 验证模板替换逻辑
func TestApplyTemplate(t *testing.T) {
	// 使用空结构体，因为 applyTemplate 不依赖 handler 的其他状态
	handler := &ConfirmMetadata{}

	tests := []struct {
		name     string
		template string
		video    *model.SavedVideo
		want     string
	}{
		{
			name:     "Basic Replace",
			template: "{original_title} | {ai_title}",
			video: &model.SavedVideo{
				Title:          "Original Video",
				GeneratedTitle: "AI Title",
			},
			want: "Original Video | AI Title",
		},
		{
			name:     "Only Original",
			template: "【搬运】{original_title}",
			video: &model.SavedVideo{
				Title: "My Video",
			},
			want: "【搬运】My Video",
		},
		{
			name:     "Missing AI Data",
			template: "{ai_title} - {original_title}",
			video: &model.SavedVideo{
				Title:          "Original",
				GeneratedTitle: "",
			},
			want: "- Original", // ai_title 被替换为空字符串
		},
		{
			name:     "Description Replace",
			template: "AI Summary:\n{ai_desc}\n\nRunning Time: {original_desc}",
			video: &model.SavedVideo{
				Description:   "10:00",
				GeneratedDesc: "This is a great video.",
			},
			want: "AI Summary:\nThis is a great video.\n\nRunning Time: 10:00",
		},
		{
			name:     "No Variables",
			template: "Static Title",
			video: &model.SavedVideo{
				Title: "Ignore Me",
			},
			want: "Static Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.applyTemplate(tt.template, tt.video)
			if got != tt.want {
				t.Errorf("applyTemplate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFillUploadMetadata 验证填充逻辑
func TestFillUploadMetadata(t *testing.T) {
	handler := &ConfirmMetadata{}

	t.Run("Template Mixture", func(t *testing.T) {
		video := &model.SavedVideo{
			Title:          "Original",
			GeneratedTitle: "AI",
			Description:    "Desc",
			GeneratedDesc:  "AI Desc",
		}

		handler.fillUploadMetadata(video, "template_mixture", "{ai_title} | {original_title}", "")

		if video.UploadTitle != "AI | Original" {
			t.Errorf("Expected UploadTitle 'AI | Original', got '%s'", video.UploadTitle)
		}
		// 描述没有模板，应该回退到优先AI
		if video.UploadDesc != "AI Desc" {
			t.Errorf("Expected UploadDesc 'AI Desc', got '%s'", video.UploadDesc)
		}
	})

	t.Run("Template Mixture with Empty AI", func(t *testing.T) {
		video := &model.SavedVideo{
			Title:          "Original",
			GeneratedTitle: "", // AI 失败
		}

		handler.fillUploadMetadata(video, "template_mixture", "{ai_title}{original_title}", "")

		if video.UploadTitle != "Original" {
			t.Errorf("Expected UploadTitle 'Original', got '%s'", video.UploadTitle)
		}
	})
}
