package services

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/difyz9/ytb2bili/internal/core/types"
	"gorm.io/gorm"
)

const (
	LICENSE_VERSION = "v1"
	LICENSE_PREFIX  = "ytb-"
	BASE_DATE       = "2024-01-01" // 基准日期
)

// 产品代码 - 统一为 YB (YouTube to Bilibili)
const PRODUCT_CODE = "YB"

// getSecretKey 从环境变量获取密钥，支持回退
func getSecretKey() string {
	if val := os.Getenv("LICENSE_SECRET_KEY"); val != "" {
		return val
	}
	// 默认密钥（生产环境必须通过环境变量覆盖）
	return "ytb2bili-membership-license-secret-2024"
}

// LicenseService 许可证服务
type LicenseService struct {
	db *gorm.DB
}

// NewLicenseService 创建许可证服务
func NewLicenseService(db *gorm.DB) *LicenseService {
	return &LicenseService{db: db}
}

// GenerateLicense 生成离线许可证
// plan: "trial" (7天), "monthly", "quarterly", "yearly", "lifetime"
// tier: "basic", "pro", "enterprise"
// GenerateLicense 生成离线许可证
// plan: "trial" (7天), "monthly", "quarterly", "yearly", "lifetime"
// tier: "basic", "pro", "enterprise"
// permCode: 4位权限代码(如"0000", "0001")，为空则默认为"0000"
func (s *LicenseService) GenerateLicense(tier types.Tier, plan string, expiresAt *time.Time, permCode string) (string, error) {
	// 1. 套餐类型编码
	var planCode string
	switch plan {
	case "trial":
		planCode = "TR" // 7天体验
	case "monthly":
		planCode = "MO" // 月度
	case "quarterly":
		planCode = "QT" // 季度
	case "yearly":
		planCode = "YR" // 年度
	case "lifetime":
		planCode = "LT" // 永久
	default:
		return "", fmt.Errorf("invalid plan: %s (supported: trial, monthly, quarterly, yearly, lifetime)", plan)
	}

	// 2. 等级编码
	var tierCode string
	switch tier {
	case types.TierBasic:
		tierCode = "BA"
	case types.TierPro:
		tierCode = "PR"
	case types.TierEnterprise:
		tierCode = "EN"
	default:
		return "", fmt.Errorf("invalid tier: %s (supported: basic, pro, enterprise)", tier)
	}

	// 权限代码处理
	if permCode == "" {
		permCode = "0000"
	}
	if len(permCode) != 4 {
		// 尝试截断或补齐
		permCode = toBase36(fromBase36(permCode), 4)
	}
	permCode = strings.ToUpper(permCode)

	// 3. 计算过期天数（从基准日期开始）
	baseDate, _ := time.Parse("2006-01-02", BASE_DATE)
	var daysFromBase int64
	if expiresAt != nil && plan != "lifetime" {
		daysFromBase = int64(expiresAt.Sub(baseDate).Hours() / 24)
		if daysFromBase < 0 {
			daysFromBase = 0
		}
	} else {
		daysFromBase = 0 // 0表示永久
	}

	// Base36编码（6位）
	timeCode := toBase36(daysFromBase, 6)

	// 4. 生成3位随机数
	randomBytes := make([]byte, 2)
	rand.Read(randomBytes)
	randomNum := int64(randomBytes[0])<<8 | int64(randomBytes[1])
	randomCode := toBase36(randomNum%46656, 3) // 36^3 = 46656

	// 5. 组合数据部分：产品(2) + 等级(2) + 套餐(2) + 权限(4) + 时间(6) + 随机(3) = 19位
	payload := PRODUCT_CODE + tierCode + planCode + permCode + timeCode + randomCode

	// 6. 计算HMAC签名
	secretKey := getSecretKey()
	data := LICENSE_VERSION + payload
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(data))
	signature := mac.Sum(nil)

	// Base64URL编码签名（取前9字节，编码为12位）
	signatureCode := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(signature[:9])
	if len(signatureCode) > 12 {
		signatureCode = signatureCode[:12]
	}
	// 替换可能出现的 - 和 _ 字符，避免与分隔符冲突
	signatureCode = strings.ReplaceAll(signatureCode, "-", "X")
	signatureCode = strings.ReplaceAll(signatureCode, "_", "Z")

	// 7. 组装完整许可证: ytb-v1-YBBAMOPPPP TTTTTTRRR-SSSSSSSSSSSS
	licenseKey := LICENSE_PREFIX + LICENSE_VERSION + "-" + payload + "-" + signatureCode

	return licenseKey, nil
}

// LicenseInfo 许可证信息
type LicenseInfo struct {
	LicenseKey string     `json:"license_key"`
	Tier       types.Tier `json:"tier"`
	Plan       string     `json:"plan"`
	PermCode   string     `json:"perm_code"` // 新增权限码
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	IsExpired  bool       `json:"is_expired"`
	IsValid    bool       `json:"is_valid"`
}

// VerifyLicense 验证许可证并返回会员信息
func (s *LicenseService) VerifyLicense(key string) (*LicenseInfo, error) {
	originalKey := strings.TrimSpace(key)

	// 1. 检查前缀（不区分大小写）
	if !strings.HasPrefix(strings.ToUpper(originalKey), strings.ToUpper(LICENSE_PREFIX)) {
		return nil, fmt.Errorf("invalid license format: wrong prefix")
	}

	// 2. 移除前缀并分割
	keyWithoutPrefix := originalKey[len(LICENSE_PREFIX):]
	parts := strings.Split(keyWithoutPrefix, "-")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid license format: expected 3 parts")
	}

	version := strings.ToLower(parts[0]) // 转为小写比较
	payload := strings.ToUpper(parts[1]) // payload转为大写
	signatureCode := parts[2]            // 签名保持原样（大小写敏感）

	// 3. 验证版本
	if version != LICENSE_VERSION {
		return nil, fmt.Errorf("unsupported license version: %s", version)
	}

	// 4. 验证payload长度（2产品+2等级+2套餐+6时间+3随机=15位，或+4权限=19位）
	var hasPermCode bool
	if len(payload) == 15 {
		hasPermCode = false
	} else if len(payload) == 19 {
		hasPermCode = true
	} else {
		return nil, fmt.Errorf("invalid license format: wrong payload length (%d)", len(payload))
	}

	// 5. 提取产品代码
	productCode := payload[0:2]
	if productCode != PRODUCT_CODE {
		return nil, fmt.Errorf("invalid product code: %s", productCode)
	}

	// 6. 验证HMAC签名（使用原始小写版本和大写payload）
	secretKey := getSecretKey()
	data := LICENSE_VERSION + payload // 使用小写版本 + 大写payload
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(data))
	signature := mac.Sum(nil)
	expectedSig := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(signature[:9])
	if len(expectedSig) > 12 {
		expectedSig = expectedSig[:12]
	}
	// 替换可能出现的 - 和 _ 字符，保持与生成时一致
	expectedSig = strings.ReplaceAll(expectedSig, "-", "X")
	expectedSig = strings.ReplaceAll(expectedSig, "_", "Z")

	if signatureCode != expectedSig {
		return nil, fmt.Errorf("invalid license: signature verification failed")
	}

	// 7. 解析数据
	tierCode := payload[2:4]
	planCode := payload[4:6]

	var timeCode string
	var permCode string = "0000" // 默认权限码

	if hasPermCode {
		permCode = payload[6:10]
		timeCode = payload[10:16]
	} else {
		timeCode = payload[6:12]
	}

	// 8. 解析等级
	var tier types.Tier
	switch tierCode {
	case "BA":
		tier = types.TierBasic
	case "PR":
		tier = types.TierPro
	case "EN":
		tier = types.TierEnterprise
	default:
		return nil, fmt.Errorf("invalid tier code: %s", tierCode)
	}

	// 9. 解析套餐
	var plan string
	switch planCode {
	case "TR":
		plan = "trial"
	case "MO":
		plan = "monthly"
	case "QT":
		plan = "quarterly"
	case "YR":
		plan = "yearly"
	case "LT":
		plan = "lifetime"
	default:
		return nil, fmt.Errorf("invalid plan code: %s", planCode)
	}

	// 10. 解析过期时间
	var expiresAt *time.Time
	isExpired := false
	if plan != "lifetime" {
		daysFromBase := fromBase36(timeCode)
		if daysFromBase > 0 {
			baseDate, _ := time.Parse("2006-01-02", BASE_DATE)
			expireTime := baseDate.AddDate(0, 0, int(daysFromBase))
			expiresAt = &expireTime

			// 检查是否过期
			if time.Now().UTC().After(expireTime) {
				isExpired = true
			}
		}
	}

	return &LicenseInfo{
		LicenseKey: key,
		Tier:       tier,
		Plan:       plan,
		PermCode:   permCode,
		ExpiresAt:  expiresAt,
		IsExpired:  isExpired,
		IsValid:    !isExpired,
	}, nil
}

// ActivateLicense 激活许可证给指定用户
func (s *LicenseService) ActivateLicense(ctx context.Context, userID, licenseKey string) error {
	// 1. 验证许可证
	licenseInfo, err := s.VerifyLicense(licenseKey)
	if err != nil {
		return fmt.Errorf("license verification failed: %w", err)
	}

	if licenseInfo.IsExpired {
		return fmt.Errorf("license has expired")
	}

	// 开启事务
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 2. 检查许可证是否已被使用
		var existingActivation types.LicenseActivation
		if err := tx.Where("license_key = ?", licenseKey).First(&existingActivation).Error; err == nil {
			if existingActivation.UserID == userID {
				return fmt.Errorf("license already activated by you")
			}
			return fmt.Errorf("license already activated by another user")
		} else if err != gorm.ErrRecordNotFound {
			return err
		}

		// 3. 获取用户当前会员信息
		var userMembership types.UserMembership
		if err := tx.Where("user_id = ?", userID).First(&userMembership).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// 如果用户不存在会员记录，创建新的
				userMembership = types.UserMembership{
					UserID:    userID,
					Tier:      types.TierBasic,
					CreatedAt: time.Now(),
				}
			} else {
				return err
			}
		}

		// 4. 更新会员等级和过期时间
		userMembership.Tier = licenseInfo.Tier

		if licenseInfo.ExpiresAt != nil {
			// 如果当前有有效期且新许可证也有有效期，取较晚的时间
			if !userMembership.ExpiresAt.IsZero() && userMembership.ExpiresAt.After(*licenseInfo.ExpiresAt) {
				// 保持原有过期时间（更晚）
			} else {
				userMembership.ExpiresAt = *licenseInfo.ExpiresAt
			}
		} else {
			// 永久许可证：设置为 2099-12-31 以避免时区转换导致年份溢出
			// UTC 9999-12-31 转换到东八区会变成 10000 年，导致 MySQL 报错
			userMembership.ExpiresAt = time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)
		}

		userMembership.UpdatedAt = time.Now()

		// 5. 保存会员信息
		if err := tx.Save(&userMembership).Error; err != nil {
			return fmt.Errorf("failed to save membership: %w", err)
		}

		// 6. 保存许可证激活记录
		activation := types.LicenseActivation{
			LicenseKey:  licenseKey,
			UserID:      userID,
			Tier:        licenseInfo.Tier,
			Plan:        licenseInfo.Plan,
			ExpiresAt:   licenseInfo.ExpiresAt,
			ActivatedAt: time.Now(),
		}

		if err := tx.Create(&activation).Error; err != nil {
			return fmt.Errorf("failed to save license activation: %w", err)
		}

		return nil
	})
}

// GetUserLicenses 获取用户的所有许可证
func (s *LicenseService) GetUserLicenses(ctx context.Context, userID string) ([]types.LicenseActivation, error) {
	var activations []types.LicenseActivation
	if err := s.db.Where("user_id = ?", userID).Find(&activations).Error; err != nil {
		return nil, err
	}
	return activations, nil
}

// toBase36 将数字转为Base36字符串（固定长度）
func toBase36(num int64, length int) string {
	if num < 0 {
		num = 0
	}
	s := strconv.FormatInt(num, 36)
	s = strings.ToUpper(s)

	// 补齐或截断到指定长度
	if len(s) < length {
		s = strings.Repeat("0", length-len(s)) + s
	} else if len(s) > length {
		s = s[len(s)-length:]
	}
	return s
}

// fromBase36 从Base36字符串转为数字
func fromBase36(s string) int64 {
	num, _ := strconv.ParseInt(strings.ToLower(s), 36, 64)
	return num
}

// IsLicenseValid 检查激活记录是否有效
func IsLicenseValid(activation *types.LicenseActivation) bool {
	if activation == nil {
		return false
	}
	// 如果 ExpiresAt 是 nil 或零值，表示永久有效
	if activation.ExpiresAt == nil || activation.ExpiresAt.IsZero() {
		return true
	}
	return time.Now().Before(*activation.ExpiresAt)
}
