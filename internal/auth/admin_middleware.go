package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RequireAdmin 要求管理员权限的中间件
// 用法：router.GET("/admin/config", auth.RequireAdmin(), handler)
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取用户角色
		role, exists := c.Get(ContextKeyUserRole)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "未登录",
			})
			c.Abort()
			return
		}

		// 检查是否为管理员
		if role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "权限不足，需要管理员权限",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetCurrentUserRole 获取当前用户角色
func GetCurrentUserRole(c *gin.Context) string {
	role, exists := c.Get(ContextKeyUserRole)
	if !exists {
		return "user" // 默认为普通用户
	}
	if r, ok := role.(string); ok {
		return r
	}
	return "user"
}

// IsAdmin 判断当前用户是否是管理员
func IsAdmin(c *gin.Context) bool {
	return GetCurrentUserRole(c) == "admin"
}

// LoadUserRole 从数据库加载用户角色到 context
// 这个中间件应该在 JWTAuth 之后执行
func LoadUserRole(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := GetUserID(c)
		if !exists || userID == 0 {
			c.Next()
			return
		}

		// 查询用户角色
		type UserRole struct {
			Role string `gorm:"column:role"`
		}

		var userRole UserRole
		if err := db.Table("cw_users").Select("role").Where("id = ?", userID).First(&userRole).Error; err != nil {
			// 查询失败，默认为普通用户
			c.Set(ContextKeyUserRole, "user")
		} else {
			// 检查角色是否为空，如果为空则默认为 user
			if userRole.Role == "" {
				userRole.Role = "user"
			}
			c.Set(ContextKeyUserRole, userRole.Role)
		}

		c.Next()
	}
}
