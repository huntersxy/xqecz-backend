
package handlers

import (
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

	title := c.GetHeader("X-Bot-Title")
	if title == "" {
		title = "Bot Upload"
	} else {
		if decoded, err := url.QueryUnescape(title); err == nil {
			title = decoded
		}
	}

	extraTagsStr := c.GetHeader("X-Bot-Tags")
	var extraTags []string
	if extraTagsStr != "" {
		if decoded, err := url.QueryUnescape(extraTagsStr); err == nil {
			extraTagsStr = decoded
		}
		for _, tag := range strings.Split(extraTagsStr, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				extraTags = append(extraTags, tag)
			}
		}
	}

	linkUrl := c.GetHeader("X-Bot-Url")
	var contentType models.ContentType
	var content *models.Content
	tags := []string{"bot"}
	tags = append(tags, extraTags...)

	if linkUrl != "" {
		if decoded, err := url.QueryUnescape(linkUrl); err == nil {
			linkUrl = decoded
		}
		contentType = models.ContentTypeLink
		content = &models.Content{
			Title:       title,
			Type:        contentType,
			Url:         linkUrl,
			UserID:      1,
			AuditStatus: models.AuditStatusApproved,
			Tags:        tags,
		}
		platform, sourceID, err := services.DetectPlatform(linkUrl)
		if err == nil {
			videoInfo, err := services.FetchVideoInfo(platform, sourceID)
			if err == nil {
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
		}
	} else {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "请上传文件或提供链接",
				"data":    nil,
			})
			return
		}

		ext := strings.ToLower(filepath.Ext(file.Filename))
		var allowedExtensions []string
		var maxSize int64

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
			return
		}

		if file.Size > maxSize {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "文件大小超出限制",
				"data":    nil,
			})
			return
		}

		uploadService := services.NewFileUploadService(&services.FileUploadConfig{
			AllowedExtensions: allowedExtensions,
			MaxSize:           maxSize,
			UserID:            1,
		})

		result, err := uploadService.Upload(file)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": err.Error(),
				"data":    nil,
			})
			return
		}

		content = &models.Content{
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
	}

	if err := utils.DB.Create(content).Error; err != nil {
		log.Printf("[Bot] 创建内容失败: %v", err)
		utils.RespondWithError(c, http.StatusInternalServerError, "创建内容失败")
		return
	}

	go func() {
		ClearContentListCache()
	}()

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

	utils.RespondWithSuccess(c, responseData)
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
