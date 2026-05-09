package handlers

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"xiaoquan-backend/config"
	"xiaoquan-backend/models"
	"xiaoquan-backend/utils"

	"github.com/gin-gonic/gin"
)

type AuditRequest struct {
	AdminID uint              `json:"admin_id" binding:"required"`
	Status  models.AuditStatus `json:"status" binding:"required"`
	Remark  string              `json:"remark"`
}

func AuditContent(c *gin.Context) {
	id := c.Param("id")
	var content models.Content

	if err := utils.DB.First(&content, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "content not found",
			"data":    nil,
		})
		return
	}

	var req AuditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
			"data":    nil,
		})
		return
	}

	oldStatus := content.AuditStatus
	content.AuditStatus = req.Status
	if err := utils.DB.Save(&content).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to update content",
			"data":    nil,
		})
		return
	}

	if oldStatus != models.AuditStatusApproved && req.Status == models.AuditStatusApproved {
		if utils.RedisClient != nil {
			go ClearContentListCache()
		}
	}

	auditLog := &models.AuditLog{
		ContentID: content.ID,
		AdminID:   req.AdminID,
		Status:    req.Status,
		Remark:    req.Remark,
	}
	if err := utils.DB.Create(auditLog).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to create audit log",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "审核成功",
		"data":    content,
	})
}

func GetPendingContent(c *gin.Context) {
	var contents []models.Content
	var total int64

	query := utils.DB.Model(&models.Content{}).Where("audit_status = ?", models.AuditStatusPending)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	query.Count(&total)

	query = query.Preload("User").
		Order("created_at DESC").Limit(pageSize).Offset(offset)

	if err := query.Find(&contents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to get pending contents",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data": gin.H{
			"list":       contents,
			"total":      total,
			"page":       page,
			"page_size":  pageSize,
			"total_page": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

func GetAllContent(c *gin.Context) {
	var contents []models.Content
	var total int64

	query := utils.DB.Model(&models.Content{})

	if tag := c.Query("tag"); tag != "" {
		query = query.Where("JSON_CONTAINS(tags, ?)", "\""+tag+"\"")
	}

	if contentType := c.Query("type"); contentType != "" {
		query = query.Where("type = ?", contentType)
	}

	if auditStatus := c.Query("audit_status"); auditStatus != "" {
		query = query.Where("audit_status = ?", auditStatus)
	}

	if keyword := c.Query("keyword"); keyword != "" {
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	sortBy := c.DefaultQuery("sort_by", "created_at")
	order := c.DefaultQuery("order", "desc")
	validSortFields := map[string]bool{"created_at": true, "updated_at": true, "id": true}
	validOrders := map[string]bool{"asc": true, "desc": true}
	if !validSortFields[sortBy] {
		sortBy = "created_at"
	}
	if !validOrders[order] {
		order = "desc"
	}
	orderStr := sortBy + " " + order

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	query.Count(&total)

	query = query.Preload("User").
		Order(orderStr).Limit(pageSize).Offset(offset)

	if err := query.Find(&contents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to get contents",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data": gin.H{
			"list":       contents,
			"total":      total,
			"page":       page,
			"page_size":  pageSize,
			"total_page": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

func GetUsers(c *gin.Context) {
	var users []models.User
	var total int64

	query := utils.DB.Model(&models.User{})

	if keyword := c.Query("keyword"); keyword != "" {
		query = query.Where("username LIKE ?", "%"+keyword+"%")
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	query.Count(&total)

	query = query.Order("created_at DESC").Limit(pageSize).Offset(offset)

	if err := query.Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to get users",
			"data":    nil,
		})
		return
	}

	for i := range users {
		users[i].Password = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data": gin.H{
			"list":       users,
			"total":      total,
			"page":       page,
			"page_size":  pageSize,
			"total_page": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

func UpdateUserRole(c *gin.Context) {
	id := c.Param("id")
	var user models.User

	if err := utils.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "用户不存在",
			"data":    nil,
		})
		return
	}

	var req struct {
		IsAdmin bool `json:"is_admin" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
			"data":    nil,
		})
		return
	}

	user.IsAdmin = req.IsAdmin

	if err := utils.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新失败",
			"data":    nil,
		})
		return
	}

	user.Password = ""

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "更新成功",
		"data":    user,
	})
}

func DeleteUser(c *gin.Context) {
	id := c.Param("id")
	var user models.User

	if err := utils.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "用户不存在",
			"data":    nil,
		})
		return
	}

	var contents []models.Content
	if err := utils.DB.Where("user_id = ?", user.ID).Find(&contents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询用户内容失败",
			"data":    nil,
		})
		return
	}

	for _, content := range contents {
		if content.FilePath != "" {
			filePath := filepath.Join(config.AppConfig.Server.UploadDir, content.FilePath)
			if err := os.Remove(filePath); err != nil {
				log.Println("Failed to delete content file:", filePath, err)
			}

			if content.Type == models.ContentTypeVideo && content.ThumbPath != "" {
				thumbPath := filepath.Join(config.AppConfig.Server.UploadDir, content.ThumbPath)
				if err := os.Remove(thumbPath); err != nil {
					log.Println("Failed to delete thumbnail:", thumbPath, err)
				}
			}
		}

		if err := utils.DB.Delete(&content).Error; err != nil {
			log.Println("Failed to delete content record:", content.ID, err)
		}
	}

	if err := utils.DB.Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除失败",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "删除成功",
		"data":    nil,
	})
}

type BanUserRequest struct {
	IsBanned bool `json:"is_banned" binding:"required"`
}

func BanUser(c *gin.Context) {
	id := c.Param("id")
	var user models.User

	if err := utils.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "用户不存在",
			"data":    nil,
		})
		return
	}

	if user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "不能封禁管理员",
			"data":    nil,
		})
		return
	}

	var req BanUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
			"data":    nil,
		})
		return
	}

	user.IsBanned = req.IsBanned

	if err := utils.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新失败",
			"data":    nil,
		})
		return
	}

	user.Password = ""

	message := "解封成功"
	if req.IsBanned {
		message = "封禁成功"
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": message,
		"data":    user,
	})
}
