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
			utils.RespondWithError(c, http.StatusUnauthorized, "未登录")
			c.Abort()
			return
		}

		userID, err := utils.GetSession(sessionID)
		if err != nil {
			utils.RespondWithError(c, http.StatusUnauthorized, "会话已过期")
			c.Abort()
			return
		}

		var user models.User
		if cached, err := utils.GetUserInfoCache(userID); err == nil {
			user = *cached
		} else {
			if err := utils.DB.First(&user, userID).Error; err != nil {
				utils.RespondWithError(c, http.StatusUnauthorized, "用户不存在")
				c.Abort()
				return
			}
			utils.SetUserInfoCache(&user)
		}

		c.Set("user", user)
		c.Next()
	}
}

func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			utils.RespondWithError(c, http.StatusUnauthorized, "未登录")
			c.Abort()
			return
		}

		u, ok := user.(models.User)
		if !ok {
			utils.RespondWithError(c, http.StatusInternalServerError, "用户信息错误")
			c.Abort()
			return
		}

		if !u.IsAdmin {
			utils.RespondWithError(c, http.StatusForbidden, "需要管理员权限")
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
			utils.RespondWithError(c, http.StatusUnauthorized, "未登录")
			c.Abort()
			return
		}

		u, ok := user.(models.User)
		if !ok {
			utils.RespondWithError(c, http.StatusInternalServerError, "用户信息错误")
			c.Abort()
			return
		}

		if u.IsBanned {
			utils.RespondWithError(c, http.StatusForbidden, "账号已被封禁")
			c.Abort()
			return
		}

		c.Next()
	}
}
