package handlers

import (
	"crypto/rand"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"log"
	"math/big"
	"net/http"
	"xiaoquan-backend/config"
	"xiaoquan-backend/models"
	"xiaoquan-backend/utils"
)

func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// InitAdmin 初始化管理员
// @Summary      初始化管理员
// @Description  首次运行时创建管理员账号，密码在服务器日志中查看（仅当无管理员时可用）
// @Tags         认证
// @Produce      json
// @Success      200 {object} utils.SwaggerResponse{data=object{admin_id=uint,username=string}}
// @Failure      403 {object} utils.SwaggerResponse
// @Router       /auth/init-admin [post]
func InitAdmin(c *gin.Context) {
	if !config.AppConfig.InitAdmin {
		utils.RespondWithError(c, http.StatusForbidden, "未启用初始化管理员功能，请在配置中设置 init_admin: true")
		return
	}

	var count int64
	utils.DB.Model(&models.User{}).Where("is_admin = ?", true).Count(&count)
	if count > 0 {
		config.AppConfig.InitAdmin = false
		config.SaveConfig("./config/config.yaml")
		utils.RespondWithError(c, http.StatusForbidden, "管理员已存在，已自动关闭初始化开关")
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
		utils.RespondWithError(c, http.StatusInternalServerError, "密码加密失败")
		return
	}

	admin := &models.User{
		Username: username,
		Password: string(hashedPassword),
		IsAdmin:  true,
	}

	if err := utils.DB.Create(admin).Error; err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "创建管理员失败")
		return
	}

	config.AppConfig.InitAdmin = false
	if err := config.SaveConfig("./config/config.yaml"); err != nil {
		log.Printf("[管理员] 保存配置失败: %v", err)
	} else {
		log.Println("[管理员] 初始化完成，已关闭 init_admin 开关")
	}

	utils.RespondWithSuccess(c, gin.H{
		"admin_id": admin.ID,
		"username": username,
	})
}
