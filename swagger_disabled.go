//go:build noswagger || production
// +build noswagger production

package main

import (
	"log"

	"github.com/gin-gonic/gin"
)

func registerSwaggerRoutes(r *gin.Engine) {
	log.Println("[启动] Swagger 文档已禁用")
}

func autoSwagInit() {
	log.Println("[启动] Swagger 文档已禁用，跳过文档生成")
}
