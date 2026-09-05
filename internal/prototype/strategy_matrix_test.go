package prototype_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/difyz9/ytb2bili/pkg/store/model"
)

// setupTestDB 初始化内存 SQLite 数据库并运行 AutoMigrate
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&model.SourceChannel{},
		&model.UserBiliAccount{},
		&model.StrategyRule{},
		&model.PublishFingerprint{},
		&model.SystemGuardrail{},
	)
	require.NoError(t, err)
	return db
}

// ============================================================================
// 1. 多对多策略矩阵映射测试 (Multi-to-Multi Strategy Matrix)
// ============================================================================

func TestStrategyMatrix_MultiToMultiMapping(t *testing.T) {
	db := setupTestDB(t)

	// 1. 创建两个来源频道 (SourceChannel)
	techChannel := model.SourceChannel{
		Platform:    "youtube",
		ChannelID:   "UC_Tech_001",
		ChannelName: "科技前沿速递",
		ChannelURL:  "https://youtube.com/channel/UC_Tech_001",
		FetchType:   "channel_video",
		IsEnabled:   true,
		Status:      "active",
	}
	gamingChannel := model.SourceChannel{
		Platform:    "youtube",
		ChannelID:   "UC_Gaming_002",
		ChannelName: "高能游戏实况",
		ChannelURL:  "https://youtube.com/channel/UC_Gaming_002",
		FetchType:   "channel_video",
		IsEnabled:   true,
		Status:      "active",
	}
	require.NoError(t, db.Create(&techChannel).Error)
	require.NoError(t, db.Create(&gamingChannel).Error)

	// 2. 创建两个 B 站投稿账号 (UserBiliAccount)
	primaryAccount := model.UserBiliAccount{
		BiliMid:   10001,
		BiliName:  "极客搬运总舵",
		IsEnabled: true,
		IsPrimary: true,
	}
	secondaryAccount := model.UserBiliAccount{
		BiliMid:   10002,
		BiliName:  "二次元综合副号",
		IsEnabled: true,
		IsPrimary: false,
	}
	require.NoError(t, db.Create(&primaryAccount).Error)
	require.NoError(t, db.Create(&secondaryAccount).Error)

	// 3. 建立多对多策略矩阵规则 (StrategyRule):
	//    - 科技频道 -> 主账号 (全自动投稿, 优先级 10)
	//    - 科技频道 -> 副号 (人工审核备份, 优先级 5)
	//    - 游戏频道 -> 副号 (全自动投稿, 优先级 8)
	ruleTechToPrimary := model.StrategyRule{
		SourceChannelID:      techChannel.ID,
		BiliAccountID:        primaryAccount.ID,
		RuleName:             "科技主号精翻全自动",
		IsEnabled:            true,
		Priority:             10,
		AutoPublish:          true,
		DynamicTitleTemplate: "【科技前沿】{title}",
		DescTemplate:         "原作者: 科技前沿速递\n本视频由 AI 协同翻译搬运",
		DefaultTags:          "科技,前沿,AI,数码",
		CategoryID:           188, // 科技区
		Copyright:            2,   // 转载
	}
	ruleTechToSecondary := model.StrategyRule{
		SourceChannelID:      techChannel.ID,
		BiliAccountID:        secondaryAccount.ID,
		RuleName:             "科技副号备份待审",
		IsEnabled:            true,
		Priority:             5,
		AutoPublish:          false, // 需要人工审核
		DynamicTitleTemplate: "【备份】{title}",
		DescTemplate:         "仅作备份存档",
		DefaultTags:          "科技,备份",
		CategoryID:           188,
		Copyright:            2,
	}
	ruleGamingToSecondary := model.StrategyRule{
		SourceChannelID:      gamingChannel.ID,
		BiliAccountID:        secondaryAccount.ID,
		RuleName:             "游戏副号切片速发",
		IsEnabled:            true,
		Priority:             8,
		AutoPublish:          true,
		DynamicTitleTemplate: "【精彩切片】{title}",
		DescTemplate:         "高能游戏精彩集锦",
		DefaultTags:          "游戏,实况,精彩片段",
		CategoryID:           17, // 游戏区
		Copyright:            2,
	}
	require.NoError(t, db.Create(&ruleTechToPrimary).Error)
	require.NoError(t, db.Create(&ruleTechToSecondary).Error)
	require.NoError(t, db.Create(&ruleGamingToSecondary).Error)

	// --- 验证场景 A: 给定来源频道，获取其所有分发规则与目标投稿账号 ---
	var techRules []model.StrategyRule
	err := db.Preload("UserBiliAccount").
		Where("source_channel_id = ? AND is_enabled = ?", techChannel.ID, true).
		Order("priority DESC").
		Find(&techRules).Error
	require.NoError(t, err)
	assert.Len(t, techRules, 2, "科技频道应当匹配到 2 条分发规则")
	assert.Equal(t, "科技主号精翻全自动", techRules[0].RuleName, "按优先级排序第一项应当是主号全自动规则")
	assert.True(t, techRules[0].AutoPublish)
	assert.Equal(t, primaryAccount.BiliName, techRules[0].UserBiliAccount.BiliName)

	assert.Equal(t, "科技副号备份待审", techRules[1].RuleName)
	assert.False(t, techRules[1].AutoPublish)
	assert.Equal(t, secondaryAccount.BiliName, techRules[1].UserBiliAccount.BiliName)

	// --- 验证场景 B: 给定投稿账号，反向查询有哪些来源频道向其输送视频 ---
	var secondaryRules []model.StrategyRule
	err = db.Preload("SourceChannel").
		Where("bili_account_id = ? AND is_enabled = ?", secondaryAccount.ID, true).
		Find(&secondaryRules).Error
	require.NoError(t, err)
	assert.Len(t, secondaryRules, 2, "副号应当接收来自 2 个不同频道的投稿规则")

	channelNames := []string{secondaryRules[0].SourceChannel.ChannelName, secondaryRules[1].SourceChannel.ChannelName}
	assert.Contains(t, channelNames, "科技前沿速递")
	assert.Contains(t, channelNames, "高能游戏实况")
}

// ============================================================================
// 2. 投稿防线：指纹幂等性与并发排队锁测试 (Idempotency & Concurrent Locks)
// ============================================================================

func TestPublishFingerprint_IdempotencyAndLocks(t *testing.T) {
	db := setupTestDB(t)

	platform := "youtube"
	sourceVideoID := "dQw4w9WgXcQ"
	targetAccountID := uint(101)
	strategyRuleID := uint(1)

	// 计算确定性指纹哈希
	fingerprintHash := model.GenerateFingerprintHash(platform, sourceVideoID, targetAccountID)
	assert.NotEmpty(t, fingerprintHash)

	// 1. 首次入库：初始待处理状态 (Pending)
	initialRecord := model.PublishFingerprint{
		SourcePlatform:  platform,
		SourceVideoID:   sourceVideoID,
		BiliAccountID:   targetAccountID,
		StrategyRuleID:  strategyRuleID,
		FingerprintHash: fingerprintHash,
		PublishStatus:   model.FingerprintStatusPending,
		RetryCount:      0,
		MaxRetries:      3,
	}
	err := db.Create(&initialRecord).Error
	require.NoError(t, err, "首次创建投稿指纹应当成功")

	// 2. 幂等性校验：重复尝试创建完全相同 (Platform:VideoID:AccountID) 的记录必须被数据库唯一索引拦截
	duplicateRecord := model.PublishFingerprint{
		SourcePlatform:  platform,
		SourceVideoID:   sourceVideoID,
		BiliAccountID:   targetAccountID,
		StrategyRuleID:  strategyRuleID,
		FingerprintHash: fingerprintHash,
		PublishStatus:   model.FingerprintStatusPending,
	}
	err = db.Create(&duplicateRecord).Error
	assert.Error(t, err, "唯一索引 fingerprint_hash 必须拦截重复记录插入")

	// 3. Worker 并发获取执行锁 (Atomic Claim)
	// 规则：只有在 status == 'pending' 或者 (status == 'locked' 且 lock_expires_at < now) 时才能加锁
	lockDuration := 5 * time.Minute
	lockExpire := time.Now().Add(lockDuration)

	// Worker 1 尝试获取锁
	result := db.Model(&model.PublishFingerprint{}).
		Where("fingerprint_hash = ? AND (publish_status = ? OR (publish_status = ? AND lock_expires_at < ?))",
			fingerprintHash, model.FingerprintStatusPending, model.FingerprintStatusLocked, time.Now()).
		Updates(map[string]interface{}{
			"publish_status":  model.FingerprintStatusLocked,
			"lock_expires_at": lockExpire,
		})
	require.NoError(t, result.Error)
	assert.Equal(t, int64(1), result.RowsAffected, "Worker 1 应当成功抢占到执行锁")

	// Worker 2 并发尝试获取锁（此时已被锁且未过期）
	resultConflict := db.Model(&model.PublishFingerprint{}).
		Where("fingerprint_hash = ? AND (publish_status = ? OR (publish_status = ? AND lock_expires_at < ?))",
			fingerprintHash, model.FingerprintStatusPending, model.FingerprintStatusLocked, time.Now()).
		Updates(map[string]interface{}{
			"publish_status":  model.FingerprintStatusLocked,
			"lock_expires_at": time.Now().Add(lockDuration),
		})
	require.NoError(t, resultConflict.Error)
	assert.Equal(t, int64(0), resultConflict.RowsAffected, "Worker 2 应当无法获取处于锁定中的任务")

	// 4. 模拟 Worker 异常崩溃，锁超时恢复测试 (Crash Recovery)
	pastExpire := time.Now().Add(-10 * time.Second) // 模拟锁已于 10 秒前过期
	db.Model(&model.PublishFingerprint{}).
		Where("fingerprint_hash = ?", fingerprintHash).
		Update("lock_expires_at", pastExpire)

	// Worker 3 检测到锁已过期，重新抢占
	newExpire := time.Now().Add(lockDuration)
	resultRecover := db.Model(&model.PublishFingerprint{}).
		Where("fingerprint_hash = ? AND (publish_status = ? OR (publish_status = ? AND lock_expires_at < ?))",
			fingerprintHash, model.FingerprintStatusPending, model.FingerprintStatusLocked, time.Now()).
		Updates(map[string]interface{}{
			"publish_status":  model.FingerprintStatusLocked,
			"lock_expires_at": newExpire,
		})
	require.NoError(t, resultRecover.Error)
	assert.Equal(t, int64(1), resultRecover.RowsAffected, "Worker 3 应当成功接管锁已过期的任务")

	// 5. 投稿完成，写入终态并保存 BVID，彻底关闭重入通道
	publishedTime := time.Now()
	bvid := "BV1xx411c7Xz"
	aid := int64(170001)
	err = db.Model(&model.PublishFingerprint{}).
		Where("fingerprint_hash = ?", fingerprintHash).
		Updates(map[string]interface{}{
			"publish_status": model.FingerprintStatusPublished,
			"bili_bvid":      bvid,
			"bili_aid":       aid,
			"published_at":   publishedTime,
		}).Error
	require.NoError(t, err)

	// 后续调度器再次扫描该视频
	var finalRecord model.PublishFingerprint
	err = db.Where("fingerprint_hash = ?", fingerprintHash).First(&finalRecord).Error
	require.NoError(t, err)
	assert.Equal(t, model.FingerprintStatusPublished, finalRecord.PublishStatus)
	assert.Equal(t, bvid, finalRecord.BiliBVID)

	// 验证终态拦截：不允许重复投稿
	canReupload := finalRecord.PublishStatus != model.FingerprintStatusPublished
	assert.False(t, canReupload, "已发布成功的记录绝对不可再次触发投稿")
}

// ============================================================================
// 3. 投稿防线：三级熔断暂停开关测试 (3-Tier Pause Switch: Global/Channel/Account)
// ============================================================================

// GuardrailDecision 熔断评估结果
type GuardrailDecision struct {
	Allowed bool
	Tier    string // global, channel, account, none
	Reason  string
}

// EvaluatePublishGuardrails 统一评估三级熔断防线
func EvaluatePublishGuardrails(db *gorm.DB, channelID uint, accountID uint) GuardrailDecision {
	now := time.Now()

	// 1. 第一级：全局开关 (Global Kill Switch)
	var globalRail model.SystemGuardrail
	if err := db.Where("scope = ? AND target_id = ?", model.GuardrailScopeGlobal, "0").First(&globalRail).Error; err == nil {
		if globalRail.IsPaused {
			// 检查是否已过自动恢复冷却时间
			if globalRail.AutoResumeAt != nil && now.After(*globalRail.AutoResumeAt) {
				// 已过冷却期，自动恢复
				db.Model(&globalRail).Updates(map[string]interface{}{
					"is_paused":            false,
					"consecutive_failures": 0,
				})
			} else {
				return GuardrailDecision{
					Allowed: false,
					Tier:    model.GuardrailScopeGlobal,
					Reason:  fmt.Sprintf("全局暂停触发: %s", globalRail.PauseReason),
				}
			}
		}
	}

	// 2. 第二级：来源频道开关 (Channel Level)
	var channel model.SourceChannel
	if err := db.First(&channel, channelID).Error; err == nil {
		if !channel.IsEnabled || channel.Status == "paused" {
			return GuardrailDecision{
				Allowed: false,
				Tier:    model.GuardrailScopeChannel,
				Reason:  fmt.Sprintf("来源频道已禁用或处于暂停态 (ID: %d)", channelID),
			}
		}
	}
	var channelRail model.SystemGuardrail
	if err := db.Where("scope = ? AND target_id = ?", model.GuardrailScopeChannel, fmt.Sprint(channelID)).First(&channelRail).Error; err == nil {
		if channelRail.IsPaused {
			if channelRail.AutoResumeAt != nil && now.After(*channelRail.AutoResumeAt) {
				db.Model(&channelRail).Updates(map[string]interface{}{
					"is_paused":            false,
					"consecutive_failures": 0,
				})
			} else {
				return GuardrailDecision{
					Allowed: false,
					Tier:    model.GuardrailScopeChannel,
					Reason:  fmt.Sprintf("频道熔断暂停: %s", channelRail.PauseReason),
				}
			}
		}
	}

	// 3. 第三级：目标投稿账号开关 (Account Level)
	var account model.UserBiliAccount
	if err := db.First(&account, accountID).Error; err == nil {
		if !account.IsEnabled {
			return GuardrailDecision{
				Allowed: false,
				Tier:    model.GuardrailScopeAccount,
				Reason:  fmt.Sprintf("B站投稿账号已停用 (MID: %d)", account.BiliMid),
			}
		}
	}
	var accountRail model.SystemGuardrail
	if err := db.Where("scope = ? AND target_id = ?", model.GuardrailScopeAccount, fmt.Sprint(accountID)).First(&accountRail).Error; err == nil {
		if accountRail.IsPaused {
			if accountRail.AutoResumeAt != nil && now.After(*accountRail.AutoResumeAt) {
				db.Model(&accountRail).Updates(map[string]interface{}{
					"is_paused":            false,
					"consecutive_failures": 0,
				})
			} else {
				return GuardrailDecision{
					Allowed: false,
					Tier:    model.GuardrailScopeAccount,
					Reason:  fmt.Sprintf("B站账号熔断暂停 (如HTTP 601限流): %s", accountRail.PauseReason),
				}
			}
		}
	}

	return GuardrailDecision{
		Allowed: true,
		Tier:    "none",
		Reason:  "防线检测通过，允许执行投稿",
	}
}

// RecordFailureAndTripCircuitBreaker 记录失败并在超过阈值时触发自动熔断
func RecordFailureAndTripCircuitBreaker(db *gorm.DB, scope, targetID, reason string, cooldown time.Duration) error {
	var rail model.SystemGuardrail
	err := db.Where("scope = ? AND target_id = ?", scope, targetID).First(&rail).Error
	now := time.Now()

	if err != nil {
		// 记录不存在则初始化
		rail = model.SystemGuardrail{
			Scope:               scope,
			TargetID:            targetID,
			IsPaused:            false,
			ConsecutiveFailures: 1,
			FailureThreshold:    3,
			LastTriggeredAt:     &now,
		}
		return db.Create(&rail).Error
	}

	rail.ConsecutiveFailures++
	rail.LastTriggeredAt = &now

	// 达到阈值，触发熔断
	if rail.ConsecutiveFailures >= rail.FailureThreshold {
		rail.IsPaused = true
		rail.PauseReason = reason
		autoResume := now.Add(cooldown)
		rail.AutoResumeAt = &autoResume
	}

	return db.Save(&rail).Error
}

func TestSystemGuardrail_ThreeTierPauseAndCircuitBreaker(t *testing.T) {
	db := setupTestDB(t)

	// 初始化测试基底
	ch1 := model.SourceChannel{Platform: "youtube", ChannelID: "ch1", ChannelName: "频道1", IsEnabled: true, Status: "active"}
	ch2 := model.SourceChannel{Platform: "youtube", ChannelID: "ch2", ChannelName: "频道2", IsEnabled: true, Status: "active"}
	acc1 := model.UserBiliAccount{BiliMid: 201, BiliName: "账号1", IsEnabled: true}
	acc2 := model.UserBiliAccount{BiliMid: 202, BiliName: "账号2", IsEnabled: true}
	require.NoError(t, db.Create(&ch1).Error)
	require.NoError(t, db.Create(&ch2).Error)
	require.NoError(t, db.Create(&acc1).Error)
	require.NoError(t, db.Create(&acc2).Error)

	// --- 阶段 1: 正常状态 ---
	dec := EvaluatePublishGuardrails(db, ch1.ID, acc1.ID)
	assert.True(t, dec.Allowed, "初始状态应全部放行")

	// --- 阶段 2: 账号级熔断测试 (Tier 3) ---
	// 模拟 acc2 连续 3 次触发 B 站 HTTP 601 限流
	for i := 1; i <= 3; i++ {
		err := RecordFailureAndTripCircuitBreaker(db, model.GuardrailScopeAccount, fmt.Sprint(acc2.ID), "HTTP 601 限流频控", 30*time.Minute)
		require.NoError(t, err)
	}

	// 验证：ch1 投往 acc2 应该被账号级拦截
	decAcc2Blocked := EvaluatePublishGuardrails(db, ch1.ID, acc2.ID)
	assert.False(t, decAcc2Blocked.Allowed)
	assert.Equal(t, model.GuardrailScopeAccount, decAcc2Blocked.Tier)
	assert.Contains(t, decAcc2Blocked.Reason, "HTTP 601 限流频控")

	// 验证隔离性：ch1 投往 acc1 仍然不受影响
	decAcc1Pass := EvaluatePublishGuardrails(db, ch1.ID, acc1.ID)
	assert.True(t, decAcc1Pass.Allowed, "账号2熔断不应当波及账号1")

	// --- 阶段 3: 频道级暂停测试 (Tier 2) ---
	// 将 ch2 手动禁用或标记为熔断
	require.NoError(t, db.Model(&ch2).Update("is_enabled", false).Error)

	decCh2Blocked := EvaluatePublishGuardrails(db, ch2.ID, acc1.ID)
	assert.False(t, decCh2Blocked.Allowed)
	assert.Equal(t, model.GuardrailScopeChannel, decCh2Blocked.Tier)

	// ch1 投往 acc1 仍然正常
	decCh1StillOk := EvaluatePublishGuardrails(db, ch1.ID, acc1.ID)
	assert.True(t, decCh1StillOk.Allowed)

	// --- 阶段 4: 全局紧急熔断测试 (Tier 1 Global Kill Switch) ---
	now := time.Now()
	globalRail := model.SystemGuardrail{
		Scope:           model.GuardrailScopeGlobal,
		TargetID:        "0",
		IsPaused:        true,
		PauseReason:     "人工紧急熔断拉闸",
		LastTriggeredAt: &now,
	}
	require.NoError(t, db.Create(&globalRail).Error)

	// 验证：原本正常的 ch1 -> acc1 现在被全局级拦截
	decGlobalBlocked := EvaluatePublishGuardrails(db, ch1.ID, acc1.ID)
	assert.False(t, decGlobalBlocked.Allowed)
	assert.Equal(t, model.GuardrailScopeGlobal, decGlobalBlocked.Tier)
	assert.Contains(t, decGlobalBlocked.Reason, "人工紧急熔断拉闸")

	// --- 阶段 5: 全局开关恢复后，分级恢复测试 ---
	require.NoError(t, db.Model(&globalRail).Update("is_paused", false).Error)

	// ch1 -> acc1 恢复允许
	assert.True(t, EvaluatePublishGuardrails(db, ch1.ID, acc1.ID).Allowed)
	// 但已禁用的 ch2 依然被阻断
	assert.False(t, EvaluatePublishGuardrails(db, ch2.ID, acc1.ID).Allowed)
	// 已被熔断的 acc2 依然被阻断
	assert.False(t, EvaluatePublishGuardrails(db, ch1.ID, acc2.ID).Allowed)

	// --- 阶段 6: 冷却时间超时自动恢复测试 ---
	// 将 acc2 的 AutoResumeAt 调整到过去
	pastTime := time.Now().Add(-5 * time.Minute)
	require.NoError(t, db.Model(&model.SystemGuardrail{}).
		Where("scope = ? AND target_id = ?", model.GuardrailScopeAccount, fmt.Sprint(acc2.ID)).
		Update("auto_resume_at", pastTime).Error)

	// 再次评估 ch1 -> acc2，应当检测到已过冷却期并自动恢复放行
	decAcc2AutoResumed := EvaluatePublishGuardrails(db, ch1.ID, acc2.ID)
	assert.True(t, decAcc2AutoResumed.Allowed, "冷却期过后应当自动放行")
}
