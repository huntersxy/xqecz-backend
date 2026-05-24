package handlers

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"xiaoquan-backend/config"
	"xiaoquan-backend/models"
	"xiaoquan-backend/utils"

	"github.com/gin-gonic/gin"
)

type CommentUser struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

type CommentReply struct {
	ID        uint          `json:"id"`
	UserID    uint          `json:"user_id"`
	User      CommentUser   `json:"user"`
	Text      string        `json:"text"`
	ParentID  *uint         `json:"parent_id,omitempty"`
	Parent    *CommentReply `json:"parent,omitempty"`
	IsBanned  bool          `json:"is_banned"`
	CreatedAt int64         `json:"created_at"`
}

type CommentItem struct {
	ID        uint           `json:"id"`
	UserID    uint           `json:"user_id"`
	User      CommentUser    `json:"user"`
	Text      string         `json:"text"`
	ParentID  *uint         `json:"parent_id,omitempty"`
	Parent    *CommentReply  `json:"parent,omitempty"`
	Replies   []CommentReply `json:"replies,omitempty"`
	IsBanned  bool           `json:"is_banned"`
	CreatedAt int64         `json:"created_at"`
}

type CommentListResponse struct {
	List      []CommentItem `json:"list"`
	Total     int64        `json:"total"`
	Page      int          `json:"page"`
	PageSize  int          `json:"page_size"`
	TotalPage int64        `json:"total_page"`
}

// AddComment 添加评论
// @Summary      添加评论
// @Description  对已审核通过的内容添加评论，支持回复其他评论
// @Tags         评论
// @Accept       multipart/form-data
// @Produce      json
// @Security     SessionAuth
// @Param        content_id formData int    true  "内容ID"
// @Param        text       formData string true  "评论内容"
// @Param        parent_id  formData int    false "父评论ID（回复时使用）"
// @Success      200 {object} utils.SwaggerResponse{data=models.Comment}
// @Failure      400 {object} utils.SwaggerResponse
// @Failure      401 {object} utils.SwaggerResponse
// @Router       /comment/add [post]
func AddComment(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		utils.RespondWithError(c, http.StatusUnauthorized, "未登录")
		return
	}
	currentUser := user.(models.User)

	contentIDStr := c.PostForm("content_id")
	contentID, err := strconv.ParseUint(contentIDStr, 10, 32)
	if err != nil || contentID == 0 {
		utils.RespondWithError(c, http.StatusBadRequest, "无效的内容ID")
		return
	}

	var content models.Content
	if err := utils.DB.Where("id = ? AND audit_status = ?", contentID, models.AuditStatusApproved).First(&content).Error; err != nil {
		utils.RespondWithError(c, http.StatusBadRequest, "内容不存在或未通过审核")
		return
	}

	text := c.PostForm("text")
	if text == "" {
		utils.RespondWithError(c, http.StatusBadRequest, "评论内容不能为空")
		return
	}

	if len(text) > 5000 {
		utils.RespondWithError(c, http.StatusBadRequest, "评论内容过长（最大5000字符）")
		return
	}

	if checkBannedWords(text) {
		utils.RespondWithError(c, http.StatusBadRequest, "内容包含违规词汇")
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
		utils.RespondWithError(c, http.StatusInternalServerError, "添加评论失败")
		return
	}

	go asyncCheckSpamAPI(comment.ID, text)

	if utils.RedisClient != nil {
		utils.ClearCommentCache(uint(contentID))
	}

	utils.RespondWithSuccess(c, comment)
}

const bannedWordsKey = 0x5A

func checkBannedWords(text string) bool {
	var words []string

	if utils.RedisClient != nil {
		if err := utils.GetCacheJSON("banned_words", &words); err == nil && len(words) > 0 {
			return checkWordsInList(text, words)
		}
	}

	words = loadBannedWords()
	if len(words) == 0 {
		return false
	}

	if utils.RedisClient != nil {
		utils.SetCacheJSON("banned_words", words, 24*time.Hour)
	}
	return checkWordsInList(text, words)
}

func loadBannedWords() []string {
	binPath := filepath.Join(".", "config", "banned_words.bin")
	data, err := os.ReadFile(binPath)
	if err != nil {
		return defaultBannedList()
	}
	if len(data) < 4 {
		return defaultBannedList()
	}
	count := int(data[0]) | int(data[1])<<8 | int(data[2])<<16 | int(data[3])<<24
	words := make([]string, 0, count)
	pos := 4
	for i := 0; i < count; i++ {
		if pos+2 > len(data) {
			break
		}
		wordLen := int(data[pos]) | int(data[pos+1])<<8
		pos += 2
		if pos+wordLen > len(data) {
			break
		}
		decoded := make([]byte, wordLen)
		for j := 0; j < wordLen; j++ {
			decoded[j] = data[pos+j] ^ bannedWordsKey
		}
		pos += wordLen
		words = append(words, string(decoded))
	}
	return words
}

func defaultBannedList() []string {
	return []string{"admin", "test", "sb", "fuck"}
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
	jsonData, err := sonic.Marshal(requestData)
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

	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	if result.Code == 966 {
		utils.DB.Model(&models.Comment{}).Where("id = ?", commentID).Update("is_banned", true)
	}
}

// GetComments 评论列表
// @Summary      评论列表
// @Description  获取指定内容的评论列表（嵌套回复，分页）
// @Tags         评论
// @Produce      json
// @Param        content_id path int true "内容ID"
// @Param        page      query int false "页码" default(1)
// @Param        page_size query int false "每页数量" default(20)
// @Success      200 {object} utils.SwaggerResponse{data=object{list=[]models.Comment,total=int,page=int,page_size=int,total_page=int}}
// @Router       /comment/list/{content_id} [get]
func GetComments(c *gin.Context) {
	contentIDStr := c.Param("content_id")
	contentID, err := strconv.ParseUint(contentIDStr, 10, 32)
	if err != nil {
		utils.RespondWithError(c, http.StatusBadRequest, "无效的内容ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	var total int64
	query := utils.DB.Model(&models.Comment{}).Where("content_id = ? AND parent_id IS NULL AND is_banned = ?", contentID, false)
	if err := query.Count(&total).Error; err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "获取评论失败")
		return
	}

	var comments []models.Comment
	offset := (page - 1) * pageSize
	cacheKey := fmt.Sprintf("comments:%s:page:%d:size:%d", contentIDStr, page, pageSize)

	if utils.RedisClient != nil {
		var cached CommentListResponse
		if err := utils.GetCacheJSON(cacheKey, &cached); err == nil {
			utils.RespondWithSuccess(c, cached)
			return
		}
	}

	if err := query.
		Preload("User").
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&comments).Error; err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "获取评论失败")
		return
	}

	var allCommentIDs []uint
	commentIDMap := make(map[uint]models.Comment)
	for _, cmt := range comments {
		allCommentIDs = append(allCommentIDs, cmt.ID)
		commentIDMap[cmt.ID] = cmt
	}

	var allReplies []models.Comment
	utils.DB.Where("content_id = ? AND parent_id IS NOT NULL AND is_banned = ?", contentID, false).
		Preload("User").
		Order("created_at ASC").
		Find(&allReplies)

	replyMap := make(map[uint]models.Comment)
	for _, reply := range allReplies {
		replyMap[reply.ID] = reply
	}

	commentRepliesMap := make(map[uint][]CommentReply)
	for _, reply := range allReplies {
		if reply.ParentID != nil {
			replyItem := toReplyItem(reply)
			if parent, ok := replyMap[*reply.ParentID]; ok {
				replyItem.Parent = &CommentReply{
					ID:        parent.ID,
					UserID:    parent.UserID,
					User:      CommentUser{ID: parent.User.ID, Username: parent.User.Username},
					Text:      parent.Text,
					ParentID:  parent.ParentID,
					IsBanned:  parent.IsBanned,
					CreatedAt: parent.CreatedAt.Unix(),
				}
			}

			rootID := *reply.ParentID
			for {
				if _, isRoot := commentIDMap[rootID]; isRoot {
					break
				}
				if parent, hasParent := replyMap[rootID]; hasParent && parent.ParentID != nil {
					rootID = *parent.ParentID
				} else {
					break
				}
			}

			if _, isRoot := commentIDMap[rootID]; isRoot {
				commentRepliesMap[rootID] = append(commentRepliesMap[rootID], replyItem)
			}
		}
	}

	result := CommentListResponse{
		List:      make([]CommentItem, 0, len(comments)),
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
		TotalPage: (total + int64(pageSize) - 1) / int64(pageSize),
	}

	for _, comment := range comments {
		item := toCommentItem(comment)
		if replies, ok := commentRepliesMap[comment.ID]; ok {
			item.Replies = replies
		}
		result.List = append(result.List, item)
	}

	if utils.RedisClient != nil {
		utils.SetCacheJSON(cacheKey, result, 1*time.Hour)
	}

	utils.RespondWithSuccess(c, result)
}

func toCommentItem(c models.Comment) CommentItem {
	return CommentItem{
		ID:        c.ID,
		UserID:    c.UserID,
		User:      CommentUser{ID: c.User.ID, Username: c.User.Username},
		Text:      c.Text,
		ParentID:  c.ParentID,
		IsBanned:  c.IsBanned,
		CreatedAt: c.CreatedAt.Unix(),
	}
}

func toReplyItem(c models.Comment) CommentReply {
	return CommentReply{
		ID:        c.ID,
		UserID:    c.UserID,
		User:      CommentUser{ID: c.User.ID, Username: c.User.Username},
		Text:      c.Text,
		ParentID:  c.ParentID,
		IsBanned:  c.IsBanned,
		CreatedAt: c.CreatedAt.Unix(),
	}
}

// DeleteComment 删除评论
// @Summary      删除评论
// @Description  作者或管理员可删除评论
// @Tags         评论
// @Produce      json
// @Security     SessionAuth
// @Param        id path int true "评论ID"
// @Success      200 {object} utils.SwaggerResponse
// @Failure      401 {object} utils.SwaggerResponse
// @Failure      403 {object} utils.SwaggerResponse
// @Failure      404 {object} utils.SwaggerResponse
// @Router       /comment/{id} [delete]
func DeleteComment(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		utils.RespondWithError(c, http.StatusUnauthorized, "未登录")
		return
	}
	currentUser := user.(models.User)

	commentIDStr := c.Param("id")
	commentID, err := strconv.ParseUint(commentIDStr, 10, 32)
	if err != nil {
		utils.RespondWithError(c, http.StatusBadRequest, "无效的评论ID")
		return
	}

	var comment models.Comment
	if err := utils.DB.First(&comment, commentID).Error; err != nil {
		utils.RespondWithError(c, http.StatusNotFound, "评论不存在")
		return
	}

	if !currentUser.IsAdmin && comment.UserID != currentUser.ID {
		utils.RespondWithError(c, http.StatusForbidden, "无权删除此评论")
		return
	}

	if err := utils.DB.Delete(&comment).Error; err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "删除评论失败")
		return
	}

	if utils.RedisClient != nil {
		utils.ClearCommentCache(comment.ContentID)
	}

	utils.RespondWithSuccess(c, nil)
}

// GetCommentCount 评论数量
// @Summary      评论数量
// @Description  获取指定内容的评论总数
// @Tags         评论
// @Produce      json
// @Param        content_id path int true "内容ID"
// @Success      200 {object} utils.SwaggerResponse{data=object{count=int}}
// @Router       /comment/count/{content_id} [get]
func GetCommentCount(c *gin.Context) {
	contentIDStr := c.Param("content_id")
	contentID, err := strconv.ParseUint(contentIDStr, 10, 32)
	if err != nil {
		utils.RespondWithError(c, http.StatusBadRequest, "无效的内容ID")
		return
	}

	var count int64
	if err := utils.DB.Model(&models.Comment{}).Where("content_id = ?", contentID).Count(&count).Error; err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "获取评论数失败")
		return
	}

	utils.RespondWithSuccess(c, gin.H{
		"content_id": contentID,
		"count":      count,
	})
}

// ReportComment 举报评论
// @Summary      举报评论
// @Description  举报违规评论
// @Tags         评论
// @Accept       multipart/form-data
// @Produce      json
// @Security     SessionAuth
// @Param        comment_id formData int    true  "评论ID"
// @Param        reason     formData string true  "举报原因"
// @Success      200 {object} utils.SwaggerResponse
// @Failure      400 {object} utils.SwaggerResponse
// @Failure      401 {object} utils.SwaggerResponse
// @Router       /comment/report [post]
func ReportComment(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		utils.RespondWithError(c, http.StatusUnauthorized, "未登录")
		return
	}
	currentUser := user.(models.User)

	commentIDStr := c.PostForm("comment_id")
	commentID, err := strconv.ParseUint(commentIDStr, 10, 32)
	if err != nil || commentID == 0 {
		utils.RespondWithError(c, http.StatusBadRequest, "无效的评论ID")
		return
	}

	var comment models.Comment
	if err := utils.DB.First(&comment, commentID).Error; err != nil {
		utils.RespondWithError(c, http.StatusNotFound, "评论不存在")
		return
	}

	if comment.UserID == currentUser.ID {
		utils.RespondWithError(c, http.StatusBadRequest, "不能举报自己的评论")
		return
	}

	var existingReport models.CommentReport
	if err := utils.DB.Where("comment_id = ? AND user_id = ?", commentID, currentUser.ID).First(&existingReport).Error; err == nil {
		utils.RespondWithError(c, http.StatusBadRequest, "您已举报过此评论")
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
		utils.RespondWithError(c, http.StatusInternalServerError, "举报失败")
		return
	}

	utils.RespondWithSuccess(c, report)
}

// GetCommentReports 举报列表
// @Summary      举报列表
// @Description  管理员查看评论举报列表
// @Tags         管理
// @Produce      json
// @Security     SessionAuth
// @Success      200 {object} utils.SwaggerResponse
// @Router       /admin/comments/reports [get]
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

// HandleReport 处理举报
// @Summary      处理举报
// @Description  管理员处理评论举报，标记为已处理
// @Tags         管理
// @Produce      json
// @Security     SessionAuth
// @Param        id path int true "举报ID"
// @Success      200 {object} utils.SwaggerResponse{data=models.CommentReport}
// @Failure      400 {object} utils.SwaggerResponse
// @Failure      404 {object} utils.SwaggerResponse
// @Router       /admin/comments/reports/{id}/handle [post]
func HandleReport(c *gin.Context) {
	reportIDStr := c.Param("id")
	reportID, err := strconv.ParseUint(reportIDStr, 10, 32)
	if err != nil {
		utils.RespondWithError(c, http.StatusBadRequest, "无效的举报ID")
		return
	}

	var report models.CommentReport
	if err := utils.DB.First(&report, reportID).Error; err != nil {
		utils.RespondWithError(c, http.StatusNotFound, "举报不存在")
		return
	}

	report.Handled = true
	if err := utils.DB.Save(&report).Error; err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "处理失败")
		return
	}

	utils.RespondWithSuccess(c, report)
}
