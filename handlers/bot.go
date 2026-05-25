package handlers

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"xiaoquan-backend/config"
	"xiaoquan-backend/models"
	"xiaoquan-backend/services"
	"xiaoquan-backend/utils"

	"github.com/gin-gonic/gin"
)

// BotUpload Bot上传
// @Summary      Bot上传
// @Description  Bot通过Token认证上传内容（自动审核通过）
// @Tags         Bot
// @Accept       multipart/form-data
// @Produce      json
// @Param        X-Bot-Token header   string false "Bot Token"
// @Param        X-Bot-Title header   string false "标题"
// @Param        X-Bot-Url   header   string false "链接URL"
// @Param        X-Bot-Tags  header   string false "标签（逗号分隔）"
// @Param        file        formData file   false "文件"
// @Success      200 {object} utils.SwaggerResponse{data=object{id=uint,type=string}}
// @Failure      400 {object} utils.SwaggerResponse
// @Failure      401 {object} utils.SwaggerResponse
// @Router       /bot/upload [post]
func BotUpload(c *gin.Context) {
	botToken := c.GetHeader("X-Bot-Token")
	if botToken == "" {
		botToken = c.Query("token")
	}
	if !validateBotToken(botToken) {
		utils.RespondWithError(c, http.StatusUnauthorized, "无效的Bot Token")
		return
	}

	title := parseBotTitle(c)
	extraTags := parseBotTags(c)
	tags := append([]string{"bot"}, extraTags...)

	linkUrl := c.GetHeader("X-Bot-Url")
	var content *models.Content
	var err error

	if linkUrl != "" {
		content, err = handleLinkUpload(title, tags, c)
	} else {
		content, err = handleFileUpload(title, tags, c)
	}
	if err != nil {
		return
	}

	if err := utils.DB.Create(content).Error; err != nil {
		log.Printf("[Bot] 创建内容失败: %v", err)
		utils.RespondWithError(c, http.StatusInternalServerError, "创建内容失败")
		return
	}

	go func() {
		ClearContentListCache()
	}()

	utils.RespondWithSuccess(c, buildBotResponse(content))
}

var errBotAbort = errors.New("bot upload aborted")

func parseBotTitle(c *gin.Context) string {
	title := c.GetHeader("X-Bot-Title")
	if title == "" {
		return "Bot Upload"
	}
	if decoded, err := url.QueryUnescape(title); err == nil {
		return decoded
	}
	return title
}

func parseBotTags(c *gin.Context) []string {
	extraTagsStr := c.GetHeader("X-Bot-Tags")
	if extraTagsStr == "" {
		return nil
	}
	if decoded, err := url.QueryUnescape(extraTagsStr); err == nil {
		extraTagsStr = decoded
	}
	var extraTags []string
	for _, tag := range strings.Split(extraTagsStr, ",") {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			extraTags = append(extraTags, tag)
		}
	}
	return extraTags
}

func handleLinkUpload(title string, tags []string, c *gin.Context) (*models.Content, error) {
	linkUrl := c.GetHeader("X-Bot-Url")
	if decoded, err := url.QueryUnescape(linkUrl); err == nil {
		linkUrl = decoded
	}
	content := &models.Content{
		Title:       title,
		Type:        models.ContentTypeLink,
		Url:         linkUrl,
		UserID:      1,
		AuditStatus: models.AuditStatusApproved,
		Tags:        tags,
	}
	enrichLinkContent(content, linkUrl, title)
	return content, nil
}

func enrichLinkContent(content *models.Content, linkUrl string, title string) {
	platform, sourceID, err := services.DetectPlatform(linkUrl)
	if err != nil {
		return
	}
	videoInfo, err := services.FetchVideoInfo(platform, sourceID)
	if err != nil {
		return
	}
	content.Platform = string(videoInfo.Platform)
	if title == "" || title == "Bot Upload" {
		content.Title = videoInfo.Title
	}
	if videoInfo.CoverURL != "" || videoInfo.Platform == services.PlatformDouyin {
		coverFilename, err := services.DownloadCover(videoInfo.CoverURL, 1, videoInfo.Platform == services.PlatformDouyin)
		if err != nil {
			log.Printf("Warning: failed to download cover for bot link: %v", err)
		} else {
			content.ThumbPath = coverFilename
		}
	}
}

func handleFileUpload(title string, tags []string, c *gin.Context) (*models.Content, error) {
	file, err := c.FormFile("file")
	if err != nil {
		utils.RespondWithError(c, http.StatusBadRequest, "请上传文件或提供链接")
		return nil, errBotAbort
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	var allowedExtensions []string
	var maxSize int64
	var contentType models.ContentType

	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		contentType = models.ContentTypeImage
		allowedExtensions = []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
		maxSize = 10 * 1024 * 1024
	case ".mp4", ".avi", ".mov", ".mkv":
		contentType = models.ContentTypeVideo
		allowedExtensions = []string{".mp4", ".avi", ".mov", ".mkv"}
		maxSize = 500 * 1024 * 1024
	default:
		utils.RespondWithError(c, http.StatusBadRequest, "不支持的文件类型，仅支持图片和视频")
		return nil, errBotAbort
	}

	if file.Size > maxSize {
		utils.RespondWithError(c, http.StatusBadRequest, "文件大小超出限制")
		return nil, errBotAbort
	}

	uploadService := services.NewFileUploadService(&services.FileUploadConfig{
		AllowedExtensions: allowedExtensions,
		MaxSize:           maxSize,
		UserID:            1,
	})

	result, err := uploadService.Upload(file)
	if err != nil {
		utils.RespondWithError(c, http.StatusBadRequest, err.Error())
		return nil, errBotAbort
	}

	content := &models.Content{
		Title:       title,
		Type:        contentType,
		FilePath:    result.Filename,
		FileSize:    result.FileSize,
		UserID:      1,
		AuditStatus: models.AuditStatusApproved,
		Tags:        tags,
	}

	if contentType == models.ContentTypeVideo {
		thumbFilename, err := utils.GenerateVideoThumbnail(result.FilePath, result.Filename)
		if err != nil {
			log.Printf("Warning: failed to generate video thumbnail: %v", err)
		} else {
			content.ThumbPath = thumbFilename
		}
	} else {
		thumbFilename, err := utils.GenerateImageThumbnail(result.FilePath, result.Filename)
		if err != nil {
			log.Printf("Warning: failed to generate image thumbnail: %v", err)
		} else {
			content.ThumbPath = thumbFilename
		}
	}

	return content, nil
}

func buildBotResponse(content *models.Content) gin.H {
	responseData := gin.H{
		"id":   content.ID,
		"type": content.Type,
	}
	if content.ThumbPath != "" {
		responseData["thumb_path"] = content.ThumbPath
	}
	if content.Url != "" {
		responseData["url"] = content.Url
	}
	return responseData
}

func validateBotToken(token string) bool {
	if token == "" {
		return false
	}
	expectedToken := config.AppConfig.Bot.Token
	if expectedToken == "" {
		return false
	}
	return token == expectedToken
}
