package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"xiaoquan-backend/config"
	"xiaoquan-backend/models"
	"xiaoquan-backend/services"
	"xiaoquan-backend/utils"

	"github.com/gin-gonic/gin"
)

type AuditRequest struct {
	AdminID uint              `json:"admin_id" binding:"required"`
	Status  models.AuditStatus `json:"status" binding:"required"`
	Remark  string              `json:"remark"`
}

// AuditContent 审核内容
// @Summary      审核内容
// @Description  管理员审核内容（通过/拒绝）
// @Tags         管理
// @Accept       json
// @Produce      json
// @Security     SessionAuth
// @Param        id   path int true "内容ID"
// @Param        body body object{admin_id=uint,status=string,remark=string} true "审核信息"
// @Success      200 {object} utils.SwaggerResponse{data=models.Content}
// @Failure      400 {object} utils.SwaggerResponse
// @Failure      401 {object} utils.SwaggerResponse
// @Failure      404 {object} utils.SwaggerResponse
// @Router       /admin/audit/{id} [post]
func AuditContent(c *gin.Context) {
	id := c.Param("id")
	var content models.Content

	if err := utils.DB.First(&content, id).Error; err != nil {
		utils.RespondWithError(c, http.StatusNotFound, "内容不存在")
		return
	}

	var req AuditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	content.AuditStatus = req.Status
	if err := utils.DB.Save(&content).Error; err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "更新内容失败")
		return
	}

	ClearContentListCache()

	auditLog := &models.AuditLog{
		ContentID: content.ID,
		AdminID:   req.AdminID,
		Status:    req.Status,
		Remark:    req.Remark,
	}
	if err := utils.DB.Create(auditLog).Error; err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "创建审核日志失败")
		return
	}

	utils.RespondWithSuccess(c, buildContentDetail(c, content))
}

// GetPendingContent 待审核内容列表
// @Summary      待审核内容
// @Description  管理员查看待审核内容列表
// @Tags         管理
// @Produce      json
// @Security     SessionAuth
// @Param        page      query int false "页码" default(1)
// @Param        page_size query int false "每页数量" default(20)
// @Success      200 {object} utils.SwaggerResponse{data=object{list=[]models.Content,total=int,page=int,page_size=int,total_page=int}}
// @Router       /admin/pending [get]
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
		utils.RespondWithError(c, http.StatusInternalServerError, "获取待审核内容失败")
		return
	}

	pendingList := make([]gin.H, 0, len(contents))
	for _, content := range contents {
		pendingList = append(pendingList, buildContentSummary(c, content))
	}

	utils.RespondWithSuccess(c, gin.H{
		"list":       pendingList,
		"total":      total,
		"page":       page,
		"page_size":  pageSize,
		"total_page": (total + int64(pageSize) - 1) / int64(pageSize),
	})
}

// GetAllContent 所有内容列表
// @Summary      所有内容
// @Description  管理员查看所有内容列表（支持筛选）
// @Tags         管理
// @Produce      json
// @Security     SessionAuth
// @Param        page         query int    false "页码" default(1)
// @Param        page_size    query int    false "每页数量" default(20)
// @Param        tag          query string false "标签筛选"
// @Param        type         query string false "内容类型" Enums(video, image, text, link)
// @Param        audit_status query string false "审核状态" Enums(approved, pending, rejected)
// @Param        keyword      query string false "关键词搜索"
// @Success      200 {object} utils.SwaggerResponse{data=object{list=[]models.Content,total=int,page=int,page_size=int,total_page=int}}
// @Router       /admin/content/all [get]
func GetAllContent(c *gin.Context) {
	var contents []models.Content
	var total int64

	query := utils.DB.Model(&models.Content{})

	if tag := c.Query("tag"); tag != "" {
		safeTag, _ := json.Marshal(tag)
		query = query.Where("JSON_CONTAINS(tags, ?)", string(safeTag))
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

	allList := make([]gin.H, 0, len(contents))
	for _, content := range contents {
		allList = append(allList, buildContentSummary(c, content))
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data": gin.H{
			"list":       allList,
			"total":      total,
			"page":       page,
			"page_size":  pageSize,
			"total_page": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

// GetUsers 用户列表
// @Summary      用户列表
// @Description  管理员查看所有用户
// @Tags         管理
// @Produce      json
// @Security     SessionAuth
// @Param        page      query int    false "页码" default(1)
// @Param        page_size query int    false "每页数量" default(20)
// @Param        keyword   query string false "用户名搜索"
// @Success      200 {object} utils.SwaggerResponse{data=object{list=[]models.User,total=int}}
// @Router       /admin/users [get]
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
		utils.RespondWithError(c, http.StatusInternalServerError, "获取用户列表失败")
		return
	}

	for i := range users {
		users[i].Password = ""
	}

	utils.RespondWithSuccess(c, gin.H{
		"list":       users,
		"total":      total,
		"page":       page,
		"page_size":  pageSize,
		"total_page": (total + int64(pageSize) - 1) / int64(pageSize),
	})
}

// UpdateUserRole 更新用户角色
// @Summary      更新用户角色
// @Description  设置/取消管理员
// @Tags         管理
// @Accept       json
// @Produce      json
// @Security     SessionAuth
// @Param        id   path int  true "用户ID"
// @Param        body body object{is_admin=bool} true "角色信息"
// @Success      200 {object} utils.SwaggerResponse{data=models.User}
// @Failure      400 {object} utils.SwaggerResponse
// @Failure      404 {object} utils.SwaggerResponse
// @Router       /admin/users/{id}/role [put]
func UpdateUserRole(c *gin.Context) {
	id := c.Param("id")
	var user models.User

	if err := utils.DB.First(&user, id).Error; err != nil {
		utils.RespondWithError(c, http.StatusNotFound, "用户不存在")
		return
	}

	var req struct {
		IsAdmin bool `json:"is_admin"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondWithError(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	user.IsAdmin = req.IsAdmin

	if err := utils.DB.Save(&user).Error; err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "更新失败")
		return
	}
	utils.ClearUserInfoCache(user.ID)

	user.Password = ""

	utils.RespondWithSuccess(c, user)
}

// DeleteUser 删除用户
// @Summary      删除用户
// @Description  删除用户及其所有内容
// @Tags         管理
// @Produce      json
// @Security     SessionAuth
// @Param        id path int true "用户ID"
// @Success      200 {object} utils.SwaggerResponse
// @Failure      404 {object} utils.SwaggerResponse
// @Router       /admin/users/{id} [delete]
func DeleteUser(c *gin.Context) {
	id := c.Param("id")
	var user models.User

	if err := utils.DB.First(&user, id).Error; err != nil {
		utils.RespondWithError(c, http.StatusNotFound, "用户不存在")
		return
	}

	var contents []models.Content
	if err := utils.DB.Where("user_id = ?", user.ID).Find(&contents).Error; err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "查询用户内容失败")
		return
	}

	for _, content := range contents {
		if content.FilePath != "" {
			filePath := filepath.Join(config.AppConfig.Server.UploadDir, content.FilePath)
			if err := os.Remove(filePath); err != nil {
				log.Println("Failed to delete content file:", filePath, err)
			}
		}

		if content.ThumbPath != "" {
			thumbPath := filepath.Join(config.AppConfig.Server.ThumbnailDir, content.ThumbPath)
			if err := os.Remove(thumbPath); err != nil {
				log.Println("Failed to delete thumbnail:", thumbPath, err)
			}
		}

		if content.CompressedPath != "" {
			compressedPath := filepath.Join(config.AppConfig.Server.UploadDir, "..", "images", content.CompressedPath)
			absPath, _ := filepath.Abs(compressedPath)
			if err := os.Remove(absPath); err != nil {
				log.Println("Failed to delete compressed image:", absPath, err)
			}
		}

		if err := utils.DB.Delete(&content).Error; err != nil {
			log.Println("Failed to delete content record:", content.ID, err)
		}
	}

	if err := utils.DB.Delete(&user).Error; err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "删除失败")
		return
	}

	utils.RespondWithSuccess(c, nil)
}

type BanUserRequest struct {
	IsBanned bool `json:"is_banned"`
}

// BanUser 封禁/解封用户
// @Summary      封禁/解封用户
// @Description  封禁或解封指定用户（不能封禁管理员）
// @Tags         管理
// @Accept       json
// @Produce      json
// @Security     SessionAuth
// @Param        id   path int  true "用户ID"
// @Param        body body object{is_banned=bool} true "封禁信息"
// @Success      200 {object} utils.SwaggerResponse{data=models.User}
// @Failure      400 {object} utils.SwaggerResponse
// @Failure      403 {object} utils.SwaggerResponse
// @Failure      404 {object} utils.SwaggerResponse
// @Router       /admin/users/{id}/ban [put]
func BanUser(c *gin.Context) {
	id := c.Param("id")
	var user models.User

	if err := utils.DB.First(&user, id).Error; err != nil {
		utils.RespondWithError(c, http.StatusNotFound, "用户不存在")
		return
	}

	if user.IsAdmin {
		utils.RespondWithError(c, http.StatusForbidden, "不能封禁管理员")
		return
	}

	var req BanUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondWithError(c, http.StatusBadRequest, "参数错误")
		return
	}

	user.IsBanned = req.IsBanned

	if err := utils.DB.Save(&user).Error; err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "更新失败")
		return
	}
	utils.ClearUserInfoCache(user.ID)

	user.Password = ""

	message := "解封成功"
	if req.IsBanned {
		message = "封禁成功"
	}

	utils.RespondWithSuccess(c, gin.H{
		"message": message,
		"data":    user,
	})
}

func RegenerateAllThumbnails(c *gin.Context) {
	var contents []models.Content
	query := utils.DB.Model(&models.Content{}).Where("type = ? AND file_path != ?", models.ContentTypeImage, "")
	if err := query.Find(&contents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询失败",
			"data":    nil,
		})
		return
	}

	var videoContents []models.Content
	utils.DB.Model(&models.Content{}).Where("type = ? AND file_path != ?", models.ContentTypeVideo, "").Find(&videoContents)
	contents = append(contents, videoContents...)

	var linkContents []models.Content
	utils.DB.Model(&models.Content{}).Where("type = ? AND url != ?", models.ContentTypeLink, "").Find(&linkContents)
	contents = append(contents, linkContents...)

	go func() {
		total := len(contents)
		success := 0
		for _, content := range contents {
			if content.Type == models.ContentTypeImage && content.FilePath != "" {
				originalPath := filepath.Join(config.AppConfig.Server.UploadDir, content.FilePath)
				thumbFilename, err := utils.GenerateImageThumbnail(originalPath, content.FilePath)
				if err != nil {
					log.Printf("Failed to generate thumbnail for content %d: %v", content.ID, err)
					continue
				}
				utils.DB.Model(&content).Update("thumb_path", thumbFilename)
				success++
			} else if content.Type == models.ContentTypeVideo && content.FilePath != "" {
				videoPath := filepath.Join(config.AppConfig.Server.UploadDir, content.FilePath)
				thumbFilename, err := utils.GenerateVideoThumbnail(videoPath, content.FilePath)
				if err != nil {
					log.Printf("Failed to generate video thumbnail for content %d: %v", content.ID, err)
					continue
				}
				utils.DB.Model(&content).Update("thumb_path", thumbFilename)
				success++
			} else if content.Type == models.ContentTypeLink && content.Url != "" {
				platform, sourceID, err := services.DetectPlatform(content.Url)
				if err != nil {
					continue
				}
				videoInfo, err := services.FetchVideoInfo(platform, sourceID)
				if err != nil {
					continue
				}
				if videoInfo.CoverURL == "" && videoInfo.Platform != services.PlatformDouyin {
					continue
				}
				coverFilename, err := services.DownloadCover(videoInfo.CoverURL, content.UserID, videoInfo.Platform == services.PlatformDouyin)
				if err != nil {
					log.Printf("Failed to download cover for content %d: %v", content.ID, err)
					continue
				}
				utils.DB.Model(&content).Update("thumb_path", coverFilename)
				success++
				success++
			}
		}
		log.Printf("[缩略图] 批量重新生成完成: 总计=%d 成功=%d", total, success)

		var allContents []models.Content
		utils.DB.Unscoped().Select("thumb_path", "compressed_path").Find(&allContents)
		recorded := make(map[string]bool)
		for _, c := range allContents {
			if c.ThumbPath != "" {
				recorded[c.ThumbPath] = true
			}
			if c.CompressedPath != "" {
				recorded[c.CompressedPath] = true
			}
		}
		deleted := 0
		thumbDir := config.AppConfig.Server.ThumbnailDir
		entries, _ := os.ReadDir(thumbDir)
		for _, e := range entries {
			if !e.IsDir() && !recorded[e.Name()] {
				if os.Remove(filepath.Join(thumbDir, e.Name())) == nil {
					deleted++
				}
			}
		}
		imagesDir := filepath.Join(config.AppConfig.Server.UploadDir, "..", "images")
		absImagesDir, _ := filepath.Abs(imagesDir)
		imgEntries, _ := os.ReadDir(absImagesDir)
		for _, e := range imgEntries {
			if !e.IsDir() && !recorded[e.Name()] {
				if os.Remove(filepath.Join(absImagesDir, e.Name())) == nil {
					deleted++
				}
			}
		}
		if deleted > 0 {
			log.Printf("[缩略图] 已清理 %d 个孤立文件", deleted)
		}

		ClearContentListCache()
	}()

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "正在后台生成缩略图",
		"data": gin.H{
			"count": len(contents),
		},
	})
}
