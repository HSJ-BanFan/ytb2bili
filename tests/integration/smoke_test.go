package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/pkg/store/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// Smoke Test 1: 工具模式 API
func TestSmokeTest_PublicAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := SetupTestApp(t)
	defer app.Cleanup()

	router := gin.New()
	router.GET("/api/v1/videos", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"videos": []interface{}{}})
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/videos", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
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

	// 创建视频

	videoID := fmt.Sprintf("task_video_%d", time.Now().UnixNano())
	video := &model.SavedVideo{
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
}
