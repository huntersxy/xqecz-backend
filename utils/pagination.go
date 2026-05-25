package utils

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	DefaultPageSize   = 20
	MaxPageSize       = 100
	CacheDuration5Min = 5 * time.Minute
)

func ParsePagination(c *gin.Context, defaultSize, maxSize int) (page, pageSize, offset int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", strconv.Itoa(defaultSize)))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultSize
	}
	if pageSize > maxSize {
		pageSize = maxSize
	}

	offset = (page - 1) * pageSize
	return
}

func BuildSortClause(c *gin.Context) string {
	sortBy := c.DefaultQuery("sort_by", "created_at")
	order := c.DefaultQuery("order", "desc")

	sortCol := "created_at"
	switch sortBy {
	case "updated_at":
		sortCol = "updated_at"
	case "id":
		sortCol = "id"
	}

	sortOrder := "desc"
	if order == "asc" {
		sortOrder = "asc"
	}
	return sortCol + " " + sortOrder
}
