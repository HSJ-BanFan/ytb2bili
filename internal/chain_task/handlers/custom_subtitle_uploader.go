package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/difyz9/bilibili-go-sdk/bilibili"
)

// CustomSubtitleUploader 自定义字幕上传器，支持 lan_doc 参数和自动重试
type CustomSubtitleUploader struct {
	client    *http.Client
	loginInfo *bilibili.LoginInfo
	userAgent string
}

// SubtitleFileWithLanDoc 带语言描述的字幕文件结构
type SubtitleFileWithLanDoc struct {
	URL        string `json:"url"`
	Language   string `json:"lan"`
	LanDoc     string `json:"lan_doc"`
	SubtitleID int    `json:"subtitle_id"`
}

// SubtitleVideoInfo 字幕相关的视频信息结构
type SubtitleVideoInfo struct {
	CID int64 `json:"cid"`
	AID int64 `json:"aid"`
}

// 语言代码到语言名称的映射
var languageDocMap = map[string]string{
	"zh-Hans": "中文（简体）",
	"zh-Hant": "中文（繁体）",
	"zh":      "中文（简体）",
	"zh-CN":   "中文（简体）",
	"zh-TW":   "中文（繁体）",
	"cn":      "中文（简体）",
	"en":      "英语",
	"en-US":   "英语",
	"ja":      "日语",
	"ko":      "韩语",
}

// NewCustomSubtitleUploader 创建自定义字幕上传器
func NewCustomSubtitleUploader(loginInfo *bilibili.LoginInfo) *CustomSubtitleUploader {
	return &CustomSubtitleUploader{
		client:    &http.Client{Timeout: 60 * time.Second},
		loginInfo: loginInfo,
		userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
	}
}

// GetVideoInfo 获取视频信息（CID和AID）
func (s *CustomSubtitleUploader) GetVideoInfo(bvid string) (*SubtitleVideoInfo, error) {
	url := fmt.Sprintf("https://member.bilibili.com/x/vupre/web/archive/view?bvid=%s", bvid)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	// 添加Cookie
	cookieStr := s.loginInfo.GetCookieString()
	req.Header.Set("Cookie", cookieStr)
	req.Header.Set("User-Agent", s.userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Videos []struct {
				CID int64 `json:"cid"`
				AID int64 `json:"aid"`
			} `json:"videos"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	if response.Code != 0 {
		return nil, fmt.Errorf("get video info failed: code=%d, message=%s", response.Code, response.Message)
	}

	if len(response.Data.Videos) == 0 {
		return nil, fmt.Errorf("video info is empty")
	}

	return &SubtitleVideoInfo{
		CID: response.Data.Videos[0].CID,
		AID: response.Data.Videos[0].AID,
	}, nil
}

// UploadSubtitleFile 上传字幕文件到Bilibili存储 (.srt)
func (s *CustomSubtitleUploader) UploadSubtitleFile(subtitlePath string) (string, string, error) {
	// 获取CSRF Token
	csrf, err := s.loginInfo.GetCSRFToken()
	if err != nil {
		return "", "", fmt.Errorf("get CSRF token failed: %w", err)
	}

	// 创建multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// 添加字段
	writer.WriteField("bucket", "subtitle")
	writer.WriteField("csrf", csrf)
	writer.WriteField("content_type", "application/x-subrip")

	// 添加文件
	file, err := os.Open(subtitlePath)
	if err != nil {
		return "", "", fmt.Errorf("open subtitle file failed: %w", err)
	}
	defer file.Close()

	fileWriter, err := writer.CreateFormFile("file", "subtitle.srt")
	if err != nil {
		return "", "", fmt.Errorf("create form file failed: %w", err)
	}

	_, err = io.Copy(fileWriter, file)
	if err != nil {
		return "", "", fmt.Errorf("copy file content failed: %w", err)
	}

	writer.Close()

	// 构建请求
	timestamp := time.Now().UnixMilli()
	url := fmt.Sprintf("https://api.bilibili.com/x/upload/web/image?t=%d&csrf=%s", timestamp, csrf)

	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return "", "", fmt.Errorf("create upload request failed: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Cookie", s.loginInfo.GetCookieString())
	req.Header.Set("User-Agent", s.userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read upload response failed: %w", err)
	}

	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Location string `json:"location"`
			Etag     string `json:"etag"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", "", fmt.Errorf("unmarshal upload response failed: %w", err)
	}

	if response.Code != 0 {
		return "", "", fmt.Errorf("upload subtitle file failed: code=%d, message=%s", response.Code, response.Message)
	}

	return response.Data.Location, response.Data.Etag, nil
}

// SaveSubtitleInfo 保存字幕信息到视频（使用 preSave API）
func (s *CustomSubtitleUploader) SaveSubtitleInfo(aid, cid int64, location, language, lanDoc string) error {
	// 获取CSRF Token
	csrf, err := s.loginInfo.GetCSRFToken()
	if err != nil {
		return fmt.Errorf("get CSRF token failed: %w", err)
	}

	// 构建字幕文件信息（包含 lan_doc）
	subtitleFiles := []SubtitleFileWithLanDoc{
		{
			URL:        location,
			Language:   language,
			LanDoc:     lanDoc,
			SubtitleID: 0,
		},
	}

	filesJSON, err := json.Marshal(subtitleFiles)
	if err != nil {
		return fmt.Errorf("marshal subtitle files failed: %w", err)
	}

	// 创建multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	writer.WriteField("oid", strconv.FormatInt(cid, 10))
	writer.WriteField("type", "1")
	writer.WriteField("files", string(filesJSON))
	writer.WriteField("aid", strconv.FormatInt(aid, 10))
	writer.WriteField("csrf", csrf)
	// 尝试添加 submit 和 sign 参数
	writer.WriteField("submit", "true")
	writer.WriteField("sign", "false")

	writer.Close()

	// 构建请求
	timestamp := time.Now().UnixMilli()
	url := fmt.Sprintf("https://api.bilibili.com/x/v2/dm/subtitle/draft/preSave?t=%d&csrf=%s", timestamp, csrf)

	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return fmt.Errorf("create save request failed: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Cookie", s.loginInfo.GetCookieString())
	req.Header.Set("User-Agent", s.userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("save request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read save response failed: %w", err)
	}

	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("unmarshal save response failed: %w", err)
	}

	if response.Code != 0 {
		// 79001: 当前语言已上传生效的字幕文件 (视为成功)
		if response.Code == 79001 {
			return nil
		}
		return fmt.Errorf("code=%d, message=%s", response.Code, response.Message)
	}

	return nil
}

// UploadSubtitleWithLanDoc 完整的字幕上传流程（带多语言代码重试）
func (s *CustomSubtitleUploader) UploadSubtitleWithLanDoc(bvid, subtitlePath, language string) error {
	// 1. 获取视频信息
	videoInfo, err := s.GetVideoInfo(bvid)
	if err != nil {
		// 尝试另一种方式获取(兼容)
		return fmt.Errorf("get video info failed: %w", err)
	}

	// 2. 上传字幕文件 (.srt)
	location, _, err := s.UploadSubtitleFile(subtitlePath)
	if err != nil {
		return fmt.Errorf("upload subtitle file failed: %w", err)
	}

	// 3. 准备要尝试的语言代码列表
	// 优先使用已知可用的 'zh'
	langsToTry := []string{}

	if language == "zh-Hans" || language == "zh-CN" || language == "cn" {
		langsToTry = append(langsToTry, "zh", "zh-CN", "zh-Hans", "cn")
	} else if strings.Contains(strings.ToLower(language), "zh") {
		langsToTry = append(langsToTry, "zh", language, "zh-CN")
	} else {
		langsToTry = append(langsToTry, language)
	}

	// 4. 循环尝试保存字幕信息
	var lastErr error
	for _, lang := range langsToTry {
		// 获取语言描述
		lanDoc, ok := languageDocMap[lang]
		if !ok {
			lanDoc = lang
			// 对于 zh-CN 等变体，尽量给个正确的 LanDoc
			if lang == "zh-CN" || lang == "zh" || lang == "cn" {
				lanDoc = "中文（简体）"
			}
		}

		err = s.SaveSubtitleInfo(videoInfo.AID, videoInfo.CID, location, lang, lanDoc)
		if err == nil {
			return nil
		}

		lastErr = err
		// 如果错误不是 79011 (不合法的语言)，则可能不是语言代码问题，直接返回
		if !strings.Contains(err.Error(), "79011") && !strings.Contains(err.Error(), "不合法的语言") {
			return err
		}
	}

	return fmt.Errorf("save subtitle info failed after retries: %w", lastErr)
}
