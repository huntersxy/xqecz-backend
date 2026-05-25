package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"time"
	"xiaoquan-backend/models"
	"xiaoquan-backend/utils"
)

const (
	SessionIDLength = 32
	CookieMaxAge    = 3600 * 24 * 30
	SessionDuration = 30 * 24 * time.Hour
)

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Register 用户注册
// @Summary      用户注册
// @Description  注册新用户账号
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body body RegisterRequest true "注册信息"
// @Success      200 {object} utils.SwaggerResponse{data=object{user_id=uint}}
// @Failure      400 {object} utils.SwaggerResponse
// @Failure      409 {object} utils.SwaggerResponse
// @Router       /auth/register [post]
// Register 处理用户注册请求
// 验证用户名和密码格式，检查用户名唯一性，使用bcrypt加密密码后创建用户
func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondWithError(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}

	if !utils.ValidateUsername(req.Username) {
		utils.RespondWithError(c, http.StatusBadRequest, "用户名无效（3-50个字符）")
		return
	}

	if !utils.ValidatePassword(req.Password) {
		utils.RespondWithError(c, http.StatusBadRequest, "密码至少6位")
		return
	}

	var existingUser models.User
	if utils.DB.Where("username = ?", req.Username).First(&existingUser).Error == nil {
		utils.RespondWithError(c, http.StatusConflict, "用户名已存在")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "密码加密失败")
		return
	}

	user := &models.User{
		Username: req.Username,
		Password: string(hashedPassword),
		IsAdmin:  false,
	}

	if err := utils.DB.Create(user).Error; err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "创建用户失败")
		return
	}

	utils.RespondWithSuccess(c, gin.H{
		"user_id": user.ID,
	})
}

// Login 用户登录
// @Summary      用户登录
// @Description  使用用户名密码登录，成功返回session cookie
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        body body LoginRequest true "登录信息"
// @Success      200 {object} utils.SwaggerResponse{data=object{user=object{id=uint,username=string,is_admin=bool}}}
// @Failure      400 {object} utils.SwaggerResponse
// @Failure      401 {object} utils.SwaggerResponse
// @Router       /auth/login [post]
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
		utils.RespondWithError(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		utils.RespondWithError(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	sessionID := generateSessionID()
	if err := utils.SetSession(sessionID, user.ID); err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "创建会话失败")
		return
	}

	c.Writer.Header().Add("Set-Cookie", fmt.Sprintf("session_id=%s; Path=/; Max-Age=%d; HttpOnly; SameSite=None; Secure", sessionID, CookieMaxAge))

	utils.RespondWithSuccess(c, gin.H{
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"is_admin": user.IsAdmin,
		},
	})
}

// Logout 用户登出
// @Summary      用户登出
// @Description  清除session，退出登录
// @Tags         认证
// @Produce      json
// @Success      200 {object} utils.SwaggerResponse
// @Router       /auth/logout [post]
// Logout 处理用户登出请求
// 从会话存储中删除会话，使Cookie失效
func Logout(c *gin.Context) {
	sessionID, err := c.Cookie("session_id")
	if err == nil {
		utils.DeleteSession(sessionID)
	}
	c.SetCookie("session_id", "", -1, "/", "", true, true)
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "登出成功",
		"data":    nil,
	})
}

// GetMe 获取当前用户
// @Summary      当前用户信息
// @Description  获取当前登录用户的信息
// @Tags         认证
// @Produce      json
// @Security     SessionAuth
// @Success      200 {object} utils.SwaggerResponse{data=object{id=uint,username=string,is_admin=bool}}
// @Failure      401 {object} utils.SwaggerResponse
// @Router       /auth/me [get]
// GetMe 获取当前登录用户信息
// 从请求上下文获取已通过AuthMiddleware验证的用户信息
func GetMe(c *gin.Context) {
	u, ok := requireUser(c)
	if !ok {
		return
	}
	utils.RespondWithSuccess(c, gin.H{
		"id":       u.ID,
		"username": u.Username,
		"is_admin": u.IsAdmin,
	})
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
