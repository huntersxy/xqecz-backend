
package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
	"xiaoquan-backend/models"
	"xiaoquan-backend/utils"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const (
	SessionIDLength = 32
	CookieMaxAge    = 3600 * 24 * 7
	SessionDuration = 24 * time.Hour
)

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Register 处理用户注册请求
// 验证用户名和密码格式，检查用户名唯一性，使用bcrypt加密密码后创建用户
func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondWithError(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}

	if !utils.ValidateUsername(req.Username) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "用户名无效（3-50个字符）",
			"data":    nil,
		})
		return
	}

	if !utils.ValidatePassword(req.Password) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "密码至少6位",
			"data":    nil,
		})
		return
	}

	var existingUser models.User
	if utils.DB.Where("username = ?", req.Username).First(&existingUser).Error == nil {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": "用户名已存在",
			"data":    nil,
		})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "密码加密失败",
			"data":    nil,
		})
		return
	}

	user := &models.User{
		Username: req.Username,
		Password: string(hashedPassword),
		IsAdmin:  false,
	}

	if err := utils.DB.Create(user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建用户失败",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "注册成功",
		"data": gin.H{
			"user_id": user.ID,
		},
	})
}

// Login 处理用户登录请求
// 验证用户名密码，生成安全会话ID，优先使用Redis存储会话，降级到内存存储
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondWithError(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}

	var user models.User
	if err := utils.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "用户名或密码错误",
			"data":    nil,
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "用户名或密码错误",
			"data":    nil,
		})
		return
	}

	sessionID := generateSessionID()
	if utils.RedisClient != nil {
		utils.SetSession(sessionID, user.ID)
	} else {
		utils.SessionStore[sessionID] = user.ID
	}

	c.SetCookie("session_id", sessionID, CookieMaxAge, "/", "", false, false)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "登录成功",
		"data": gin.H{
			"user": gin.H{
				"id":       user.ID,
				"username": user.Username,
				"is_admin": user.IsAdmin,
			},
		},
	})
}

// Logout 处理用户登出请求
// 从会话存储中删除会话，使Cookie失效
func Logout(c *gin.Context) {
	sessionID, err := c.Cookie("session_id")
	if err == nil {
		if utils.RedisClient != nil {
			utils.DeleteSession(sessionID)
		} else {
			delete(utils.SessionStore, sessionID)
		}
	}
	c.SetCookie("session_id", "", -1, "/", "", false, false)
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "登出成功",
		"data":    nil,
	})
}

// GetMe 获取当前登录用户信息
// 从请求上下文获取已通过AuthMiddleware验证的用户信息
func GetMe(c *gin.Context) {
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
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data": gin.H{
			"id":       u.ID,
			"username": u.Username,
			"is_admin": u.IsAdmin,
		},
	})
}

// shouldUseSecureCookie 判断是否应该使用Secure Cookie
// localhost开发环境不使用Secure，以便在HTTP下正常工作
func shouldUseSecureCookie(c *gin.Context) bool {
	host := c.Request.Host
	if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") {
		return false
	}
	return true
}

// generateSessionID 生成加密安全的会话ID
// 使用crypto/rand生成32字节随机数据，转换为64字符hex字符串
func generateSessionID() string {
	b := make([]byte, SessionIDLength)
	_, err := rand.Read(b)
	if err != nil {
		return hex.EncodeToString(b)
	}
	return hex.EncodeToString(b)
}
