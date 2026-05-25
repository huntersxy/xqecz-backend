package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type SwaggerResponse struct {
	Code    int         `json:"code" example:"200"`
	Message string      `json:"message" example:"操作成功"`
	Data    interface{} `json:"data"`
}

type ErrorResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func RespondWithError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, ErrorResponse{
		Code:    statusCode,
		Message: message,
		Data:    nil,
	})
}

func RespondWithSuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "操作成功",
		"data":    data,
	})
}
