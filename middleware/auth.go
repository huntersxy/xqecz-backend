package middleware

import (
	"net/http"
	"xiaoquan-backend/models"
	"xiaoquan-backend/utils"

	"github.com/gin-gonic/gin"
)

func abort(c *gin.Context, status int, msg string) {
	utils.RespondWithError(c, status, msg)
	c.Abort()
}

func getUser(c *gin.Context) (models.User, bool) {
	user, exists := c.Get("user")
	if !exists {
		abort(c, http.StatusUnauthorized, "未登录")
		return models.User{}, false
	}
	u, ok := user.(models.User)
	if !ok {
		abort(c, http.StatusInternalServerError, "用户信息错误")
		return models.User{}, false
	}
	return u, true
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := c.Cookie("session_id")
		if err != nil {
			abort(c, http.StatusUnauthorized, "未登录")
			return
		}

		userID, err := utils.GetSession(sessionID)
		if err != nil {
			abort(c, http.StatusUnauthorized, "会话已过期")
			return
		}

		var user models.User
		if cached, err := utils.GetUserInfoCache(userID); err == nil {
			user = *cached
		} else {
			if err := utils.DB.First(&user, userID).Error; err != nil {
				abort(c, http.StatusUnauthorized, "用户不存在")
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
		u, ok := getUser(c)
		if !ok {
			return
		}
		if !u.IsAdmin {
			abort(c, http.StatusForbidden, "需要管理员权限")
			return
		}
		c.Next()
	}
}

func BannedMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := getUser(c)
		if !ok {
			return
		}
		if u.IsBanned {
			abort(c, http.StatusForbidden, "账号已被封禁")
			return
		}
		c.Next()
	}
}
