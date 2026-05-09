
package handlers

import (
	"log"
	"math/rand"
	"net/http"
	"time"
	"xiaoquan-backend/models"
	"xiaoquan-backend/utils"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func InitAdmin(c *gin.Context) {
	var count int64
	utils.DB.Model(&models.User{}).Where("is_admin = ?", true).Count(&count)
	if count > 0 {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "管理员已存在",
			"data":    nil,
		})
		return
	}

	username := "admin"
	password := generateRandomPassword(8)

	log.Println("========================================")
	log.Println("初始管理员账号已创建")
	log.Printf("用户名: %s", username)
	log.Printf("密码: %s", password)
	log.Println("========================================")

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "密码加密失败",
			"data":    nil,
		})
		return
	}

	admin := &models.User{
		Username: username,
		Password: string(hashedPassword),
		IsAdmin:  true,
	}

	if err := utils.DB.Create(admin).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建管理员失败",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "管理员创建成功，请查看后端日志获取密码",
		"data": gin.H{
			"admin_id": admin.ID,
			"username": username,
		},
	})
}
