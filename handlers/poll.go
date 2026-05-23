package handlers

import (
	"fmt"
	"net/http"
	"xiaoquan-backend/models"
	"xiaoquan-backend/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CreatePoll 创建投票
// @Summary      创建投票
// @Description  创建新的投票
// @Tags         投票
// @Accept       json
// @Produce      json
// @Security     SessionAuth
// @Param        body body object{title=string,description=string,options=[]string} true "投票信息"
// @Success      200 {object} utils.SwaggerResponse{data=models.Poll}
// @Failure      400 {object} utils.SwaggerResponse
// @Failure      401 {object} utils.SwaggerResponse
// @Router       /poll/create [post]
func CreatePoll(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		utils.RespondWithError(c, http.StatusUnauthorized, "未登录")
		return
	}
	currentUser := user.(models.User)

	var req struct {
		Title       string   `json:"title" binding:"required"`
		Description string   `json:"description"`
		Options     []string `json:"options" binding:"required,min=2"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondWithError(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	poll := models.Poll{
		Title:       req.Title,
		Description: req.Description,
		Options:     req.Options,
		UserID:      currentUser.ID,
	}

	if err := utils.DB.Create(&poll).Error; err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "创建失败")
		return
	}

	if err := utils.DB.Preload("User").First(&poll, poll.ID).Error; err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "查询失败")
		return
	}

	utils.RespondWithSuccess(c, poll)
}

// GetPollList 投票列表
// @Summary      投票列表
// @Description  获取投票列表
// @Tags         投票
// @Produce      json
// @Param        page      query int false "页码" default(1)
// @Param        page_size query int false "每页数量" default(20)
// @Success      200 {object} utils.SwaggerResponse{data=object{list=[]models.Poll,total=int,page=int,page_size=int,total_page=int}}
// @Router       /poll/list [get]
func GetPollList(c *gin.Context) {
	var polls []models.Poll
	query := utils.DB.Preload("User").Order("created_at DESC")

	if err := query.Find(&polls).Error; err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "查询失败")
		return
	}

	utils.RespondWithSuccess(c, gin.H{
		"list": polls,
	})
}

// GetPoll 投票详情
// @Summary      投票详情
// @Description  获取单个投票的详细信息
// @Tags         投票
// @Produce      json
// @Param        id path int true "投票ID"
// @Success      200 {object} utils.SwaggerResponse{data=models.Poll}
// @Failure      404 {object} utils.SwaggerResponse
// @Router       /poll/{id} [get]
func GetPoll(c *gin.Context) {
	id := c.Param("id")
	var poll models.Poll

	if err := utils.DB.Preload("User").First(&poll, id).Error; err != nil {
		utils.RespondWithError(c, http.StatusNotFound, "投票不存在")
		return
	}

	var voteCounts = make(map[int]int64)
	var totalVotes int64
	utils.DB.Model(&models.PollVote{}).Where("poll_id = ?", poll.ID).Count(&totalVotes)

	var votes []models.PollVote
	utils.DB.Where("poll_id = ?", poll.ID).Find(&votes)
	for _, vote := range votes {
		voteCounts[vote.OptionIndex]++
	}

	user, exists := c.Get("user")
	var myVoteIndex *int
	if exists {
		currentUser := user.(models.User)
		var myVote models.PollVote
		if err := utils.DB.Where("poll_id = ? AND user_id = ?", poll.ID, currentUser.ID).First(&myVote).Error; err == nil {
			myVoteIndex = &myVote.OptionIndex
		}
	} else {
		visitorID, err := c.Cookie("visitor_id")
		if err == nil {
			var myVote models.PollVote
			if err := utils.DB.Where("poll_id = ? AND visitor_id = ?", poll.ID, visitorID).First(&myVote).Error; err == nil {
				myVoteIndex = &myVote.OptionIndex
			}
		}
	}

	utils.RespondWithSuccess(c, gin.H{
		"poll":        poll,
		"vote_counts": voteCounts,
		"total_votes": totalVotes,
		"my_vote":     myVoteIndex,
	})
}

// VotePoll 投票
// @Summary      投票
// @Description  对投票进行投票（登录用户或匿名访客均可）
// @Tags         投票
// @Accept       json
// @Produce      json
// @Param        id path int true "投票ID"
// @Param        body body object{option_index=int,visitor_id=string} true "投票信息"
// @Success      200 {object} utils.SwaggerResponse
// @Failure      400 {object} utils.SwaggerResponse
// @Router       /poll/{id}/vote [post]
func VotePoll(c *gin.Context) {
	id := c.Param("id")
	var poll models.Poll
	if err := utils.DB.First(&poll, id).Error; err != nil {
		utils.RespondWithError(c, http.StatusNotFound, "投票不存在")
		return
	}

	var req struct {
		OptionIndex int `json:"option_index"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondWithError(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	if req.OptionIndex < 0 {
		utils.RespondWithError(c, http.StatusBadRequest, "选项索引不能为负数")
		return
	}

	if req.OptionIndex >= len(poll.Options) {
		utils.RespondWithError(c, http.StatusBadRequest, "选项无效")
		return
	}

	var vote models.PollVote
	user, exists := c.Get("user")
	if exists {
		currentUser := user.(models.User)
		if err := utils.DB.Where("poll_id = ? AND user_id = ?", poll.ID, currentUser.ID).First(&vote).Error; err == nil {
			utils.RespondWithError(c, http.StatusBadRequest, "已经投过票了")
			return
		}
		vote = models.PollVote{
			PollID:      poll.ID,
			UserID:      &currentUser.ID,
			OptionIndex: req.OptionIndex,
		}
	} else {
		visitorID, err := c.Cookie("visitor_id")
		if err != nil {
			visitorID = utils.GenerateRandomString(32)
		}

		if err := utils.DB.Where("poll_id = ? AND visitor_id = ?", poll.ID, visitorID).First(&vote).Error; err == nil {
			utils.RespondWithError(c, http.StatusBadRequest, "已经投过票了")
			return
		}

		vote = models.PollVote{
			PollID:      poll.ID,
			VisitorID:   visitorID,
			OptionIndex: req.OptionIndex,
		}

		c.SetCookie("visitor_id", visitorID, 30*24*60*60, "/", "", true, false)
		c.Writer.Header().Add("Set-Cookie", fmt.Sprintf("visitor_id=%s; Path=/; Max-Age=%d; SameSite=None; Secure", visitorID, 30*24*60*60))
	}

	if err := utils.DB.Create(&vote).Error; err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "投票失败")
		return
	}

	utils.DB.Model(&poll).UpdateColumn("vote_count", gorm.Expr("vote_count + 1"))

	utils.RespondWithSuccess(c, nil)
}

// DeletePoll 删除投票
// @Summary      删除投票
// @Description  作者或管理员删除投票
// @Tags         投票
// @Produce      json
// @Security     SessionAuth
// @Param        id path int true "投票ID"
// @Success      200 {object} utils.SwaggerResponse
// @Failure      401 {object} utils.SwaggerResponse
// @Failure      403 {object} utils.SwaggerResponse
// @Failure      404 {object} utils.SwaggerResponse
// @Router       /poll/{id} [delete]
func DeletePoll(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		utils.RespondWithError(c, http.StatusUnauthorized, "未登录")
		return
	}
	currentUser := user.(models.User)

	id := c.Param("id")
	var poll models.Poll
	if err := utils.DB.First(&poll, id).Error; err != nil {
		utils.RespondWithError(c, http.StatusNotFound, "投票不存在")
		return
	}

	if !currentUser.IsAdmin && poll.UserID != currentUser.ID {
		utils.RespondWithError(c, http.StatusForbidden, "无权删除")
		return
	}

	if err := utils.DB.Delete(&poll).Error; err != nil {
		utils.RespondWithError(c, http.StatusInternalServerError, "删除失败")
		return
	}

	utils.RespondWithSuccess(c, nil)
}
