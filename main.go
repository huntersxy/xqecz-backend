// @title           小泉动漫 API
// @version         1.0
// @description     小泉动漫后端API接口 - 支持内容管理、评论、投票、用户认证、站外资源等
// @host            localhost:8080
// @BasePath        /api
//
// @securityDefinitions.apikey  SessionAuth
// @in                          cookie
// @name                        session_id
// @description                登录后自动携带的session cookie

package main

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"xiaoquan-backend/config"
	"xiaoquan-backend/handlers"
	"xiaoquan-backend/middleware"
	"xiaoquan-backend/models"
	"xiaoquan-backend/scheduler"
	"xiaoquan-backend/services"
	"xiaoquan-backend/utils"

	"github.com/gin-gonic/gin"
)

//go:embed dy.webp
var defaultCoverData []byte

func main() {
	gin.SetMode(gin.ReleaseMode)
	log.Println("[启动] 正在初始化应用程序...")

	// 自动重新生成 Swagger 文档
	autoSwagInit()

	if err := config.LoadConfig("./config/config.yaml"); err != nil {
		log.Fatalf("[启动] 配置加载失败: %v", err)
	}
	log.Println("[启动] 配置加载完成")

	if err := utils.InitDB(); err != nil {
		log.Fatalf("[启动] 数据库初始化失败: %v", err)
	}
	log.Println("[启动] 数据库连接成功")

	result := utils.DB.Unscoped().Where("deleted_at IS NOT NULL").Delete(&models.Content{})
	if result.RowsAffected > 0 {
		log.Printf("[启动] 已清理 %d 条软删除记录", result.RowsAffected)
	}

	if config.AppConfig.MigrateThumbnails {
		log.Println("[启动] 执行缩略图路径迁移...")
		MigrateThumbnails()
		config.AppConfig.MigrateThumbnails = false
		if err := config.SaveConfig("./config/config.yaml"); err != nil {
			log.Printf("[启动] 保存迁移状态失败: %v", err)
	} else {
		log.Println("[启动] Redis连接成功")
		utils.ClearCachesOnStartup()
		go scheduler.StartRecommendScheduler()
	}
	}

	if err := utils.InitRedis(); err != nil {
		log.Printf("[警告] Redis连接失败，将使用内存存储Session: %v", err)
	} else {
		log.Println("[启动] Redis连接成功")
		// 启动推荐列表更新调度器
		go scheduler.StartRecommendScheduler()
	}

	if err := utils.CheckFFmpeg(); err != nil {
		log.Printf("[警告] %v，视频封面图功能将不可用", err)
	} else {
		version, _ := utils.GetFFmpegVersion()
		log.Printf("[信息] FFmpeg环境检测成功 - %s", version)
	}

	services.EnsureDefaultCover(defaultCoverData)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(corsMiddleware())

	// 使用全局错误处理中间件
	r.Use(middleware.ErrorHandler())

	r.MaxMultipartMemory = config.AppConfig.Server.MaxUploadSize

	r.Static("/uploads", config.AppConfig.Server.UploadDir)
	r.Static("/thumbnails", config.AppConfig.Server.ThumbnailDir)

	registerSwaggerRoutes(r)

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

		bot := api.Group("/bot")
		{
			bot.POST("/upload", handlers.BotUpload)
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
			content.POST("/:content_id/claim", middleware.AuthMiddleware(), handlers.CreateClaim)
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
			admin.PUT("/content/:id/author", handlers.UpdateContentAuthor)
			admin.DELETE("/content/purge", handlers.PurgeDeletedContent)
			admin.GET("/users", handlers.GetUsers)
			admin.PUT("/users/:id/role", handlers.UpdateUserRole)
			admin.PUT("/users/:id/ban", handlers.BanUser)
			admin.DELETE("/users/:id", handlers.DeleteUser)
			admin.GET("/comments/reports", handlers.GetCommentReports)
			admin.POST("/comments/reports/:id/handle", handlers.HandleReport)
			admin.POST("/content/:id/regenerate-thumbnail", handlers.RegenerateVideoThumbnail)
			admin.POST("/content/regenerate-all-thumbnails", handlers.RegenerateAllThumbnails)
			admin.DELETE("/files/clean", handlers.CleanOrphanedFiles)
			admin.GET("/claims", handlers.GetClaimList)
			admin.POST("/claims/:id/handle", handlers.HandleClaim)
		}

		poll := api.Group("/poll")
		{
			poll.GET("/list", handlers.GetPollList)
			poll.GET("/:id", handlers.GetPoll)
			poll.POST("/:id/vote", handlers.VotePoll)
		}

		pollAuth := api.Group("/poll")
		pollAuth.Use(middleware.AuthMiddleware(), middleware.BannedMiddleware())
		{
			pollAuth.POST("/create", handlers.CreatePoll)
			pollAuth.DELETE("/:id", handlers.DeletePoll)
		}
	}

	addr := fmt.Sprintf(":%d", config.AppConfig.Server.Port)
	log.Printf("Server starting on port %d", config.AppConfig.Server.Port)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func MigrateThumbnails() {
	var contents []models.Content
	utils.DB.Where("thumb_path != ?", "").Find(&contents)

	count := 0
	for _, content := range contents {
		srcPath := filepath.Join(config.AppConfig.Server.UploadDir, content.ThumbPath)
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			continue
		}

		destPath := filepath.Join(config.AppConfig.Server.ThumbnailDir, content.ThumbPath)
		if _, err := os.Stat(destPath); err == nil {
			os.Remove(srcPath)
			count++
			continue
		}

		if err := os.Rename(srcPath, destPath); err != nil {
			log.Printf("[迁移] 移动失败 ID=%d file=%s: %v", content.ID, content.ThumbPath, err)
		} else {
			count++
		}
	}
	log.Printf("[迁移] 已移动 %d 个缩略图文件到 thumbnails/ 目录", count)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			c.Next()
			return
		}
		if !isOriginAllowed(origin) {
			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(204)
			} else {
				c.JSON(http.StatusForbidden, gin.H{
					"code":    403,
					"message": fmt.Sprintf("跨域请求被拒绝，来源 %s 不在白名单中。请在 config.yaml 的 server.allowed_origins 中添加该域名，或通过 localhost 访问。", origin),
					"data":    nil,
				})
			}
			return
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
	}
}

func isOriginAllowed(origin string) bool {
	if strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "https://localhost:") {
		return true
	}
	for _, o := range config.AppConfig.Server.AllowedOrigins {
		if o == origin {
			return true
		}
	}
	return false
}


