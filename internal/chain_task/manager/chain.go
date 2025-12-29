package manager

import (
	"fmt"
	"strings"
	"time"

	"github.com/difyz9/ytb2bili/internal/core/types"
	"go.uber.org/zap"
)

// TaskConfig 任务配置
type TaskConfig struct {
	Name         string   // 任务名称
	Dependencies []string // 依赖的任务
	Required     bool     // 是否必需（失败是否终止链）
}

// 预定义任务配置
// ┌────┬──────────────┬─────────────────────────────┬──────┬────────────┐
// │序号│    任务名    │           依赖              │ 必需 │  失败处理  │
// ├────┼──────────────┼─────────────────────────────┼──────┼────────────┤
// │ 1  │ 获取元数据   │           无                │  是  │  终止链    │
// │ 2  │ 下载视频     │           无                │  是  │  终止链    │
// │ 3  │ 下载字幕     │           无                │  否  │  继续执行  │
// │ 4  │ 下载封面     │       获取元数据            │  是  │  终止链    │
// │ 5  │ 翻译字幕     │        下载字幕             │  否  │    跳过    │
// │ 6  │ AI增强元数据 │ 下载视频,翻译字幕           │  否  │    跳过    │
// │ 7  │ 确认元数据   │       获取元数据            │  是  │  终止链    │
// │    │  (根据用户设置决定使用原始/AI数据)          │      │            │
// │ 8  │ 上传到B站    │       确认元数据            │  否  │    -       │
// │ 9  │ 上传字幕     │       上传到B站             │  否  │  延迟重试  │
// └────┴──────────────┴─────────────────────────────┴──────┴────────────┘
var TaskConfigs = map[string]TaskConfig{
	"获取元数据":   {Name: "获取元数据", Dependencies: nil, Required: true},
	"下载视频":    {Name: "下载视频", Dependencies: nil, Required: true},
	"下载字幕":    {Name: "下载字幕", Dependencies: nil, Required: false},
	"下载封面":    {Name: "下载封面", Dependencies: []string{"获取元数据"}, Required: true},
	"翻译字幕":    {Name: "翻译字幕", Dependencies: []string{"下载字幕"}, Required: false},
	"AI增强元数据": {Name: "AI增强元数据", Dependencies: []string{"下载视频", "翻译字幕"}, Required: false},
	// 确认元数据：只依赖获取元数据（确保有基础数据），不依赖 AI增强元数据
	// 根据用户配置决定使用原始数据还是 AI 生成的数据
	"确认元数据":         {Name: "确认元数据", Dependencies: []string{"获取元数据"}, Required: true},
	"上传到Bilibili":   {Name: "上传到Bilibili", Dependencies: []string{"确认元数据"}, Required: false},
	"上传字幕到Bilibili": {Name: "上传字幕到Bilibili", Dependencies: []string{"上传到Bilibili"}, Required: false},
	// 兼容旧名称
	"生成元数据": {Name: "生成元数据", Dependencies: []string{"下载视频", "翻译字幕"}, Required: false},
}

// TaskChain 任务链
type TaskChain struct {
	Tasks            []types.Task
	Context          map[string]interface{}
	Logger           *zap.SugaredLogger
	VideoID          string
	CompletedTasks   map[string]bool // 记录已完成的任务
	FailedTasks      map[string]bool // 记录失败的任务
	SkippedTasks     map[string]bool // 记录硬跳过的任务（依赖未满足）
	SoftSkippedTasks map[string]bool // 记录软跳过的任务（执行成功但无需处理）
}

// NewTaskChain 创建任务链
func NewTaskChain() *TaskChain {
	return &TaskChain{
		Tasks:            make([]types.Task, 0),
		Context:          make(map[string]interface{}),
		CompletedTasks:   make(map[string]bool),
		FailedTasks:      make(map[string]bool),
		SkippedTasks:     make(map[string]bool),
		SoftSkippedTasks: make(map[string]bool),
	}
}

// SetLogger 设置日志记录器
func (c *TaskChain) SetLogger(logger *zap.SugaredLogger) *TaskChain {
	c.Logger = logger
	return c
}

// SetVideoID 设置视频ID
func (c *TaskChain) SetVideoID(videoID string) *TaskChain {
	c.VideoID = videoID
	return c
}

// SetCompletedTasks 设置已完成的任务（用于单任务执行时预填充依赖状态）
func (c *TaskChain) SetCompletedTasks(completedTasks []string) *TaskChain {
	for _, taskName := range completedTasks {
		c.CompletedTasks[taskName] = true
	}
	return c
}

// AddTask 添加任务到链中
func (c *TaskChain) AddTask(task types.Task) *TaskChain {
	if err := task.InsertTask(); err != nil {
		c.log("WARN", "添加任务到数据库失败: %v", err)
	}
	c.Tasks = append(c.Tasks, task)
	return c
}

// log 统一日志输出
func (c *TaskChain) log(level string, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if c.Logger != nil {
		switch level {
		case "INFO":
			c.Logger.Info(msg)
		case "WARN":
			c.Logger.Warn(msg)
		case "ERROR":
			c.Logger.Error(msg)
		case "DEBUG":
			c.Logger.Debug(msg)
		}
	}
}

// printChainHeader 打印任务链开始信息
func (c *TaskChain) printChainHeader() {
	c.log("INFO", "")
	c.log("INFO", "╔══════════════════════════════════════════════════════════════╗")
	c.log("INFO", "║                    📋 任务链开始执行                          ║")
	c.log("INFO", "╠══════════════════════════════════════════════════════════════╣")
	c.log("INFO", "║  视频ID: %-52s║", c.VideoID)
	c.log("INFO", "║  任务数: %-52d║", len(c.Tasks))
	c.log("INFO", "╚══════════════════════════════════════════════════════════════╝")
}

// printTaskStart 打印任务开始信息
func (c *TaskChain) printTaskStart(index int, taskName string, config TaskConfig) {
	c.log("INFO", "")
	c.log("INFO", "┌─────────────────────────────────────────────────────────────┐")
	c.log("INFO", "│ [%d/%d] 🚀 开始执行: %s", index+1, len(c.Tasks), taskName)
	if len(config.Dependencies) > 0 {
		c.log("INFO", "│       📌 依赖: %s", strings.Join(config.Dependencies, ", "))
	}
	if config.Required {
		c.log("INFO", "│       ⚠️  必需任务 (失败将终止链)")
	}
	c.log("INFO", "└─────────────────────────────────────────────────────────────┘")
}

// printTaskResult 打印任务结果
func (c *TaskChain) printTaskResult(taskName string, success bool, skipped bool, duration time.Duration, errMsg string) {
	if skipped {
		c.log("INFO", "│ ⏭️  [%s] 已跳过 (依赖未满足)", taskName)
	} else if success {
		c.log("INFO", "│ ✅ [%s] 执行成功 (耗时: %v)", taskName, duration.Round(time.Millisecond))
	} else {
		c.log("ERROR", "│ ❌ [%s] 执行失败: %s", taskName, errMsg)
	}
}

// printChainSummary 打印任务链执行摘要
func (c *TaskChain) printChainSummary(totalDuration time.Duration, terminated bool) {
	c.log("INFO", "")
	c.log("INFO", "╔══════════════════════════════════════════════════════════════╗")
	c.log("INFO", "║                    📊 任务链执行摘要                          ║")
	c.log("INFO", "╠══════════════════════════════════════════════════════════════╣")
	// 跳过数 = 硬跳过 + 软跳过
	skippedCount := len(c.SkippedTasks) + len(c.SoftSkippedTasks)
	c.log("INFO", "║  ✅ 成功: %-3d  ❌ 失败: %-3d  ⏭️  跳过: %-3d                    ║",
		len(c.CompletedTasks), len(c.FailedTasks), skippedCount)
	c.log("INFO", "║  ⏱️  总耗时: %-48v║", totalDuration.Round(time.Millisecond))
	if terminated {
		c.log("INFO", "║  ⚠️  状态: 链已终止 (必需任务失败)                            ║")
	} else {
		c.log("INFO", "║  ✅ 状态: 链执行完成                                         ║")
	}
	c.log("INFO", "╚══════════════════════════════════════════════════════════════╝")
}

// checkDependencies 检查任务依赖是否满足
// 返回：(是否满足, 原因)
// 注意：区分 "软跳过"（任务执行成功但返回 skipped 标记）和 "硬跳过"（依赖未满足）
//
//	软跳过的任务被视为完成，不会阻止后续任务
func (c *TaskChain) checkDependencies(taskName string) (bool, string) {
	config, exists := TaskConfigs[taskName]
	if !exists || len(config.Dependencies) == 0 {
		return true, ""
	}

	for _, dep := range config.Dependencies {
		// 硬失败：依赖任务执行失败（返回 false）
		if c.FailedTasks[dep] {
			return false, fmt.Sprintf("依赖任务 [%s] 执行失败", dep)
		}
		// 完成或软跳过都视为满足依赖
		// 软跳过：任务执行成功（返回 true）但设置了 skipped 标记
		// 例如：翻译字幕禁用时返回 true + skipped，后续 AI增强元数据 应继续执行
		if c.CompletedTasks[dep] || c.SoftSkippedTasks[dep] {
			continue
		}
		// 硬跳过：由于依赖未满足导致的跳过，后续任务也应跳过
		if c.SkippedTasks[dep] {
			return false, fmt.Sprintf("依赖任务 [%s] 被跳过", dep)
		}
		// 未执行
		return false, fmt.Sprintf("依赖任务 [%s] 未执行", dep)
	}
	return true, ""
}

// Run 执行任务链
func (c *TaskChain) Run(stopOnRequiredFailure bool) map[string]interface{} {
	chainStartTime := time.Now()
	terminated := false

	c.printChainHeader()

	for i, task := range c.Tasks {
		taskName := task.GetName()
		config := TaskConfigs[taskName]
		if config.Name == "" {
			config = TaskConfig{Name: taskName, Required: false}
		}

		c.printTaskStart(i, taskName, config)

		// 检查依赖
		depsOK, depReason := c.checkDependencies(taskName)
		if !depsOK {
			c.SkippedTasks[taskName] = true
			c.Context["skipped"] = depReason
			// 硬跳过：更新数据库中的步骤状态
			task.UpdateStatus("skipped", "依赖未满足: "+depReason)
			c.printTaskResult(taskName, false, true, 0, depReason)
			continue
		}

		// 执行任务
		taskStartTime := time.Now()
		success := false
		var panicMsg string

		func() {
			defer func() {
				if r := recover(); r != nil {
					panicMsg = fmt.Sprintf("任务执行异常: %v", r)
					c.log("ERROR", "任务 [%s] 发生 panic: %v", taskName, r)
				}
			}()
			success = task.Execute(c.Context)
		}()

		taskDuration := time.Since(taskStartTime)

		// 处理结果
		if panicMsg != "" {
			c.FailedTasks[taskName] = true
			c.Context["error"] = panicMsg
			c.printTaskResult(taskName, false, false, taskDuration, panicMsg)
		} else if success {
			// 检查是否被标记为跳过（软跳过：任务执行成功但无需处理）
			if _, skipped := c.Context["skipped"]; skipped {
				// 软跳过：任务返回 true 但设置了 skipped 标记
				// 视为"成功"，不会阻止依赖此任务的后续任务
				c.SoftSkippedTasks[taskName] = true
				c.printTaskResult(taskName, false, true, taskDuration, "")
				delete(c.Context, "skipped")
			} else {
				c.CompletedTasks[taskName] = true
				c.printTaskResult(taskName, true, false, taskDuration, "")
			}
		} else {
			c.FailedTasks[taskName] = true
			errMsg := ""
			if err, exists := c.Context["error"]; exists {
				errMsg = fmt.Sprintf("%v", err)
			}
			c.printTaskResult(taskName, false, false, taskDuration, errMsg)

			// 如果是必需任务失败，终止链
			if config.Required && stopOnRequiredFailure {
				c.log("ERROR", "⛔ 必需任务 [%s] 失败，终止任务链", taskName)
				terminated = true
				break
			}
		}
	}

	c.printChainSummary(time.Since(chainStartTime), terminated)

	return c.Context
}

// RunWithContext 使用指定的初始 context 执行任务链
func (c *TaskChain) RunWithContext(initialContext map[string]interface{}) map[string]interface{} {
	// 合并初始 context 到任务链的 context
	for k, v := range initialContext {
		c.Context[k] = v
	}
	return c.Run(false)
}
