package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"xiaoquan-backend/utils"
)

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			utils.RespondWithError(c, http.StatusInternalServerError, "遇到问题啦")
			c.Abort()
		}
	}
}
