
package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"xiaoquan-backend/models"
	"xiaoquan-backend/utils"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const (
	SessionIDLength = 32
	CookieMaxAge    = 3600 * 24 * 7
)

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
			"data":    nil,
		})
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

func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
			"data":    nil,
		})
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
	utils.SessionStore[sessionID] = user.ID

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

func Logout(c *gin.Context) {
	sessionID, err := c.Cookie("session_id")
	if err == nil {
		delete(utils.SessionStore, sessionID)
	}
	c.SetCookie("session_id", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "登出成功",
		"data":    nil,
	})
}

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

func generateSessionID() string {
	b := make([]byte, SessionIDLength)
	_, err := rand.Read(b)
	if err != nil {
		return hex.EncodeToString(b)
	}
	return hex.EncodeToString(b)
}
