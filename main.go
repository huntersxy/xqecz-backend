package main

import (
	"fmt"
	"log"
	"xiaoquan-backend/config"
	"xiaoquan-backend/handlers"
	"xiaoquan-backend/middleware"
	"xiaoquan-backend/utils"

	"github.com/gin-gonic/gin"
)

func main() {
	log.Println("[启动] 正在初始化应用程序...")

	if err := config.LoadConfig("./config/config.yaml"); err != nil {
		log.Fatalf("[启动] 配置加载失败: %v", err)
	}
	log.Println("[启动] 配置加载完成")

	if err := utils.InitDB(); err != nil {
		log.Fatalf("[启动] 数据库初始化失败: %v", err)
	}
	log.Println("[启动] 数据库连接成功")

	if err := utils.InitRedis(); err != nil {
		log.Printf("[警告] Redis连接失败，将使用内存存储Session: %v", err)
	} else {
		log.Println("[启动] Redis连接成功")
	}

	if err := utils.CheckFFmpeg(); err != nil {
		log.Printf("[警告] %v，视频封面图功能将不可用", err)
	} else {
		version, _ := utils.GetFFmpegVersion()
		log.Printf("[信息] FFmpeg环境检测成功 - %s", version)
	}

	r := gin.Default()

	// 添加CORS中间件
	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 使用全局错误处理中间件
	r.Use(middleware.ErrorHandler())

	r.MaxMultipartMemory = config.AppConfig.Server.MaxUploadSize

	r.Static("/uploads", config.AppConfig.Server.UploadDir)
	r.Static("/thumbnails", config.AppConfig.Server.ThumbnailDir)

	api := r.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", handlers.Register)
			auth.POST("/login", handlers.Login)
			auth.POST("/logout", handlers.Logout)
			auth.POST("/init-admin", handlers.InitAdmin)
			auth.GET("/me", middleware.AuthMiddleware(), handlers.GetMe)
		}

		content := api.Group("/content")
		{
			content.POST("/upload", middleware.AuthMiddleware(), middleware.BannedMiddleware(), handlers.UploadContent)
			content.POST("/upload-image", middleware.AuthMiddleware(), middleware.BannedMiddleware(), handlers.UploadArticleImage)
			content.GET("/list", handlers.GetContentList)
			content.GET("/search", handlers.SearchContent)
			content.GET("/recommend", handlers.RecommendContent)
			content.GET("/:id", handlers.GetContent)
			content.PUT("/:id", middleware.AuthMiddleware(), middleware.BannedMiddleware(), handlers.UpdateContent)
			content.DELETE("/:id", middleware.AuthMiddleware(), middleware.BannedMiddleware(), handlers.DeleteContent)
			content.GET("/my", middleware.AuthMiddleware(), handlers.GetMyContentList)
			content.GET("/tags", handlers.GetAllTags)
		}

		comment := api.Group("/comment")
		comment.Use(middleware.AuthMiddleware(), middleware.BannedMiddleware())
		{
			comment.POST("/add", handlers.AddComment)
			comment.DELETE("/:id", handlers.DeleteComment)
			comment.POST("/report", handlers.ReportComment)
		}

		commentPublic := api.Group("/comment")
		{
			commentPublic.GET("/list/:content_id", handlers.GetComments)
			commentPublic.GET("/count/:content_id", handlers.GetCommentCount)
		}

		admin := api.Group("/admin")
		admin.Use(middleware.AuthMiddleware(), middleware.AdminMiddleware())
		{
			admin.POST("/audit/:id", handlers.AuditContent)
			admin.GET("/pending", handlers.GetPendingContent)
			admin.GET("/content/all", handlers.GetAllContent)
			admin.GET("/users", handlers.GetUsers)
			admin.PUT("/users/:id/role", handlers.UpdateUserRole)
			admin.PUT("/users/:id/ban", handlers.BanUser)
			admin.DELETE("/users/:id", handlers.DeleteUser)
			admin.GET("/comments/reports", handlers.GetCommentReports)
			admin.POST("/comments/reports/:id/handle", handlers.HandleReport)
			admin.POST("/content/:id/regenerate-thumbnail", handlers.RegenerateVideoThumbnail)
		}
	}

	addr := fmt.Sprintf(":%d", config.AppConfig.Server.Port)
	log.Printf("Server starting on port %d", config.AppConfig.Server.Port)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
