package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"file-receive/internal/appstate"
	"file-receive/internal/models"
)

// UploadFileHandler 处理文件上传
func UploadFileHandler(state *appstate.AppState, uploadDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
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
			state.Progress.Broadcast(models.UploadProgress{
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
					state.Progress.Broadcast(models.UploadProgress{
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
				state.Progress.Broadcast(models.UploadProgress{
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
				state.Progress.Broadcast(models.UploadProgress{
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

		state.Progress.Broadcast(models.UploadProgress{
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
	}
}

// GetRecordsHandler 获取历史记录
func GetRecordsHandler(state *appstate.AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		records := state.GetRecords()
		// 反转记录，最新的在最前面
		for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
			records[i], records[j] = records[j], records[i]
		}
		c.JSON(http.StatusOK, gin.H{
			"records": records,
		})
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// WSProgressHandler WebSocket 进度推送
func WSProgressHandler(state *appstate.AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			fmt.Printf("Failed to set websocket upgrade: %v\n", err)
			return
		}
		defer conn.Close()

		id, ch := state.Progress.AddClient()
		defer state.Progress.RemoveClient(id)

		for {
			select {
			case p, ok := <-ch:
				if !ok {
					// Channel closed, client removed
					return
				}
				// Send progress over websocket
				if err := conn.WriteJSON(p); err != nil {
					fmt.Printf("Error writing JSON to websocket: %v\n", err)
					return
				}
				if p.Done {
					return
				}
			}
		}
	}
}

// HealthCheckHandler 健康检查
func HealthCheckHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	}
}
