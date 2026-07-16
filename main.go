package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/gin-gonic/gin"
)

const (
	defaultPort      = "8080"
	defaultUploadDir = "./uploads"
)

type Progress struct {
	mu      sync.Mutex
	clients map[string]chan<- UploadProgress
	nextID  int
}

type UploadProgress struct {
	Filename string  `json:"filename"`
	Total    int64   `json:"total"`
	Current  int64   `json:"current"`
	Percent  float64 `json:"percent"`
	Done     bool    `json:"done"`
	Error    string  `json:"error,omitempty"`
}

func NewProgress() *Progress {
	return &Progress{
		clients: make(map[string]chan<- UploadProgress),
		nextID:  1,
	}
}

func (p *Progress) AddClient() (string, <-chan UploadProgress) {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := fmt.Sprintf("client-%d", p.nextID)
	p.nextID++
	ch := make(chan UploadProgress, 100)
	p.clients[id] = ch
	return id, ch
}

func (p *Progress) RemoveClient(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ch, ok := p.clients[id]; ok {
		close(ch)
		delete(p.clients, id)
	}
}

func (p *Progress) Broadcast(progress UploadProgress) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, ch := range p.clients {
		select {
		case ch <- progress:
		default:
		}
	}
}

func main() {
	r := gin.Default()
	progress := NewProgress()

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

	r.POST("/upload", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "请选择要上传的文件",
			})
			return
		}

		filename := filepath.Base(file.Filename)
		dst := filepath.Join(uploadDir, filename)

		if err := c.SaveUploadedFile(file, dst); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("文件保存失败: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":  "文件上传成功",
			"filename": filename,
			"path":     dst,
		})
	})

	r.POST("/upload/progress", func(c *gin.Context) {
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "解析表单失败",
			})
			return
		}

		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "请选择要上传的文件",
			})
			return
		}
		defer file.Close()

		filename := filepath.Base(header.Filename)
		dst := filepath.Join(uploadDir, filename)

		out, err := os.Create(dst)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("创建文件失败: %v", err),
			})
			return
		}
		defer out.Close()

		totalSize := header.Size
		var current int64
		buf := make([]byte, 32*1024)

		for {
			n, err := file.Read(buf)
			if n > 0 {
				if _, writeErr := out.Write(buf[:n]); writeErr != nil {
					progress.Broadcast(UploadProgress{
						Filename: filename,
						Total:    totalSize,
						Current:  current,
						Percent:  float64(current) / float64(totalSize) * 100,
						Done:     false,
						Error:    fmt.Sprintf("写入文件失败: %v", writeErr),
					})
					c.JSON(http.StatusInternalServerError, gin.H{
						"error": fmt.Sprintf("写入文件失败: %v", writeErr),
					})
					return
				}
				current += int64(n)
				percent := float64(current) / float64(totalSize) * 100
				progress.Broadcast(UploadProgress{
					Filename: filename,
					Total:    totalSize,
					Current:  current,
					Percent:  percent,
					Done:     false,
				})
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				progress.Broadcast(UploadProgress{
					Filename: filename,
					Total:    totalSize,
					Current:  current,
					Percent:  float64(current) / float64(totalSize) * 100,
					Done:     false,
					Error:    fmt.Sprintf("读取文件失败: %v", err),
				})
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("读取文件失败: %v", err),
				})
				return
			}
		}

		progress.Broadcast(UploadProgress{
			Filename: filename,
			Total:    totalSize,
			Current:  current,
			Percent:  100,
			Done:     true,
		})

		c.JSON(http.StatusOK, gin.H{
			"message":  "文件上传成功",
			"filename": filename,
			"path":     dst,
		})
	})

	r.GET("/upload/progress/sse", func(c *gin.Context) {
		id, ch := progress.AddClient()
		defer progress.RemoveClient(id)

		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

		c.Stream(func(w io.Writer) bool {
			select {
			case p, ok := <-ch:
				if !ok {
					return false
				}
				if p.Done {
					c.SSEvent("progress", p)
					c.SSEvent("done", p)
					return false
				}
				c.SSEvent("progress", p)
				return true
			case <-c.Writer.CloseNotify():
				return false
			}
		})
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	log.Printf("服务已启动，监听端口: %s", port)
	log.Printf("文件上传目录: %s", uploadDir)
	log.Printf("健康检查: http://localhost:%s/health", port)
	log.Printf("简单上传: http://localhost:%s/upload", port)
	log.Printf("带进度上传: http://localhost:%s/upload/progress", port)
	log.Printf("进度SSE: http://localhost:%s/upload/progress/sse", port)

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
