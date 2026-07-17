package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"file-receive/internal/appstate"
	"file-receive/internal/handlers"
)

const (
	defaultPort      = "8080"
	defaultUploadDir = "./uploads"
)

//go:embed static
var embeddedFiles embed.FS

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	state := appstate.NewAppState()

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = defaultUploadDir
	}

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("无法创建上传目录: %v", err)
	}

	// Use embedded files for static content
	subFS, err := fs.Sub(embeddedFiles, "static")
	if err != nil {
		log.Fatalf("Failed to get sub FS: %v", err)
	}
	r.StaticFS("/static", http.FS(subFS))
	r.GET("/", func(c *gin.Context) {
		c.FileFromFS("index.html", http.FS(subFS))
	})

	// API 路由
	r.POST("/upload", handlers.UploadFileHandler(state, uploadDir))
	r.GET("/api/records", handlers.GetRecordsHandler(state))
	r.GET("/ws/upload-progress", handlers.WSProgressHandler(state))
	r.GET("/health", handlers.HealthCheckHandler())

	log.Printf("服务已启动，监听端口: %s", port)
	log.Printf("文件上传目录: %s", uploadDir)
	log.Printf("访问地址: http://localhost:%s/", port)

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
