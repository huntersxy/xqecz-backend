
package middleware

import (
	"net/http"
	"xiaoquan-backend/models"
	"xiaoquan-backend/utils"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := c.Cookie("session_id")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "未登录",
				"data":    nil,
			})
			c.Abort()
			return
		}

		userID, exists := utils.SessionStore[sessionID]
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "会话已过期",
				"data":    nil,
			})
			c.Abort()
			return
		}

		var user models.User
		if err := utils.DB.First(&user, userID).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "用户不存在",
				"data":    nil,
			})
			c.Abort()
			return
		}

		c.Set("user", user)
		c.Next()
	}
}

func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "未登录",
				"data":    nil,
			})
			c.Abort()
			return
		}

		u, ok := user.(models.User)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "用户信息错误",
				"data":    nil,
			})
			c.Abort()
			return
		}

		if !u.IsAdmin {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "需要管理员权限",
				"data":    nil,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func BannedMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "未登录",
				"data":    nil,
			})
			c.Abort()
			return
		}

		u, ok := user.(models.User)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "用户信息错误",
				"data":    nil,
			})
			c.Abort()
			return
		}

		if u.IsBanned {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "账号已被封禁",
				"data":    nil,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
