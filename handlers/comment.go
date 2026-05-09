package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"xiaoquan-backend/config"
	"xiaoquan-backend/models"
	"xiaoquan-backend/utils"

	"github.com/gin-gonic/gin"
)

func AddComment(c *gin.Context) {
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

	contentIDStr := c.PostForm("content_id")
	contentID, err := strconv.ParseUint(contentIDStr, 10, 32)
	if err != nil || contentID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的内容ID",
			"data":    nil,
		})
		return
	}

	var content models.Content
	if err := utils.DB.Where("id = ? AND audit_status = ?", contentID, models.AuditStatusApproved).First(&content).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "内容不存在或未通过审核",
			"data":    nil,
		})
		return
	}

	text := c.PostForm("text")
	if text == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "评论内容不能为空",
			"data":    nil,
		})
		return
	}

	if len(text) > 5000 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "评论内容过长（最大5000字符）",
			"data":    nil,
		})
		return
	}

	if checkBannedWords(text) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "内容包含违规词汇",
			"data":    nil,
		})
		return
	}

	parentIDStr := c.PostForm("parent_id")
	var parentID *uint
	if parentIDStr != "" {
		if pid, err := strconv.ParseUint(parentIDStr, 10, 32); err == nil && pid > 0 {
			pidUint := uint(pid)
			parentID = &pidUint
		}
	}

	comment := &models.Comment{
		ContentID: uint(contentID),
		UserID:    currentUser.ID,
		Text:      text,
		ParentID:  parentID,
	}

	if err := utils.DB.Create(comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "添加评论失败",
			"data":    nil,
		})
		return
	}

	go asyncCheckSpamAPI(comment.ID, text)

	if utils.RedisClient != nil {
		utils.ClearCommentCache(uint(contentID))
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "评论成功",
		"data":    comment,
	})
}

func checkBannedWords(text string) bool {
	var bannedWords []string

	if utils.RedisClient != nil {
		if err := utils.GetCacheJSON("banned_words", &bannedWords); err == nil && len(bannedWords) > 0 {
			return checkWordsInList(text, bannedWords)
		}
	}

	return checkBannedWordsFromFile(text)
}

func checkBannedWordsFromFile(text string) bool {
	configDir := "./config"
	bannedWordsFile := filepath.Join(configDir, "banned_words.json")

	if err := ensureBannedWordsFile(bannedWordsFile); err != nil {
		return false
	}

	file, err := os.Open(bannedWordsFile)
	if err != nil {
		return false
	}
	defer file.Close()

	var data struct {
		Words []string `json:"words"`
	}

	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return false
	}

	if utils.RedisClient != nil {
		utils.SetCacheJSON("banned_words", data.Words, 24*time.Hour)
	}

	return checkWordsInList(text, data.Words)
}

func ensureBannedWordsFile(filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("[敏感词] 敏感词文件不存在，正在创建: %s", filePath)

		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		defaultBannedWords := map[string][]string{
			"words": {
				"admin", "test", "傻逼", "sb", "fuck",
			},
		}

		data, err := json.MarshalIndent(defaultBannedWords, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal default banned words: %w", err)
		}

		if err := os.WriteFile(filePath, data, 0644); err != nil {
			return fmt.Errorf("failed to write default banned words file: %w", err)
		}

		log.Printf("[敏感词] 默认敏感词文件已创建: %s", filePath)
	}
	return nil
}

func checkWordsInList(text string, words []string) bool {
	textLower := strings.ToLower(text)
	for _, word := range words {
		if strings.Contains(textLower, strings.ToLower(word)) {
			return true
		}
	}
	return false
}

func asyncCheckSpamAPI(commentID uint, text string) {
	spamAPIURL := config.AppConfig.SpamAPI.URL
	if spamAPIURL == "" {
		return
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	requestData := map[string]string{"text": text}
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return
	}

	resp, err := client.Post(spamAPIURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var result struct {
		Code int `json:"code"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	if result.Code == 966 {
		utils.DB.Model(&models.Comment{}).Where("id = ?", commentID).Update("is_banned", true)
	}
}

func GetComments(c *gin.Context) {
	contentIDStr := c.Param("content_id")
	contentID, err := strconv.ParseUint(contentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的内容ID",
			"data":    nil,
		})
		return
	}

	var comments []models.Comment
	cacheKey := "comments:" + contentIDStr

	if utils.RedisClient != nil {
		if err := utils.GetCacheJSON(cacheKey, &comments); err == nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"message": "获取成功",
				"data":    comments,
			})
			return
		}
	}

	if err := utils.DB.Where("content_id = ? AND parent_id IS NULL AND is_banned = ?", contentID, false).
		Preload("User").
		Order("created_at DESC").
		Find(&comments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取评论失败",
			"data":    nil,
		})
		return
	}

	for i := range comments {
		var replies []models.Comment
		if err := utils.DB.Where("parent_id = ? AND is_banned = ?", comments[i].ID, false).
			Preload("User").
			Preload("Parent").
			Preload("Parent.User").
			Order("created_at ASC").
			Find(&replies).Error; err == nil {
			comments[i].Replies = replies
		}
	}

	if utils.RedisClient != nil {
		utils.SetCacheJSON(cacheKey, comments, 1*time.Hour)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    comments,
	})
}

func DeleteComment(c *gin.Context) {
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

	commentIDStr := c.Param("id")
	commentID, err := strconv.ParseUint(commentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的评论ID",
			"data":    nil,
		})
		return
	}

	var comment models.Comment
	if err := utils.DB.First(&comment, commentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "评论不存在",
			"data":    nil,
		})
		return
	}

	if !currentUser.IsAdmin && comment.UserID != currentUser.ID {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "无权删除此评论",
			"data":    nil,
		})
		return
	}

	if err := utils.DB.Delete(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除评论失败",
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

func GetCommentCount(c *gin.Context) {
	contentIDStr := c.Param("content_id")
	contentID, err := strconv.ParseUint(contentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的内容ID",
			"data":    nil,
		})
		return
	}

	var count int64
	if err := utils.DB.Model(&models.Comment{}).Where("content_id = ?", contentID).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取评论数失败",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data": gin.H{
			"content_id": contentID,
			"count":      count,
		},
	})
}

func ReportComment(c *gin.Context) {
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

	commentIDStr := c.PostForm("comment_id")
	commentID, err := strconv.ParseUint(commentIDStr, 10, 32)
	if err != nil || commentID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的评论ID",
			"data":    nil,
		})
		return
	}

	var comment models.Comment
	if err := utils.DB.First(&comment, commentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "评论不存在",
			"data":    nil,
		})
		return
	}

	if comment.UserID == currentUser.ID {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "不能举报自己的评论",
			"data":    nil,
		})
		return
	}

	var existingReport models.CommentReport
	if err := utils.DB.Where("comment_id = ? AND user_id = ?", commentID, currentUser.ID).First(&existingReport).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "您已举报过此评论",
			"data":    nil,
		})
		return
	}

	reason := c.PostForm("reason")
	if reason == "" {
		reason = "其他"
	}

	report := &models.CommentReport{
		CommentID: uint(commentID),
		UserID:    currentUser.ID,
		Reason:    reason,
	}

	if err := utils.DB.Create(report).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "举报失败",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "举报成功，管理员将尽快处理",
		"data":    report,
	})
}

func GetCommentReports(c *gin.Context) {
	var reports []models.CommentReport
	if err := utils.DB.Where("handled = ?", false).
		Preload("Comment").
		Preload("Comment.User").
		Preload("User").
		Order("created_at DESC").
		Find(&reports).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取举报列表失败",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    reports,
	})
}

func HandleReport(c *gin.Context) {
	reportIDStr := c.Param("id")
	reportID, err := strconv.ParseUint(reportIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的举报ID",
			"data":    nil,
		})
		return
	}

	var report models.CommentReport
	if err := utils.DB.First(&report, reportID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "举报不存在",
			"data":    nil,
		})
		return
	}

	report.Handled = true
	if err := utils.DB.Save(&report).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "处理失败",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "处理成功",
		"data":    report,
	})
}
