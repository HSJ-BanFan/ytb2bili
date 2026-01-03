package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/difyz9/ytb2bili/internal/auth"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/pkg/store/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// ============================================================================
// Smoke Test 1: 认证流程
// 验证：JWT 中间件和路由保护有效
// ============================================================================

func TestSmokeTest_AuthFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup
	app := SetupTestApp(t)
	defer app.Cleanup()

	jwtConfig := auth.JWTConfig{
		SecretKey:     "test_secret_key_for_smoke_test",
		Issuer:        "smoke-test",
		AccessExpiry:  24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	}
	jwtService := auth.NewJWTService(jwtConfig)
	authMiddleware := auth.NewAuthMiddleware(app.DB, jwtService)

	// 创建测试路由
	router := gin.New()

	// 公开路由
	router.POST("/api/v1/auth/login", func(c *gin.Context) {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "参数错误"})
			return
		}

		// 查询用户
		var user model.User
		if err := app.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
			c.JSON(401, gin.H{"error": "用户不存在"})
			return
		}

		// 验证密码（bcrypt 哈希比对）
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
			c.JSON(401, gin.H{"error": "密码错误"})
			return
		}

		// 生成 token
		token, _ := jwtService.GenerateAccessToken(user.ID, user.Username, "free", "test")
		c.JSON(200, gin.H{"token": token})
	})

	// 受保护路由
	protected := router.Group("/api/v1")
	protected.Use(authMiddleware.JWTAuth())
	protected.GET("/videos", func(c *gin.Context) {
		userID, _ := auth.GetUserID(c)
		c.JSON(200, gin.H{"user_id": userID, "videos": []interface{}{}})
	})

	// ========================================
	// 测试 1: 无 token 访问受保护路由 → 401
	// ========================================
	t.Run("无token访问受保护路由应返回401", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/videos", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	// ========================================
	// 测试 2: 登录获取 token
	// ========================================
	var accessToken string
	t.Run("登录应返回token", func(t *testing.T) {
		loginData := map[string]string{
			"email":    "test@example.com",
			"password": "123456",
		}
		body, _ := json.Marshal(loginData)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]string
		json.Unmarshal(w.Body.Bytes(), &resp)
		accessToken = resp["token"]
		assert.NotEmpty(t, accessToken)
	})

	// ========================================
	// 测试 2.5: 错误密码 → 401
	// ========================================
	t.Run("错误密码应返回401", func(t *testing.T) {
		loginData := map[string]string{
			"email":    "test@example.com",
			"password": "wrong_password",
		}
		body, _ := json.Marshal(loginData)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		var resp map[string]string
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "密码错误", resp["error"])
	})

	// ========================================
	// 测试 3: 带 token 访问受保护路由 → 200
	// ========================================
	t.Run("带token访问受保护路由应返回200", func(t *testing.T) {
		require.NotEmpty(t, accessToken, "需要先获取 token")

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/videos", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NotNil(t, resp["user_id"])
	})

	// ========================================
	// 测试 4: 无效 token → 401
	// ========================================
	t.Run("无效token应返回401", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/videos", nil)
		req.Header.Set("Authorization", "Bearer invalid_token_12345")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// ============================================================================
// Smoke Test 2: 用户隔离
// 验证：User A 不能访问 User B 的资源
// ============================================================================

func TestSmokeTest_UserIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup
	app := SetupTestApp(t)
	defer app.Cleanup()

	// 创建两个用户
	userA := &model.User{
		Username:      "user_a",
		Email:         fmt.Sprintf("user_a_%d@example.com", time.Now().UnixNano()),
		Password:      "hashed",
		Role:          "user",
		Status:        1,
		EmailVerified: true,
	}
	app.DB.Create(userA)

	userB := &model.User{
		Username:      "user_b",
		Email:         fmt.Sprintf("user_b_%d@example.com", time.Now().UnixNano()),
		Password:      "hashed",
		Role:          "user",
		Status:        1,
		EmailVerified: true,
	}
	app.DB.Create(userB)

	// User A 创建视频
	videoA := &model.SavedVideo{
		UserID:  userA.ID,
		VideoID: fmt.Sprintf("video_a_%d", time.Now().UnixNano()),
		Title:   "User A 的视频",
		Status:  "001",
		URL:     "https://youtube.com/watch?v=test",
	}
	app.DB.Create(videoA)

	// 初始化服务
	videoService := services.NewSavedVideoService(app.DB)

	// ========================================
	// 测试 1: User A 可以访问自己的视频
	// ========================================
	t.Run("用户可以访问自己的视频", func(t *testing.T) {
		video, err := videoService.GetVideoByIDForUser(videoA.ID, userA.ID)

		assert.NoError(t, err)
		assert.NotNil(t, video)
		assert.Equal(t, "User A 的视频", video.Title)
	})

	// ========================================
	// 测试 2: User B 不能访问 User A 的视频
	// ========================================
	t.Run("用户不能访问其他用户的视频", func(t *testing.T) {
		video, err := videoService.GetVideoByIDForUser(videoA.ID, userB.ID)

		assert.Error(t, err)
		assert.Nil(t, video)
	})

	// ========================================
	// 测试 3: User B 删除 User A 的视频应失败
	// ========================================
	t.Run("用户不能删除其他用户的视频", func(t *testing.T) {
		err := videoService.DeleteVideoForUser(videoA.ID, userB.ID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "视频不存在或无权删除")

		// 验证视频仍然存在
		video, _ := videoService.GetVideoByIDForUser(videoA.ID, userA.ID)
		assert.NotNil(t, video)
	})

	// ========================================
	// 测试 4: User A 列表只显示自己的视频
	// ========================================
	t.Run("视频列表只显示用户自己的视频", func(t *testing.T) {
		// User B 创建自己的视频
		videoB := &model.SavedVideo{
			UserID:  userB.ID,
			VideoID: fmt.Sprintf("video_b_%d", time.Now().UnixNano()),
			Title:   "User B 的视频",
			Status:  "001",
			URL:     "https://youtube.com/watch?v=test2",
		}
		app.DB.Create(videoB)

		// User A 只能看到自己的 1 个视频
		videosA, totalA, err := videoService.GetVideosPaginatedForUser(0, 10, userA.ID)
		assert.NoError(t, err)
		assert.Equal(t, 1, totalA)
		assert.Len(t, videosA, 1)

		// User B 只能看到自己的 1 个视频
		videosB, totalB, err := videoService.GetVideosPaginatedForUser(0, 10, userB.ID)
		assert.NoError(t, err)
		assert.Equal(t, 1, totalB)
		assert.Len(t, videosB, 1)
	})
}

// ============================================================================
// Smoke Test 3: 任务链状态机
// 验证：任务步骤状态流转正确
// ============================================================================

func TestSmokeTest_TaskChainStateFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup
	app := SetupTestApp(t)
	defer app.Cleanup()

	// 创建用户和视频
	user := &model.User{
		Username:      "task_user",
		Email:         fmt.Sprintf("task_%d@example.com", time.Now().UnixNano()),
		Password:      "hashed",
		Role:          "user",
		Status:        1,
		EmailVerified: true,
	}
	app.DB.Create(user)

	videoID := fmt.Sprintf("task_video_%d", time.Now().UnixNano())
	video := &model.SavedVideo{
		UserID:  user.ID,
		VideoID: videoID,
		Title:   "任务测试视频",
		Status:  "001",
		URL:     "https://youtube.com/watch?v=test",
	}
	app.DB.Create(video)

	// 初始化服务
	taskStepService := services.NewTaskStepService(app.DB)

	// ========================================
	// 测试 1: 初始化任务步骤
	// ========================================
	t.Run("初始化任务步骤应创建9个步骤", func(t *testing.T) {
		err := taskStepService.InitTaskSteps(videoID)
		assert.NoError(t, err)

		steps, err := taskStepService.GetTaskStepsByVideoID(videoID)
		assert.NoError(t, err)
		assert.Len(t, steps, 9)

		// 验证初始状态都是 waiting
		for _, step := range steps {
			assert.Equal(t, model.TaskStepStatusWaiting, step.Status)
		}
	})

	// ========================================
	// 测试 2: 状态流转 pending → running → completed
	// ========================================
	t.Run("状态应正确流转", func(t *testing.T) {
		stepName := "下载视频"

		// pending → running
		err := taskStepService.UpdateTaskStepStatus(videoID, stepName, model.TaskStepStatusRunning)
		assert.NoError(t, err)

		step, _ := taskStepService.GetTaskStepByName(videoID, stepName)
		assert.Equal(t, model.TaskStepStatusRunning, step.Status)
		assert.NotNil(t, step.StartTime)

		// running → completed
		err = taskStepService.UpdateTaskStepStatus(videoID, stepName, model.TaskStepStatusCompleted)
		assert.NoError(t, err)

		step, _ = taskStepService.GetTaskStepByName(videoID, stepName)
		assert.Equal(t, model.TaskStepStatusCompleted, step.Status)
		assert.NotNil(t, step.EndTime)
	})

	// ========================================
	// 测试 3: 获取任务进度
	// ========================================
	t.Run("任务进度应正确计算", func(t *testing.T) {
		// 完成一个步骤后检查进度
		progress, err := taskStepService.GetTaskProgress(videoID)

		assert.NoError(t, err)
		assert.Equal(t, 9, progress["total_steps"])
		assert.Equal(t, 1, progress["completed_steps"])
		assert.True(t, progress["progress_percent"].(int) > 0)
	})

	// ========================================
	// 测试 4: 失败步骤可以重置
	// ========================================
	t.Run("失败步骤应可以重置", func(t *testing.T) {
		stepName := "下载字幕"

		// 设置为失败
		taskStepService.UpdateTaskStepStatus(videoID, stepName, model.TaskStepStatusFailed, "模拟下载失败")

		step, _ := taskStepService.GetTaskStepByName(videoID, stepName)
		assert.Equal(t, model.TaskStepStatusFailed, step.Status)
		assert.Equal(t, "模拟下载失败", step.ErrorMsg)

		// 重置
		err := taskStepService.ResetTaskStep(videoID, stepName)
		assert.NoError(t, err)

		step, _ = taskStepService.GetTaskStepByName(videoID, stepName)
		assert.Equal(t, model.TaskStepStatusPending, step.Status)
		assert.Empty(t, step.ErrorMsg)
	})

	// ========================================
	// 测试 5: 用户隔离 - 其他用户不能访问任务步骤
	// ========================================
	t.Run("其他用户不能访问任务步骤", func(t *testing.T) {
		otherUser := &model.User{
			Username:      "other_user",
			Email:         fmt.Sprintf("other_%d@example.com", time.Now().UnixNano()),
			Password:      "hashed",
			Role:          "user",
			Status:        1,
			EmailVerified: true,
		}
		app.DB.Create(otherUser)

		// 其他用户尝试访问
		steps, err := taskStepService.GetTaskStepsByVideoIDForUser(videoID, otherUser.ID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "视频不存在或无权访问")
		assert.Nil(t, steps)
	})
}
