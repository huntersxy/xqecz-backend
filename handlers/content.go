package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
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
	"gorm.io/gorm"
)

const (
	DefaultPageSize = 20
	MaxPageSize    = 100
	MaxRecommendCount = 50
	CacheDuration5Min = 5 * time.Minute
	CacheDuration1Hour = 1 * time.Hour
	CacheDuration12Hour = 12 * time.Hour
)

const (
	sqlAuditStatus  = "audit_status = ?"
	sqlContentID    = "content_id = ?"
	sqlOrderCreated = "created_at DESC"
	thumbnailPath   = "/thumbnails/"
)

type httpError struct {
	code    int
	message string
}

func (e *httpError) Error() string { return e.message }

// UploadArticleImage 上传文章图片
// @Summary      上传文章图片
// @Description  上传文章内嵌图片（支持jpg/jpeg/png/gif/webp）
// @Tags         内容管理
// @Accept       multipart/form-data
// @Produce      json
// @Param        file formData file true "图片文件（≤10MB）"
// @Security     SessionAuth
// @Success      200 {object} utils.SwaggerResponse{data=object{id=int,filename=string,file_size=int64,image_url=string,upload_time=string}}
// @Failure      400 {object} utils.SwaggerResponse
// @Failure      401 {object} utils.SwaggerResponse
// @Router       /content/upload-image [post]
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

	imageURL := result.URL

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
	Url     string             `form:"url"`
	UserID  uint               `form:"user_id" binding:"required"`
	Tags    []string           `form:"tags"`
}

// UploadContent 上传内容
// @Summary      上传内容
// @Description  上传新内容。支持类型：video(视频)、image(图片)、text(文字)、link(链接，B站/抖音/YouTube视频链接自动抓取封面和标题)
// @Tags         内容管理
// @Accept       multipart/form-data
// @Produce      json
// @Param        title   formData string   true  "标题（1-200字符）"
// @Param        type    formData string   true  "内容类型" Enums(video, image, text, link)
// @Param        content formData string   false "文字内容（type=text时有效）"
// @Param        url     formData string   false "外部链接（type=link时必填，B站/抖音/YouTube视频链接会自动抓取封面和标题）"
// @Param        tags    formData []string false "标签列表"
// @Param        file    formData file     false "文件（type=video或image时必填）"
// @Security     SessionAuth
// @Success      200 {object} utils.SwaggerResponse{data=models.Content}
// @Failure      400 {object} utils.SwaggerResponse
// @Failure      401 {object} utils.SwaggerResponse
// @Failure      403 {object} utils.SwaggerResponse
// @Router       /content/upload [post]
func UploadContent(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录", "data": nil})
		return
	}
	currentUser := user.(models.User)

	var req UploadRequest
	if err := c.ShouldBind(&req); err != nil {
		utils.RespondWithError(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}

	if he := validateUploadRequest(&req); he != nil {
		c.JSON(he.code, gin.H{"code": he.code, "message": he.message, "data": nil})
		return
	}

	content := &models.Content{
		Title:       utils.SanitizeHTML(req.Title),
		Type:        req.Type,
		Content:     utils.SanitizeHTML(req.Content),
		Url:         req.Url,
		UserID:      currentUser.ID,
		AuditStatus: models.AuditStatusPending,
	}

	if len(req.Tags) > 0 {
		content.Tags = deduplicateTags(req.Tags)
	}

	if req.Type == models.ContentTypeLink {
		processUploadLink(content, req.Url, currentUser.ID)
	}

	if req.Type == models.ContentTypeVideo || req.Type == models.ContentTypeImage {
		if he := handleUploadFile(c, content, &req); he != nil {
			utils.RespondWithError(c, he.code, he.message)
			return
		}
	}

	if err := utils.DB.Create(content).Error; err != nil {
		log.Printf("[错误] 创建内容失败: user_id=%d, error=%v", currentUser.ID, err)
		utils.RespondWithError(c, http.StatusInternalServerError, "创建内容失败，请稍后重试")
		return
	}

	go func() {
		ClearContentListCache()
		ClearRecommendCache()
	}()

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "上传成功", "data": buildContentDetail(c, *content)})
}

func validateUploadRequest(req *UploadRequest) *httpError {
	if !utils.ValidateContentTitle(req.Title) {
		return &httpError{code: http.StatusBadRequest, message: "标题无效（1-200字符）"}
	}
	if req.Type == models.ContentTypeText && !utils.ValidateTextContent(req.Content) {
		return &httpError{code: http.StatusBadRequest, message: "内容过长"}
	}
	if req.Type == models.ContentTypeLink && req.Url == "" {
		return &httpError{code: http.StatusBadRequest, message: "链接不能为空"}
	}
	return nil
}

func processUploadLink(content *models.Content, linkUrl string, userID uint) {
	platform, sourceID, err := services.DetectPlatform(linkUrl)
	if err != nil {
		return
	}
	videoInfo, err := services.FetchVideoInfo(platform, sourceID)
	if err != nil {
		log.Printf("Warning: failed to fetch video info for link: %v", err)
		return
	}
	content.Platform = string(videoInfo.Platform)
	if content.Title == "" {
		content.Title = utils.SanitizeHTML(videoInfo.Title)
	}
	if videoInfo.CoverURL != "" || videoInfo.Platform == services.PlatformDouyin {
		coverFilename, err := services.DownloadCover(videoInfo.CoverURL, userID, videoInfo.Platform == services.PlatformDouyin)
		if err != nil {
			log.Printf("Warning: failed to download cover for link: %v", err)
		} else {
			content.ThumbPath = coverFilename
		}
	}
}

func handleUploadFile(c *gin.Context, content *models.Content, req *UploadRequest) *httpError {
	file, err := c.FormFile("file")
	if err != nil {
		return &httpError{code: http.StatusBadRequest, message: "请上传文件"}
	}

	allowedExts, maxSize := getFileUploadConfig(req.Type)
	uploadService := services.NewFileUploadService(&services.FileUploadConfig{
		AllowedExtensions: allowedExts,
		MaxSize:           maxSize,
		UserID:            content.UserID,
	})

	result, err := uploadService.Upload(file)
	if err != nil {
		return &httpError{code: http.StatusBadRequest, message: err.Error()}
	}

	content.FilePath = result.Filename
	content.FileSize = result.FileSize
	generateContentThumb(content, result.FilePath, result.Filename)
	return nil
}

type ContentListCache struct {
	List      []gin.H `json:"list"`
	Total     int64   `json:"total"`
	Page      int     `json:"page"`
	PageSize  int     `json:"page_size"`
	TotalPage int64   `json:"total_page"`
}

// GetContentList 获取内容列表
// @Summary      获取内容列表
// @Description  获取公开内容列表，支持类型/标签/关键词筛选和排序
// @Tags         内容管理
// @Produce      json
// @Param        page         query int    false "页码" default(1)
// @Param        page_size    query int    false "每页数量" default(20) maximum(100)
// @Param        type         query string false "内容类型" Enums(video, image, text, link)
// @Param        tag          query string false "标签筛选"
// @Param        keyword      query string false "关键词搜索"
// @Param        audit_status query string false "审核状态" Enums(approved, pending, rejected, all) default(approved)
// @Param        sort_by      query string false "排序字段" Enums(created_at, updated_at, id) default(created_at)
// @Param        order        query string false "排序方向" Enums(asc, desc) default(desc)
// @Success      200 {object} utils.SwaggerResponse{data=object{list=[]models.Content,total=int,page=int,page_size=int,total_page=int}}
// @Router       /content/list [get]
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

	query := applyContentListFilters(utils.DB.Model(&models.Content{}), c)

	sortBy := c.DefaultQuery("sort_by", "created_at")
	order := c.DefaultQuery("order", "desc")
	orderStr := validateSortParams(sortBy, order)

	page, pageSize, offset := parsePaginationParams(c)

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
		result := buildContentSummary(c, content)
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
		go utils.SetCacheJSON(cacheKey, cacheData, CacheDuration1Hour)
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

func applyContentListFilters(query *gorm.DB, c *gin.Context) *gorm.DB {
	if auditStatus := c.Query("audit_status"); auditStatus != "" {
		if auditStatus != "all" {
			query = query.Where(sqlAuditStatus, auditStatus)
		}
	} else {
		query = query.Where(sqlAuditStatus, models.AuditStatusApproved)
	}

	if tag := c.Query("tag"); tag != "" {
		tags := strings.Split(tag, ",")
		for _, t := range tags {
			t = strings.TrimSpace(t)
			if t != "" {
				safeTag, _ := json.Marshal(t)
				query = query.Where("JSON_CONTAINS(tags, ?)", string(safeTag))
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

	return query
}

func validateSortParams(sortBy, order string) string {
	validSortFields := map[string]bool{"created_at": true, "updated_at": true, "id": true}
	validOrders := map[string]bool{"asc": true, "desc": true}
	if !validSortFields[sortBy] {
		sortBy = "created_at"
	}
	if !validOrders[order] {
		order = "desc"
	}
	return sortBy + " " + order
}

// GetMyContentList 获取我的内容列表
// @Summary      我的内容
// @Description  获取当前登录用户的内容列表
// @Tags         内容管理
// @Produce      json
// @Security     SessionAuth
// @Param        page         query int    false "页码" default(1)
// @Param        page_size    query int    false "每页数量" default(20) maximum(100)
// @Param        type         query string false "内容类型" Enums(video, image, text, link)
// @Param        audit_status query string false "审核状态" Enums(approved, pending, rejected)
// @Param        big_tag_id   query int    false "大标签ID"
// @Param        small_tag_id query int    false "小标签ID"
// @Success      200 {object} utils.SwaggerResponse{data=object{list=[]models.Content,total=int,page=int,page_size=int,total_page=int}}
// @Failure      401 {object} utils.SwaggerResponse
// @Router       /content/my [get]
func parsePagination(c *gin.Context) (page, pageSize, offset int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset = (page - 1) * pageSize
	return
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
		query = query.Where(sqlAuditStatus, auditStatus)
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

	page, pageSize, offset := parsePagination(c)

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

	summaryList := make([]gin.H, 0, len(contents))
	for _, content := range contents {
		summaryList = append(summaryList, buildContentSummary(c, content))
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data": gin.H{
			"list":       summaryList,
			"total":      total,
			"page":       page,
			"page_size":  pageSize,
			"total_page": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

// GetContent 获取内容详情
// @Summary      内容详情
// @Description  获取单个内容的详细信息，含用户信息和封面/视频URL
// @Tags         内容管理
// @Produce      json
// @Param        id path int true "内容ID"
// @Success      200 {object} utils.SwaggerResponse{data=models.Content}
// @Failure      404 {object} utils.SwaggerResponse
// @Router       /content/{id} [get]
func GetContent(c *gin.Context) {
	id := c.Param("id")
	var content models.Content

	cacheKey := "content:" + id
	if utils.RedisClient != nil {
		if err := utils.GetCacheJSON(cacheKey, &content); err == nil {
			go func() {
				incrementViewCount(content.ID)
			}()
			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "获取成功",
				"data":    buildContentDetail(c, content),
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

	go func() {
		incrementViewCount(content.ID)
	}()

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    buildContentDetail(c, content),
	})
}

func incrementViewCount(contentID uint) {
	now := time.Now()
	dateStr := now.Format("2006-01-02")

	// 更新 MySQL 总浏览量
	var content models.Content
	utils.DB.First(&content, contentID)
	utils.DB.Model(&content).Update("view_count", gorm.Expr("view_count + 1"))

	// 更新 Redis 按天记录的浏览量
	if utils.RedisClient != nil {
		utils.IncrementViewCountByDate(contentID, dateStr)
	}
}

// UpdateContent 更新内容
// @Summary      更新内容
// @Description  更新内容信息。作者或管理员可操作，更新后重新进入审核。link类型修改url若为视频链接会自动重新抓取封面和标题。
// @Tags         内容管理
// @Accept       multipart/form-data
// @Produce      json
// @Security     SessionAuth
// @Param        id      path     int      true  "内容ID"
// @Param        title   formData string   false "新标题"
// @Param        content formData string   false "新文字内容"
// @Param        url     formData string   false "新链接"
// @Param        tags    formData []string false "新标签列表"
// @Param        file    formData file     false "新文件"
// @Success      200 {object} utils.SwaggerResponse{data=models.Content}
// @Failure      400 {object} utils.SwaggerResponse
// @Failure      401 {object} utils.SwaggerResponse
// @Failure      403 {object} utils.SwaggerResponse
// @Failure      404 {object} utils.SwaggerResponse
// @Router       /content/{id} [put]
func UpdateContent(c *gin.Context) {
	id := c.Param("id")
	var content models.Content

	if err := utils.DB.First(&content, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "内容不存在", "data": nil})
		return
	}

	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录", "data": nil})
		return
	}
	currentUser := user.(models.User)
	if !currentUser.IsAdmin && content.UserID != currentUser.ID {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "无权修改此内容", "data": nil})
		return
	}

	processUpdateContentForm(c, &content)

	if content.Type == models.ContentTypeVideo || content.Type == models.ContentTypeImage {
		if fh, err := c.FormFile("file"); err == nil {
			if he := processUpdateContentFile(c, &content, fh); he != nil {
				c.JSON(he.code, gin.H{"code": he.code, "message": he.message, "data": nil})
				return
			}
		}
	}

	content.AuditStatus = models.AuditStatusPending

	if err := utils.DB.Save(&content).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "更新失败", "data": nil})
		return
	}

	go func() {
		ClearContentListCache()
		ClearRecommendCache()
		utils.ClearContentCache(content.ID)
	}()

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "更新成功", "data": buildContentDetail(c, content)})
}

func processUpdateContentForm(c *gin.Context, content *models.Content) {
	title := c.PostForm("title")
	newText := c.PostForm("content")
	newUrl := c.PostForm("url")
	tags := c.PostFormArray("tags")

	if title != "" && utils.ValidateContentTitle(title) {
		content.Title = utils.SanitizeHTML(title)
	}

	if content.Type == models.ContentTypeText && newText != "" && utils.ValidateTextContent(newText) {
		content.Content = utils.SanitizeHTML(newText)
	}

	if content.Type == models.ContentTypeLink && newUrl != "" && newUrl != content.Url {
		content.Url = newUrl
		updateContentLinkInfo(content, newUrl, title)
	}

	if len(tags) > 0 {
		content.Tags = deduplicateTags(tags)
	}
}

func updateContentLinkInfo(content *models.Content, newUrl string, title string) {
	platform, sourceID, err := services.DetectPlatform(newUrl)
	if err != nil {
		return
	}
	videoInfo, err := services.FetchVideoInfo(platform, sourceID)
	if err != nil {
		log.Printf("Warning: failed to fetch video info: %v", err)
		return
	}
	content.Platform = string(videoInfo.Platform)
	if title == "" {
		content.Title = utils.SanitizeHTML(videoInfo.Title)
	}
	if videoInfo.CoverURL != "" || videoInfo.Platform == services.PlatformDouyin {
		if content.ThumbPath != "" {
			oldThumbPath := filepath.Join(config.AppConfig.Server.ThumbnailDir, content.ThumbPath)
			os.Remove(oldThumbPath)
		}
		coverFilename, err := services.DownloadCover(videoInfo.CoverURL, content.UserID, videoInfo.Platform == services.PlatformDouyin)
		if err != nil {
			log.Printf("Warning: failed to download cover: %v", err)
		} else {
			content.ThumbPath = coverFilename
		}
	}
}

func deduplicateTags(tags []string) []string {
	uniqueTags := make([]string, 0)
	tagSet := make(map[string]bool)
	for _, tag := range tags {
		if tag != "" && !tagSet[tag] {
			tagSet[tag] = true
			uniqueTags = append(uniqueTags, tag)
		}
	}
	return uniqueTags
}

func processUpdateContentFile(c *gin.Context, content *models.Content, file *multipart.FileHeader) *httpError {
	cleanupOldContentFiles(content)

	allowedExts, maxSize := getFileUploadConfig(content.Type)
	uploadService := services.NewFileUploadService(&services.FileUploadConfig{
		AllowedExtensions: allowedExts,
		MaxSize:           maxSize,
		UserID:            content.UserID,
	})

	result, err := uploadService.Upload(file)
	if err != nil {
		return &httpError{code: http.StatusBadRequest, message: err.Error()}
	}

	content.FilePath = result.Filename
	content.FileSize = result.FileSize
	generateContentThumb(content, result.FilePath, result.Filename)
	return nil
}

func cleanupOldContentFiles(content *models.Content) {
	if content.FilePath != "" {
		deleteService := services.NewFileUploadService(&services.FileUploadConfig{
			UserID: content.UserID,
		})
		deleteService.DeleteFile(content.FilePath)
	}

	if content.ThumbPath != "" {
		os.Remove(filepath.Join(config.AppConfig.Server.ThumbnailDir, content.ThumbPath))
	}

	if content.CompressedPath != "" {
		compressedPath := filepath.Join(config.AppConfig.Server.UploadDir, "..", "images", content.CompressedPath)
		absPath, _ := filepath.Abs(compressedPath)
		os.Remove(absPath)
		content.CompressedPath = ""
		content.ThumbPath = ""
	}
}

func getFileUploadConfig(ct models.ContentType) ([]string, int64) {
	if ct == models.ContentTypeImage {
		return []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}, 10 * 1024 * 1024
	}
	return []string{".mp4", ".avi", ".mov", ".mkv"}, 500 * 1024 * 1024
}

func generateContentThumb(content *models.Content, filePath, filename string) {
	if content.Type == models.ContentTypeVideo {
		if tn, err := utils.GenerateVideoThumbnail(filePath, filename); err == nil {
			content.ThumbPath = tn
		} else {
			log.Printf("Warning: failed to generate video thumbnail: %v", err)
		}
	} else if content.Type == models.ContentTypeImage {
		if tn, err := utils.GenerateImageThumbnail(filePath, filename); err == nil {
			content.ThumbPath = tn
		} else {
			log.Printf("Warning: failed to generate image thumbnail: %v", err)
		}
	}
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
		oldThumbPath := filepath.Join(config.AppConfig.Server.ThumbnailDir, content.ThumbPath)
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

// DeleteContent 删除内容
// @Summary      删除内容
// @Description  删除内容及关联文件和评论。作者或管理员可操作。
// @Tags         内容管理
// @Produce      json
// @Security     SessionAuth
// @Param        id path int true "内容ID"
// @Success      200 {object} utils.SwaggerResponse
// @Failure      401 {object} utils.SwaggerResponse
// @Failure      403 {object} utils.SwaggerResponse
// @Failure      404 {object} utils.SwaggerResponse
// @Router       /content/{id} [delete]
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

	utils.DB.Where(sqlContentID, content.ID).Delete(&models.AuditLog{})
	utils.DB.Where(sqlContentID, content.ID).Delete(&models.Comment{})

	if err := utils.DB.Unscoped().Delete(&content).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除失败",
			"data":    nil,
		})
		return
	}

	ClearContentListCache()

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "删除成功",
		"data":    nil,
	})
}

// SearchContent 搜索内容
// @Summary      搜索内容
// @Description  按关键词搜索已审核通过的内容（模糊匹配标题和内容字段）
// @Tags         内容管理
// @Produce      json
// @Param        keyword   query string true  "搜索关键词"
// @Param        page      query int    false "页码" default(1)
// @Param        page_size query int    false "每页数量" default(20) maximum(100)
// @Success      200 {object} utils.SwaggerResponse{data=object{list=[]models.Content,total=int,page=int,page_size=int,total_page=int}}
// @Failure      400 {object} utils.SwaggerResponse
// @Router       /content/search [get]
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

	query := utils.DB.Model(&models.Content{}).Where(sqlAuditStatus, models.AuditStatusApproved).
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
		Order(sqlOrderCreated).Limit(pageSize).Offset(offset)

	if err := query.Find(&contents).Error; err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "搜索失败")
		return
	}

	results := make([]gin.H, 0, len(contents))
	for _, content := range contents {
		result := buildContentSummary(c, content)
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

// RecommendContent 推荐内容
// @Summary      推荐内容
// @Description  获取推荐内容列表（基于Redis热度排序，降级为时间倒序）
// @Tags         内容管理
// @Produce      json
// @Param        count query int false "返回数量（1-50）" default(10)
// @Param        page  query int false "页码" default(1)
// @Success      200 {object} utils.SwaggerResponse{data=object{list=[]models.Content,count=int}}
// @Router       /content/recommend [get]
func RecommendContent(c *gin.Context) {
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

	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	start := int64((page - 1) * count)
	end := start + int64(count) - 1

	var contentIDs []string
	var zsetAvailable bool
	if utils.RedisClient != nil {
		contentIDs, err = utils.ZRevRangeRecommend(start, end)
		if err == nil && len(contentIDs) > 0 {
			zsetAvailable = true
		}
	}

	var result []models.Content
	if !zsetAvailable {
		result, err = getRecommendFromDB(start, count)
	} else {
		result, err = getRecommendFromZSet(contentIDs)
	}
	if err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "获取推荐失败")
		return
	}

	results := make([]gin.H, 0, len(result))
	for _, content := range result {
		results = append(results, buildContentResponse(c, content))
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

func getRecommendFromDB(start int64, count int) ([]models.Content, error) {
	var contents []models.Content
	query := utils.DB.Model(&models.Content{}).
		Where(sqlAuditStatus, models.AuditStatusApproved).
		Order(sqlOrderCreated).
		Limit(count).
		Offset(int(start))
	if err := query.Preload("User").Find(&contents).Error; err != nil {
		return nil, err
	}
	return contents, nil
}

func getRecommendFromZSet(contentIDs []string) ([]models.Content, error) {
	var ids []uint
	for _, idStr := range contentIDs {
		var id uint
		fmt.Sscanf(idStr, "%d", &id)
		ids = append(ids, id)
	}

	var result []models.Content
	if err := utils.DB.Preload("User").Where("id IN ?", ids).Find(&result).Error; err != nil {
		return nil, err
	}

	idMap := make(map[uint]models.Content)
	for _, content := range result {
		idMap[content.ID] = content
	}

	var orderedResult []models.Content
	for _, id := range ids {
		if content, ok := idMap[id]; ok {
			orderedResult = append(orderedResult, content)
		}
	}
	return orderedResult, nil
}

// GetAllTags 获取所有标签
// @Summary      所有标签
// @Description  获取系统中所有已使用的标签
// @Tags         内容管理
// @Produce      json
// @Success      200 {object} utils.SwaggerResponse{data=[]string}
// @Router       /content/tags [get]
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

func ClearRecommendCache() {
	if utils.RedisClient == nil {
		return
	}
	utils.ZClearRecommend()
}

// UpdateContentAuthor 更新内容作者
// @Summary      更新内容作者
// @Description  管理员将内容转移给其他用户
// @Tags         管理
// @Accept       json
// @Produce      json
// @Security     SessionAuth
// @Param        id   path int  true "内容ID"
// @Param        body body object{user_id=uint} true "目标用户ID"
// @Success      200 {object} utils.SwaggerResponse
// @Failure      400 {object} utils.SwaggerResponse
// @Failure      404 {object} utils.SwaggerResponse
// @Router       /admin/content/{id}/author [put]
func UpdateContentAuthor(c *gin.Context) {
	contentID := c.Param("id")
	var content models.Content

	if err := utils.DB.First(&content, contentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "内容不存在",
			"data":    nil,
		})
		return
	}

	var req struct {
		UserID uint `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请提供有效的用户ID",
			"data":    nil,
		})
		return
	}

	var user models.User
	if err := utils.DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "目标用户不存在",
			"data":    nil,
		})
		return
	}

	oldUserID := content.UserID
	content.UserID = req.UserID

	if err := utils.DB.Save(&content).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新失败",
			"data":    nil,
		})
		return
	}

	go func() {
		ClearContentListCache()
		ClearRecommendCache()
		utils.ClearContentCache(content.ID)
	}()

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "更新成功",
		"data": gin.H{
			"content_id":   content.ID,
			"old_user_id":  oldUserID,
			"new_user_id":  req.UserID,
			"new_username": user.Username,
		},
	})
}

func sanitizeSearchInput(input string) string {
	input = strings.TrimSpace(input)
	input = strings.ReplaceAll(input, "%", "\\%")
	input = strings.ReplaceAll(input, "_", "\\_")
	return input
}

func setTypeFields(result gin.H, content models.Content) {
	switch content.Type {
	case models.ContentTypeVideo:
		if content.FilePath != "" {
			result["video"] = "/uploads/" + content.FilePath
			result["file_size"] = content.FileSize
		}
		if content.ThumbPath != "" {
			result["thumb"] = thumbnailPath + content.ThumbPath
		}
	case models.ContentTypeImage:
		if content.FilePath != "" {
			if content.CompressedPath != "" {
				result["img"] = "/images/" + content.CompressedPath
			} else {
				result["img"] = "/uploads/" + content.FilePath
			}
			result["file_size"] = content.FileSize
		}
		if content.ThumbPath != "" {
			result["thumb"] = thumbnailPath + content.ThumbPath
		}
	case models.ContentTypeText:
		result["text"] = content.Content
	case models.ContentTypeLink:
		result["url"] = content.Url
		if content.Platform != "" {
			result["platform"] = content.Platform
		}
		if content.ThumbPath != "" {
			result["thumb"] = thumbnailPath + content.ThumbPath
		}
	}
}

func buildContentResponse(c *gin.Context, content models.Content) gin.H {
	return buildContentDetail(c, content)
}

func buildContentDetail(c *gin.Context, content models.Content) gin.H {
	result := gin.H{
		"id":           content.ID,
		"title":        content.Title,
		"type":         content.Type,
		"view_count":   content.ViewCount,
		"audit_status": content.AuditStatus,
		"tags":         safeTags(content.Tags),
		"created_at":   content.CreatedAt.Unix(),
		"updated_at":   content.UpdatedAt.Unix(),
	}

	setTypeFields(result, content)

	if content.User.ID > 0 {
		result["user"] = gin.H{
			"id":       content.User.ID,
			"username": content.User.Username,
			"is_admin": content.User.IsAdmin,
		}
	}

	return result
}

func buildContentSummary(c *gin.Context, content models.Content) gin.H {
	result := gin.H{
		"id":         content.ID,
		"title":      content.Title,
		"type":       content.Type,
		"view_count": content.ViewCount,
		"tags":       safeTags(content.Tags),
		"created_at": content.CreatedAt.Unix(),
	}

	switch content.Type {
	case models.ContentTypeVideo, models.ContentTypeImage:
		if content.ThumbPath != "" {
			result["thumb"] = thumbnailPath + content.ThumbPath
		}
	case models.ContentTypeLink:
		result["url"] = content.Url
		if content.ThumbPath != "" {
			result["thumb"] = thumbnailPath + content.ThumbPath
		}
	}

	if content.User.ID > 0 {
		result["user"] = gin.H{
			"id":       content.User.ID,
			"username": content.User.Username,
			"is_admin": content.User.IsAdmin,
		}
	}

	return result
}

func safeTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
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

// CleanOrphanedFiles 清理孤立文件
// @Summary      清理孤立文件
// @Description  管理员清理未被任何内容引用的上传文件和缩略图
// @Tags         管理
// @Produce      json
// @Security     SessionAuth
// @Success      200 {object} utils.SwaggerResponse{data=object{deleted_count=int,deleted_files=[]string}}
// @Failure      500 {object} utils.SwaggerResponse
// @Router       /admin/files/clean [delete]
func CleanOrphanedFiles(c *gin.Context) {
	var allContents []models.Content
	if err := utils.DB.Unscoped().Select("file_path", "thumb_path", "compressed_path").Find(&allContents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取文件列表失败",
			"data":    nil,
		})
		return
	}

	recordedFiles := collectRecordedFiles(allContents)

	uploadDir := config.AppConfig.Server.UploadDir
	thumbDir := config.AppConfig.Server.ThumbnailDir

	deletedCount := 0
	var deletedFiles []string

	c1, f1 := cleanDirectory(uploadDir, recordedFiles)
	deletedCount += c1
	deletedFiles = append(deletedFiles, f1...)

	c2, f2 := cleanDirectory(thumbDir, recordedFiles)
	deletedCount += c2
	deletedFiles = append(deletedFiles, f2...)

	c3, f3 := cleanDirectory(filepath.Join(uploadDir, "..", "images"), recordedFiles)
	deletedCount += c3
	deletedFiles = append(deletedFiles, f3...)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "清理完成",
		"data": gin.H{
			"deleted_count": deletedCount,
			"deleted_files": deletedFiles,
		},
	})
}

func collectRecordedFiles(allContents []models.Content) map[string]bool {
	recordedFiles := make(map[string]bool)
	for _, content := range allContents {
		if content.FilePath != "" {
			recordedFiles[content.FilePath] = true
		}
		if content.ThumbPath != "" {
			recordedFiles[content.ThumbPath] = true
		}
		if content.CompressedPath != "" {
			recordedFiles[content.CompressedPath] = true
		}
	}
	return recordedFiles
}

func cleanDirectory(dir string, recordedFiles map[string]bool) (deletedCount int, deletedFiles []string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if !recordedFiles[filename] {
			filePath := filepath.Join(dir, filename)
			if err := os.Remove(filePath); err == nil {
				deletedCount++
				deletedFiles = append(deletedFiles, filename)
			} else {
				log.Printf("Failed to delete orphaned file: %s, error: %v", filePath, err)
			}
		}
	}
	return
}

func PurgeDeletedContent(c *gin.Context) {
	result := utils.DB.Unscoped().Where("deleted_at IS NOT NULL").Delete(&models.Content{})

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "清理完成",
		"data": gin.H{
			"deleted_count": result.RowsAffected,
		},
	})
}

// CreateClaim 认领内容
// @Summary      内容认领
// @Description  提交内容作者认领申请（不能认领自己的内容）
// @Tags         内容管理
// @Accept       json
// @Produce      json
// @Security     SessionAuth
// @Param        content_id path string true "内容ID"
// @Param        body body object{reason=string} true "认领理由"
// @Success      200 {object} utils.SwaggerResponse{data=models.Claim}
// @Failure      400 {object} utils.SwaggerResponse
// @Failure      401 {object} utils.SwaggerResponse
// @Failure      404 {object} utils.SwaggerResponse
// @Failure      409 {object} utils.SwaggerResponse
// @Router       /content/{content_id}/claim [post]
func CreateClaim(c *gin.Context) {
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

	contentID := c.Param("content_id")
	var content models.Content
	if err := utils.DB.First(&content, contentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "内容不存在",
			"data":    nil,
		})
		return
	}

	if content.UserID == currentUser.ID {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "不能认领自己的内容",
			"data":    nil,
		})
		return
	}

	var existingClaim models.Claim
	if err := utils.DB.Where("content_id = ? AND user_id = ? AND status = ?", content.ID, currentUser.ID, models.ClaimStatusPending).First(&existingClaim).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": "您已提交过该内容的认领申请",
			"data":    nil,
		})
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请提供认领理由",
			"data":    nil,
		})
		return
	}

	claim := &models.Claim{
		ContentID: content.ID,
		UserID:    currentUser.ID,
		Reason:    req.Reason,
		Status:    models.ClaimStatusPending,
	}

	if err := utils.DB.Create(claim).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "提交失败",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "提交成功，等待管理员审核",
		"data": gin.H{
			"id":         claim.ID,
			"content_id": claim.ContentID,
			"reason":     claim.Reason,
			"status":     claim.Status,
			"created_at": claim.CreatedAt.Unix(),
		},
	})
}

// GetClaimList 认领列表
// @Summary      认领申请列表
// @Description  管理员查看内容认领申请列表
// @Tags         管理
// @Produce      json
// @Security     SessionAuth
// @Param        status     query string false "状态筛选" Enums(pending, approved, rejected)
// @Param        content_id query int    false "内容ID筛选"
// @Param        page       query int    false "页码" default(1)
// @Param        page_size  query int    false "每页数量" default(20)
// @Success      200 {object} utils.SwaggerResponse{data=object{list=[]models.Claim,total=int}}
// @Router       /admin/claims [get]
func GetClaimList(c *gin.Context) {
	var claims []models.Claim
	query := utils.DB.Model(&models.Claim{}).Preload("Content").Preload("User")

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if contentID := c.Query("content_id"); contentID != "" {
		query = query.Where(sqlContentID, contentID)
	}

	var total int64
	query.Count(&total)

	page, pageSize, offset := parsePaginationParams(c)

	query = query.Order(sqlOrderCreated).Limit(pageSize).Offset(offset)
	if err := query.Find(&claims).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取失败",
			"data":    nil,
		})
		return
	}

	claimList := make([]gin.H, 0, len(claims))
	for _, claim := range claims {
		item := gin.H{
			"id":         claim.ID,
			"reason":     claim.Reason,
			"status":     claim.Status,
			"user": gin.H{
				"id":       claim.User.ID,
				"username": claim.User.Username,
			},
			"created_at": claim.CreatedAt.Unix(),
		}
		if claim.Content.ID > 0 {
			item["content"] = buildContentSummary(c, claim.Content)
		}
		claimList = append(claimList, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data": gin.H{
			"list":       claimList,
			"total":      total,
			"page":       page,
			"page_size":  pageSize,
			"total_page": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

// HandleClaim 处理认领
// @Summary      处理认领申请
// @Description  管理员通过或拒绝内容认领申请
// @Tags         管理
// @Accept       json
// @Produce      json
// @Security     SessionAuth
// @Param        id   path int true "认领申请ID"
// @Param        body body object{action=string,remark=string} true "操作(approve/reject)和备注"
// @Success      200 {object} utils.SwaggerResponse{data=models.Claim}
// @Failure      400 {object} utils.SwaggerResponse
// @Failure      404 {object} utils.SwaggerResponse
// @Router       /admin/claims/{id}/handle [post]
func HandleClaim(c *gin.Context) {
	claimID := c.Param("id")
	var claim models.Claim
	if err := utils.DB.Preload("Content").First(&claim, claimID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "认领申请不存在",
			"data":    nil,
		})
		return
	}

	if claim.Status != models.ClaimStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "该申请已处理",
			"data":    nil,
		})
		return
	}

	admin, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
			"data":    nil,
		})
		return
	}
	currentAdmin := admin.(models.User)

	var req struct {
		Action string `json:"action" binding:"required"`
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请提供操作类型",
			"data":    nil,
		})
		return
	}

	if req.Action == "approve" {
		claim.Status = models.ClaimStatusApproved
		claim.ApprovedBy = &currentAdmin.ID
		claim.Remark = req.Remark

		utils.DB.Model(&models.Content{}).Where("id = ?", claim.ContentID).Update("user_id", claim.UserID)

		utils.DB.Model(&models.Claim{}).Where("content_id = ? AND id != ?", claim.ContentID, claim.ID).Update("status", models.ClaimStatusRejected)

	} else if req.Action == "reject" {
		claim.Status = models.ClaimStatusRejected
		claim.ApprovedBy = &currentAdmin.ID
		claim.Remark = req.Remark
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的操作类型",
			"data":    nil,
		})
		return
	}

	if err := utils.DB.Save(&claim).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "处理失败",
			"data":    nil,
		})
		return
	}

	go func() {
		ClearContentListCache()
		utils.ClearContentCache(claim.ContentID)
	}()

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "处理成功",
		"data": gin.H{
			"id":         claim.ID,
			"status":     claim.Status,
			"remark":     claim.Remark,
			"updated_at": claim.UpdatedAt.Unix(),
		},
	})
}
