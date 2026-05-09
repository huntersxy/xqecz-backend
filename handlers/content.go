package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"xiaoquan-backend/config"
	"xiaoquan-backend/models"
	"xiaoquan-backend/services"
	"xiaoquan-backend/utils"

	"github.com/gin-gonic/gin"
)

const (
	DefaultPageSize = 20
	MaxPageSize    = 100
	MaxRecommendCount = 50
	CacheDuration5Min = 5 * time.Minute
	CacheDuration12Hour = 12 * time.Hour
)

func UploadArticleImage(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		utils.RespondWithError(c, http.StatusUnauthorized, "未登录")
		return
	}
	currentUser := user.(models.User)

	file, err := c.FormFile("file")
	if err != nil {
		utils.RespondWithError(c, http.StatusBadRequest, "请上传图片文件")
		return
	}

	uploadService := services.NewFileUploadService(&services.FileUploadConfig{
		AllowedExtensions: []string{".jpg", ".jpeg", ".png", ".gif", ".webp"},
		MaxSize:           10 * 1024 * 1024,
		UserID:            currentUser.ID,
	})

	result, err := uploadService.Upload(file)
	if err != nil {
		utils.RespondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	imageURL := fmt.Sprintf("http://%s%s", c.Request.Host, result.URL)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "上传成功",
		"data": gin.H{
			"id":          0,
			"filename":    result.Filename,
			"file_size":   result.FileSize,
			"image_url":   imageURL,
			"upload_time": time.Now().Format(time.RFC3339),
		},
	})
}

type UploadRequest struct {
	Title   string             `form:"title" binding:"required"`
	Type    models.ContentType `form:"type" binding:"required"`
	Content string             `form:"content"`
	UserID  uint               `form:"user_id" binding:"required"`
	Tags    []string           `form:"tags"`
}

func UploadContent(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
			"data":    nil,
		})
		return
	}
	currentUser := user.(models.User)

	var req UploadRequest
	if err := c.ShouldBind(&req); err != nil {
		utils.RespondWithError(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}

	if !utils.ValidateContentTitle(req.Title) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "标题无效（1-200字符）",
			"data":    nil,
		})
		return
	}

	if req.Type == models.ContentTypeText && !utils.ValidateTextContent(req.Content) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "内容过长",
			"data":    nil,
		})
		return
	}

	content := &models.Content{
		Title:       utils.SanitizeHTML(req.Title),
		Type:        req.Type,
		Content:     utils.SanitizeHTML(req.Content),
		UserID:      currentUser.ID,
		AuditStatus: models.AuditStatusPending,
	}

	if len(req.Tags) > 0 {
		uniqueTags := make([]string, 0)
		tagSet := make(map[string]bool)
		for _, tag := range req.Tags {
			if tag != "" && !tagSet[tag] {
				tagSet[tag] = true
				uniqueTags = append(uniqueTags, tag)
			}
		}
		content.Tags = uniqueTags
	}

	if req.Type == models.ContentTypeVideo || req.Type == models.ContentTypeImage {
		file, err := c.FormFile("file")
		if err != nil {
			utils.RespondWithError(c, http.StatusBadRequest, "请上传文件")
			return
		}

		var allowedExtensions []string
		var maxSize int64

		if req.Type == models.ContentTypeImage {
			allowedExtensions = []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
			maxSize = 10 * 1024 * 1024
		} else {
			allowedExtensions = []string{".mp4", ".avi", ".mov", ".mkv"}
			maxSize = 500 * 1024 * 1024
		}

		uploadService := services.NewFileUploadService(&services.FileUploadConfig{
			AllowedExtensions: allowedExtensions,
			MaxSize:           maxSize,
			UserID:            currentUser.ID,
		})

		result, err := uploadService.Upload(file)
		if err != nil {
			utils.RespondWithError(c, http.StatusBadRequest, err.Error())
			return
		}

		content.FilePath = result.Filename
		content.FileSize = result.FileSize

		if req.Type == models.ContentTypeVideo {
			thumbFilename, err := utils.GenerateVideoThumbnail(result.FilePath, result.Filename)
			if err != nil {
				log.Printf("Warning: failed to generate video thumbnail: %v", err)
			} else {
				content.ThumbPath = thumbFilename
			}
		}
	}

	if err := utils.DB.Create(content).Error; err != nil {
		log.Printf("[错误] 创建内容失败: user_id=%d, error=%v", currentUser.ID, err)
		utils.RespondWithError(c, http.StatusInternalServerError, "创建内容失败，请稍后重试")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "上传成功",
		"data":    content,
	})
}

type ContentListCache struct {
	List      []gin.H `json:"list"`
	Total     int64   `json:"total"`
	Page      int     `json:"page"`
	PageSize  int     `json:"page_size"`
	TotalPage int64   `json:"total_page"`
}

func GetContentList(c *gin.Context) {
	var contents []models.Content
	var total int64

	cacheParams := map[string]string{
		"audit_status": c.Query("audit_status"),
		"tag":          c.Query("tag"),
		"type":         c.Query("type"),
		"keyword":      c.Query("keyword"),
		"sort_by":      c.DefaultQuery("sort_by", "created_at"),
		"order":        c.DefaultQuery("order", "desc"),
		"page":         c.DefaultQuery("page", "1"),
		"page_size":    c.DefaultQuery("page_size", fmt.Sprintf("%d", DefaultPageSize)),
	}
	cacheKey := generateSortedCacheKey("content_list:", cacheParams)

	if utils.RedisClient != nil {
		var cacheData ContentListCache
		if err := utils.GetCacheJSON(cacheKey, &cacheData); err == nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "获取成功",
				"data": gin.H{
					"list":       cacheData.List,
					"total":      cacheData.Total,
					"page":       cacheData.Page,
					"page_size":  cacheData.PageSize,
					"total_page": cacheData.TotalPage,
				},
			})
			return
		}
	}

	query := utils.DB.Model(&models.Content{})

	if auditStatus := c.Query("audit_status"); auditStatus != "" {
		if auditStatus != "all" {
			query = query.Where("audit_status = ?", auditStatus)
		}
	} else {
		query = query.Where("audit_status = ?", models.AuditStatusApproved)
	}

	if tag := c.Query("tag"); tag != "" {
		tags := strings.Split(tag, ",")
		for _, t := range tags {
			t = strings.TrimSpace(t)
			if t != "" {
				safeTag := sanitizeSearchInput(t)
				query = query.Where("JSON_CONTAINS(tags, ?)", "\""+safeTag+"\"")
			}
		}
	}

	if contentType := c.Query("type"); contentType != "" {
		query = query.Where("type = ?", contentType)
	}

	if keyword := c.Query("keyword"); keyword != "" {
		safeKeyword := sanitizeSearchInput(keyword)
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+safeKeyword+"%", "%"+safeKeyword+"%")
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

	results := make([]gin.H, 0, len(contents))
	for _, content := range contents {
		result := buildContentResponse(c, content)
		results = append(results, result)
	}

	totalPage := (total + int64(pageSize) - 1) / int64(pageSize)

	if utils.RedisClient != nil {
		cacheData := ContentListCache{
			List:      results,
			Total:     total,
			Page:      page,
			PageSize:  pageSize,
			TotalPage: totalPage,
		}
		go utils.SetCacheJSON(cacheKey, cacheData, CacheDuration5Min)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data": gin.H{
			"list":       results,
			"total":      total,
			"page":       page,
			"page_size":  pageSize,
			"total_page": totalPage,
		},
	})
}

func GetMyContentList(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
			"data":    nil,
		})
		return
	}
	u := user.(models.User)

	var contents []models.Content
	var total int64

	query := utils.DB.Model(&models.Content{}).Where("user_id = ?", u.ID)

	if bigTagID := c.Query("big_tag_id"); bigTagID != "" {
		if id, err := strconv.ParseUint(bigTagID, 10, 32); err == nil {
			query = query.Where("big_tag_id = ?", id)
		}
	}

	if smallTagID := c.Query("small_tag_id"); smallTagID != "" {
		if id, err := strconv.ParseUint(smallTagID, 10, 32); err == nil {
			query = query.Where("small_tag_id = ?", id)
		}
	}

	if contentType := c.Query("type"); contentType != "" {
		query = query.Where("type = ?", contentType)
	}

	if auditStatus := c.Query("audit_status"); auditStatus != "" {
		query = query.Where("audit_status = ?", auditStatus)
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

func GetContent(c *gin.Context) {
	id := c.Param("id")
	var content models.Content

	cacheKey := "content:" + id
	if utils.RedisClient != nil {
		if err := utils.GetCacheJSON(cacheKey, &content); err == nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "获取成功",
				"data":    content,
			})
			return
		}
	}

	if err := utils.DB.Preload("User").First(&content, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "content not found",
			"data":    nil,
		})
		return
	}

	if utils.RedisClient != nil {
		utils.SetCacheJSON(cacheKey, content, 12*time.Hour)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    content,
	})
}

func UpdateContent(c *gin.Context) {
	id := c.Param("id")
	var content models.Content

	if err := utils.DB.First(&content, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "内容不存在",
			"data":    nil,
		})
		return
	}

	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
			"data":    nil,
		})
		return
	}

	currentUser := user.(models.User)
	if !currentUser.IsAdmin && content.UserID != currentUser.ID {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "无权修改此内容",
			"data":    nil,
		})
		return
	}

	title := c.PostForm("title")
	newContent := c.PostForm("content")
	tags := c.PostFormArray("tags")

	if title != "" {
		if !utils.ValidateContentTitle(title) {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "标题无效（1-200字符）",
				"data":    nil,
			})
			return
		}
		content.Title = utils.SanitizeHTML(title)
	}

	if content.Type == models.ContentTypeText && newContent != "" {
		if !utils.ValidateTextContent(newContent) {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "内容过长",
				"data":    nil,
			})
			return
		}
		content.Content = utils.SanitizeHTML(newContent)
	}

	if len(tags) > 0 {
		uniqueTags := make([]string, 0)
		tagSet := make(map[string]bool)
		for _, tag := range tags {
			if tag != "" && !tagSet[tag] {
				tagSet[tag] = true
				uniqueTags = append(uniqueTags, tag)
			}
		}
		content.Tags = uniqueTags
	}

	if content.Type == models.ContentTypeVideo || content.Type == models.ContentTypeImage {
		file, err := c.FormFile("file")
		if err == nil {
			if content.FilePath != "" {
				deleteService := services.NewFileUploadService(&services.FileUploadConfig{
					UserID: content.UserID,
				})
				if err := deleteService.DeleteFile(content.FilePath); err != nil {
					log.Printf("Warning: failed to delete old file: %v", err)
				}

				if content.Type == models.ContentTypeVideo && content.ThumbPath != "" {
					if err := deleteService.DeleteFile(content.ThumbPath); err != nil {
						log.Printf("Warning: failed to delete old thumbnail: %v", err)
					}
				}
			}

			var allowedExtensions []string
			var maxSize int64

			if content.Type == models.ContentTypeImage {
				allowedExtensions = []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
				maxSize = 10 * 1024 * 1024
			} else {
				allowedExtensions = []string{".mp4", ".avi", ".mov", ".mkv"}
				maxSize = 500 * 1024 * 1024
			}

			uploadService := services.NewFileUploadService(&services.FileUploadConfig{
				AllowedExtensions: allowedExtensions,
				MaxSize:           maxSize,
				UserID:            content.UserID,
			})

			result, err := uploadService.Upload(file)
			if err != nil {
				utils.RespondWithError(c, http.StatusBadRequest, err.Error())
				return
			}

			content.FilePath = result.Filename
			content.FileSize = result.FileSize

			if content.Type == models.ContentTypeVideo {
				thumbFilename, err := utils.GenerateVideoThumbnail(result.FilePath, result.Filename)
				if err != nil {
					log.Printf("Warning: failed to generate video thumbnail: %v", err)
				} else {
					content.ThumbPath = thumbFilename
				}
			}
		}
	}

	content.AuditStatus = models.AuditStatusPending

	if err := utils.DB.Save(&content).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新失败",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "更新成功",
		"data":    content,
	})
}

func RegenerateVideoThumbnail(c *gin.Context) {
	id := c.Param("id")
	var content models.Content

	if err := utils.DB.First(&content, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "内容不存在",
			"data":    nil,
		})
		return
	}

	if content.Type != models.ContentTypeVideo {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "仅支持视频内容",
			"data":    nil,
		})
		return
	}

	if content.FilePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "视频文件不存在",
			"data":    nil,
		})
		return
	}

	videoPath := filepath.Join(config.AppConfig.Server.UploadDir, content.FilePath)
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "视频文件不存在",
			"data":    nil,
		})
		return
	}

	if content.ThumbPath != "" {
		oldThumbPath := filepath.Join(config.AppConfig.Server.UploadDir, content.ThumbPath)
		if err := os.Remove(oldThumbPath); err != nil {
			log.Println("Failed to delete old thumbnail:", oldThumbPath, err)
		}
	}

	thumbFilename, err := utils.GenerateVideoThumbnail(videoPath, content.FilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成缩略图失败: " + err.Error(),
			"data":    nil,
		})
		return
	}

	content.ThumbPath = thumbFilename
	if err := utils.DB.Save(&content).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新失败",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "缩略图更新成功",
		"data": gin.H{
			"id":         content.ID,
			"thumb_path": content.ThumbPath,
		},
	})
}

func DeleteContent(c *gin.Context) {
	id := c.Param("id")
	var content models.Content

	if err := utils.DB.First(&content, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "内容不存在",
			"data":    nil,
		})
		return
	}

	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
			"data":    nil,
		})
		return
	}

	currentUser := user.(models.User)
	if !currentUser.IsAdmin && content.UserID != currentUser.ID {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "无权删除此内容",
			"data":    nil,
		})
		return
	}

	if content.FilePath != "" {
		filePath := filepath.Join(config.AppConfig.Server.UploadDir, content.FilePath)
		if err := os.Remove(filePath); err != nil {
			log.Println("Failed to delete file:", filePath, err)
		}

		if content.Type == models.ContentTypeVideo && content.ThumbPath != "" {
			thumbPath := filepath.Join(config.AppConfig.Server.UploadDir, content.ThumbPath)
			if err := os.Remove(thumbPath); err != nil {
				log.Println("Failed to delete thumbnail:", thumbPath, err)
			}
		}
	}

	utils.DB.Where("content_id = ?", content.ID).Delete(&models.AuditLog{})
	utils.DB.Where("content_id = ?", content.ID).Delete(&models.Comment{})

	if err := utils.DB.Delete(&content).Error; err != nil {
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

func SearchContent(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请输入搜索关键词",
			"data":    nil,
		})
		return
	}

	var contents []models.Content
	var total int64

	query := utils.DB.Model(&models.Content{}).Where("audit_status = ?", models.AuditStatusApproved).
		Where("title LIKE ? OR content LIKE ?", "%"+keyword+"%", "%"+keyword+"%")

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
		utils.RespondWithError(c, http.StatusInternalServerError, "搜索失败")
		return
	}

	results := make([]gin.H, 0, len(contents))
	for _, content := range contents {
		result := buildContentResponse(c, content)
		results = append(results, result)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "搜索成功",
		"data": gin.H{
			"list":       results,
			"total":      total,
			"page":       page,
			"page_size":  pageSize,
			"total_page": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

func RecommendContent(c *gin.Context) {
	var contents []models.Content

	countStr := c.Query("count")
	if countStr == "" {
		utils.RespondWithError(c, http.StatusBadRequest, "count参数必填")
		return
	}

	count, err := strconv.Atoi(countStr)
	if err != nil || count < 1 {
		utils.RespondWithError(c, http.StatusBadRequest, "count参数无效，必须是大于0的整数")
		return
	}

	if count > MaxRecommendCount {
		count = MaxRecommendCount
	}

	var total int64
	query := utils.DB.Model(&models.Content{}).Where("audit_status = ?", models.AuditStatusApproved)

	if tag := c.Query("tag"); tag != "" {
		tags := strings.Split(tag, ",")
		for _, t := range tags {
			t = strings.TrimSpace(t)
			if t != "" {
				safeTag := sanitizeSearchInput(t)
				query = query.Where("JSON_CONTAINS(tags, ?)", "\""+safeTag+"\"")
			}
		}
	}

	query.Count(&total)
	if total == 0 {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "获取推荐成功",
			"data": gin.H{
				"list":  []gin.H{},
				"count": 0,
			},
		})
		return
	}

	if int(total) <= count {
		query = query.Preload("User").Order("created_at DESC").Limit(count)
	} else {
		var maxID, minID uint
		utils.DB.Model(&models.Content{}).Select("MAX(id)", "MIN(id)").Row().Scan(&maxID, &minID)
		if maxID > minID {
			randomOffset := uint(time.Now().UnixNano() % int64(maxID-minID))
			startID := minID + randomOffset
			subQuery := utils.DB.Model(&models.Content{}).
				Where("id >= ? AND audit_status = ?", startID, models.AuditStatusApproved).
				Order("id ASC").Limit(count)

			if tag := c.Query("tag"); tag != "" {
				tags := strings.Split(tag, ",")
				for _, t := range tags {
					t = strings.TrimSpace(t)
					if t != "" {
						safeTag := sanitizeSearchInput(t)
						subQuery = subQuery.Where("JSON_CONTAINS(tags, ?)", "\""+safeTag+"\"")
					}
				}
			}

			if err := subQuery.Preload("User").Find(&contents).Error; err != nil {
				utils.RespondWithError(c, http.StatusInternalServerError, "获取推荐失败")
				return
			}

			if len(contents) < count {
				var remaining []models.Content
				remainingQuery := utils.DB.Model(&models.Content{}).
					Where("id < ? AND audit_status = ?", startID, models.AuditStatusApproved)

				if tag := c.Query("tag"); tag != "" {
					tags := strings.Split(tag, ",")
					for _, t := range tags {
						t = strings.TrimSpace(t)
						if t != "" {
							safeTag := sanitizeSearchInput(t)
							remainingQuery = remainingQuery.Where("JSON_CONTAINS(tags, ?)", "\""+safeTag+"\"")
						}
					}
				}

				remainingQuery.Preload("User").Order("id DESC").Limit(count - len(contents)).Find(&remaining)
				contents = append(contents, remaining...)
			}
		} else {
			query = query.Preload("User").Order("created_at DESC").Limit(count)
		}
	}

	if len(contents) == 0 {
		query.Preload("User").Find(&contents)
	}

	results := make([]gin.H, 0, len(contents))
	for _, content := range contents {
		result := buildContentResponse(c, content)
		results = append(results, result)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取推荐成功",
		"data": gin.H{
			"list":  results,
			"count": len(results),
		},
	})
}

func GetAllTags(c *gin.Context) {
	var contents []models.Content
	if err := utils.DB.Select("tags").Find(&contents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取标签失败",
			"data":    nil,
		})
		return
	}

	tagSet := make(map[string]bool)
	for _, content := range contents {
		if len(content.Tags) > 0 {
			for _, tag := range content.Tags {
				if tag != "" {
					tagSet[tag] = true
				}
			}
		}
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    tags,
	})
}

func ClearContentListCache() {
	if utils.RedisClient == nil {
		return
	}
	ctx := context.Background()
	iter := utils.RedisClient.Scan(ctx, 0, "xiaoquan:content_list:*", 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		utils.RedisClient.Del(ctx, key)
	}
	if err := iter.Err(); err != nil {
		log.Printf("Error clearing content list cache: %v", err)
	}
}

func getThumbnailPath(c *gin.Context, filePath string) string {
	if filePath == "" {
		return ""
	}

	originalPath := filepath.Join(config.AppConfig.Server.UploadDir, filePath)
	thumbFilename, err := utils.GenerateImageThumbnail(originalPath, filePath)
	if err != nil {
		log.Printf("Warning: failed to generate thumbnail for %s: %v", filePath, err)
		return "/uploads/" + filePath
	}

	return "/thumbnails/" + thumbFilename
}

func sanitizeSearchInput(input string) string {
	input = strings.TrimSpace(input)
	input = strings.ReplaceAll(input, "%", "\\%")
	input = strings.ReplaceAll(input, "_", "\\_")
	return input
}

func buildContentResponse(c *gin.Context, content models.Content) gin.H {
	result := gin.H{
		"ID":          content.ID,
		"Title":       content.Title,
		"Type":        content.Type,
		"Content":     content.Content,
		"FilePath":    content.FilePath,
		"FileSize":    content.FileSize,
		"UserID":      content.UserID,
		"Tags":        content.Tags,
		"AuditStatus": content.AuditStatus,
		"CreatedAt":   content.CreatedAt,
		"UpdatedAt":   content.UpdatedAt,
		"image":       "",
		"video":       "",
	}

	if content.Type == models.ContentTypeImage && content.FilePath != "" {
		thumbPath := getThumbnailPath(c, content.FilePath)
		result["image"] = "http://" + c.Request.Host + thumbPath
	}

	if content.Type == models.ContentTypeVideo && content.FilePath != "" {
		result["video"] = "http://" + c.Request.Host + "/uploads/" + content.FilePath
		if content.ThumbPath != "" {
			thumbPath := getThumbnailPath(c, content.ThumbPath)
			result["image"] = "http://" + c.Request.Host + thumbPath
		}
	}

	if content.User.ID > 0 {
		result["User"] = gin.H{
			"ID":       content.User.ID,
			"Username": content.User.Username,
			"IsAdmin":  content.User.IsAdmin,
		}
	}

	return result
}

func generateSortedCacheKey(prefix string, params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	return prefix + strings.Join(parts, "&")
}

func parsePaginationParams(c *gin.Context) (page int, pageSize int, offset int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", fmt.Sprintf("%d", DefaultPageSize)))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}

	offset = (page - 1) * pageSize
	return
}
