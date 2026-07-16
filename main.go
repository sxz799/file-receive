package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultPort      = "8080"
	defaultUploadDir = "./uploads"
	staticDir        = "./static"
)

// UploadRecord 上传记录
type UploadRecord struct {
	ID         string    `json:"id"`
	Filename   string    `json:"filename"`
	Size       int64     `json:"size"`
	Path       string    `json:"path"`
	UploadedAt time.Time `json:"uploaded_at"`
}

// Progress 进度管理
type Progress struct {
	mu      sync.Mutex
	clients map[string]chan<- UploadProgress
	nextID  int
}

// UploadProgress 上传进度
type UploadProgress struct {
	Filename string  `json:"filename"`
	Total    int64   `json:"total"`
	Current  int64   `json:"current"`
	Percent  float64 `json:"percent"`
	Done     bool    `json:"done"`
	Error    string  `json:"error,omitempty"`
}

// AppState 应用状态
type AppState struct {
	mu           sync.Mutex
	records      []UploadRecord
	nextRecordID int
	progress     *Progress
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

func NewAppState() *AppState {
	return &AppState{
		records:      make([]UploadRecord, 0),
		nextRecordID: 1,
		progress:     NewProgress(),
	}
}

func (s *AppState) AddRecord(filename string, size int64, path string) UploadRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	record := UploadRecord{
		ID:         fmt.Sprintf("rec-%d", s.nextRecordID),
		Filename:   filename,
		Size:       size,
		Path:       path,
		UploadedAt: time.Now(),
	}

	s.nextRecordID++
	s.records = append(s.records, record)
	return record
}

func (s *AppState) GetRecords() []UploadRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 返回副本，避免外部修改
	records := make([]UploadRecord, len(s.records))
	copy(records, s.records)
	return records
}

func formatBytes(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}
	const k = 1024
	sizes := []string{"B", "KB", "MB", "GB"}
	i := 0
	for ; bytes >= k && i < len(sizes)-1; i++ {
		bytes /= k
	}
	return fmt.Sprintf("%d %s", bytes, sizes[i])
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	state := NewAppState()

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

	if err := os.MkdirAll(staticDir, 0755); err != nil {
		log.Fatalf("无法创建静态目录: %v", err)
	}

	// 静态文件服务
	r.Static("/static", staticDir)
	r.GET("/", func(c *gin.Context) {
		c.File(filepath.Join(staticDir, "index.html"))
	})

	// 文件上传接口
	r.POST("/upload", func(c *gin.Context) {
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
			state.progress.Broadcast(UploadProgress{
				Filename: filename,
				Total:    header.Size,
				Current:  0,
				Percent:  0,
				Done:     false,
				Error:    fmt.Sprintf("创建文件失败: %v", err),
			})
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
					state.progress.Broadcast(UploadProgress{
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
				state.progress.Broadcast(UploadProgress{
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
				state.progress.Broadcast(UploadProgress{
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

		// 添加记录
		record := state.AddRecord(filename, totalSize, dst)

		state.progress.Broadcast(UploadProgress{
			Filename: filename,
			Total:    totalSize,
			Current:  current,
			Percent:  100,
			Done:     true,
		})

		c.JSON(http.StatusOK, gin.H{
			"message": "文件上传成功",
			"record":  record,
		})
	})

	// 获取历史记录
	r.GET("/api/records", func(c *gin.Context) {
		records := state.GetRecords()
		// 反转记录，最新的在最前面
		for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
			records[i], records[j] = records[j], records[i]
		}
		c.JSON(http.StatusOK, gin.H{
			"records": records,
		})
	})

	// SSE 进度推送
	r.GET("/api/progress/sse", func(c *gin.Context) {
		id, ch := state.progress.AddClient()
		defer state.progress.RemoveClient(id)

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

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	log.Printf("服务已启动，监听端口: %s", port)
	log.Printf("文件上传目录: %s", uploadDir)
	log.Printf("访问地址: http://localhost:%s/", port)

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
