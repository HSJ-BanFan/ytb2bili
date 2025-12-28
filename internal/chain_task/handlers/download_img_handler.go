package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/difyz9/ytb2bili/internal/chain_task/base"
	"github.com/difyz9/ytb2bili/internal/chain_task/manager"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/models"
	"github.com/difyz9/ytb2bili/pkg/cos"
	"github.com/difyz9/ytb2bili/pkg/utils"
	"gorm.io/gorm"
)

type DownloadImgHandler struct {
	base.BaseTask
	App *core.AppServer
	DB  *gorm.DB
}

func NewDownloadImgHandler(name string, app *core.AppServer, stateManager *manager.StateManager, client *cos.CosClient) *DownloadImgHandler {
	return &DownloadImgHandler{
		BaseTask: base.BaseTask{
			Name:         name,
			StateManager: stateManager,
			Client:       client,
		},
		App: app,
	}

}

func (t *DownloadImgHandler) Execute(context map[string]interface{}) bool {
	t.App.Logger.Info("========================================")
	t.App.Logger.Info("📷 开始下载视频封面")
	t.App.Logger.Info("========================================")

	t.App.Logger.Infof("📂 保存目录: %s", t.StateManager.CurrentDir)

	// 1. 尝试从元数据中获取封面 URL（支持所有 yt-dlp 平台）
	var thumbnailURL string
	if metadata, ok := context["metadata"].(*VideoMetadataInfo); ok && metadata != nil && metadata.Thumbnail != "" {
		thumbnailURL = metadata.Thumbnail
		t.App.Logger.Infof("📥 从元数据获取封面 URL: %s", thumbnailURL)
	}

	// 2. 如果有 thumbnail URL，直接下载
	if thumbnailURL != "" {
		coverPath, err := t.downloadFromURL(thumbnailURL)
		if err == nil {
			context["cover_image_path"] = coverPath
			t.App.Logger.Infof("✅ 封面下载成功: %s", coverPath)

			// 尝试上传到 COS（可选）
			t.uploadToCOS(coverPath)

			t.App.Logger.Info("========================================")
			t.App.Logger.Info("✅ 封面下载完成")
			t.App.Logger.Info("========================================")
			return true
		}
		t.App.Logger.Warnf("⚠️ 从元数据 URL 下载失败: %v，尝试 YouTube 格式", err)
	}

	// 3. 回退：尝试 YouTube 格式（兼容旧数据）
	t.App.Logger.Info("📥 尝试 YouTube 封面格式...")
	opt := utils.DownloadOptions{
		SavePath:         t.StateManager.CurrentDir,
		FilenameTemplate: "{quality}",
		Timeout:          10 * time.Second,
		MaxRetries:       3,
		QualityFallback:  true,
		CreateDirs:       true,
		Overwrite:        false,
	}

	qualities := []utils.ImageQuality{utils.QualityMax, utils.QualityStandard}
	t.App.Logger.Infof("📥 下载封面质量: %v", qualities)

	resultsInterface := utils.DownloadYouTubeThumbnail(t.StateManager.VideoID, qualities, opt, "")
	results, ok := resultsInterface.(map[string]utils.DownloadResult)
	if !ok {
		t.App.Logger.Error("❌ 下载封面返回结果类型错误")
		context["error"] = "下载封面返回结果类型错误"
		return false
	}

	var maxQualityCoverPath string
	var anySuccess bool

	for k, v := range results {
		if v.Success {
			anySuccess = true
			t.App.Logger.Infof("✓ 下载成功: %s - %s (%d bytes)", k, v.FilePath, v.FileSize)

			// 尝试上传到 COS（可选，失败不影响任务）
			t.uploadToCOS(v.FilePath)

			// 如果是最高质量的封面，保存到context中供后续上传使用
			if k == string(utils.QualityMax) {
				maxQualityCoverPath = v.FilePath
				context["cover_image_path"] = v.FilePath
				t.App.Logger.Infof("✓ 最高质量封面已设置: %s", v.FilePath)
			}
		} else {
			t.App.Logger.Warnf("⚠️ 下载失败: %s - %s", k, v.ErrorMessage)
		}
	}

	// 如果没有下载到最高质量的封面，使用其他质量的封面
	if maxQualityCoverPath == "" {
		for _, v := range results {
			if v.Success {
				context["cover_image_path"] = v.FilePath
				t.App.Logger.Infof("✓ 备用质量封面已设置: %s", v.FilePath)
				break
			}
		}
	}

	if !anySuccess {
		t.App.Logger.Error("❌ 所有封面下载都失败了")
		context["error"] = "所有封面下载都失败了"
		return false
	}

	t.App.Logger.Info("========================================")
	t.App.Logger.Info("✅ 封面下载完成")
	t.App.Logger.Info("========================================")

	return true
}

// downloadFromURL 从指定 URL 下载封面
func (t *DownloadImgHandler) downloadFromURL(imageURL string) (string, error) {

	// 确保目录存在
	if err := os.MkdirAll(t.StateManager.CurrentDir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	// 下载图片
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// 确定文件扩展名
	ext := ".jpg"
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "png") {
		ext = ".png"
	} else if strings.Contains(contentType, "webp") {
		ext = ".webp"
	}

	// 保存文件
	filePath := filepath.Join(t.StateManager.CurrentDir, "cover"+ext)
	f, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	return filePath, nil
}

// uploadToCOS 上传到 COS（可选）
func (t *DownloadImgHandler) uploadToCOS(filePath string) {
	if t.Client != nil {
		cosKeyName, err := t.Client.UploadImageToCOS(filePath, "")
		if err != nil {
			t.App.Logger.Warnf("⚠️ 上传封面到 COS 失败: %v", err)
		} else if cosKeyName != "" {
			// 更新数据库记录
			tbVideo := &models.TbVideo{
				Id:      t.StateManager.Id,
				VideoId: t.StateManager.VideoID,
				ImgURL:  cosKeyName,
				Status:  "img",
			}
			if err := t.StateManager.UpdateTBVideo(tbVideo); err != nil {
				t.App.Logger.Warnf("⚠️ 更新数据库记录失败: %v", err)
			}
		}
	}
}
