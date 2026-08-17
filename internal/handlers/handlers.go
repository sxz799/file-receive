package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

		for p := range ch {
			// Send progress over websocket
			if err := conn.WriteJSON(p); err != nil {
				fmt.Printf("Error writing JSON to websocket: %v\n", err)
				return
			}
			if p.Done {
				return // Exit if done
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

// uploadScriptTemplate Linux 上传脚本模板，下载时动态替换默认服务地址
const uploadScriptTemplate = `#!/usr/bin/env bash
#
# 文件上传脚本 (Linux)
# 运行后列出当前目录下的文件，可选择文件上传到文件接收服务
#
# 用法:
#   ./upload.sh [服务地址]
#
# 示例:
#   ./upload.sh                        # 使用默认地址 http://localhost:8080
#   ./upload.sh http://192.168.1.10:8080   # 上传到指定服务
#   SERVER_URL=http://192.168.1.10:8080 ./upload.sh   # 通过环境变量指定
#

# 服务地址: 优先取命令行参数，其次取环境变量 SERVER_URL，最后用默认值
SERVER_URL="${SERVER_URL:-http://localhost:8080}"
if [ $# -ge 1 ]; then
    SERVER_URL="$1"
fi

# 检查 curl 是否存在
if ! command -v curl >/dev/null 2>&1; then
    echo "错误: 未找到 curl，请先安装 (Ubuntu/Debian: sudo apt install curl)" >&2
    exit 1
fi

echo "======================================"
echo "  文件上传脚本"
echo "  服务地址: ${SERVER_URL}"
echo "======================================"

while true; do
    # 列出当前目录下的文件（仅文件，不含子目录）
    mapfile -t files < <(find . -maxdepth 1 -type f -printf '%f\n' | sort)

    count=${#files[@]}
    if [ "$count" -eq 0 ]; then
        echo "当前目录下没有文件，退出。"
        exit 0
    fi

    echo ""
    echo "当前目录下的文件:"
    for i in "${!files[@]}"; do
        idx=$((i + 1))
        size=$(stat -c%s "${files[$i]}" 2>/dev/null || echo "?")
        size_h=$(numfmt --to=iec "$size" 2>/dev/null || echo "${size} B")
        printf "  %2d) %-40s (%s)\n" "$idx" "${files[$i]}" "$size_h"
    done

    echo ""
    echo "请选择要上传的文件编号 (1-${count})，输入 q 退出，输入 r 重新列出:"
    if ! read -r -p "> " choice; then
        echo "输入结束，退出。"
        exit 0
    fi

    case "$choice" in
        q|Q|quit|exit)
            echo "已退出。"
            exit 0
            ;;
        r|R|refresh)
            continue
            ;;
        ''|*[!0-9]*)
            echo "无效输入，请输入 1-${count} 之间的数字。"
            continue
            ;;
        *)
            if [ "$choice" -lt 1 ] || [ "$choice" -gt "$count" ]; then
                echo "编号超出范围，请输入 1-${count} 之间的数字。"
                continue
            fi
            ;;
    esac

    file="${files[$((choice - 1))]}"
    echo ""
    echo "开始上传: ${file} -> ${SERVER_URL}/upload"
    echo ""

    # 上传文件，字段名 file 对应服务端接口
    if curl --fail --progress-bar -F "file=@${file}" "${SERVER_URL}/upload"; then
        echo ""
        echo "✅ 文件 ${file} 上传成功。"
    else
        echo ""
        echo "❌ 文件 ${file} 上传失败，请检查服务地址是否可用。" >&2
    fi

    echo ""
    echo "按 Enter 继续..."
    read -r -p "" || exit 0
done
`

// DownloadScriptHandler 下载 Linux 上传脚本，动态替换脚本中的服务地址
func DownloadScriptHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 动态生成当前访问地址（支持反代传递的协议头）
		scheme := "http"
		if proto := c.Request.Header.Get("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		} else if c.Request.TLS != nil {
			scheme = "https"
		}
		serverURL := scheme + "://" + c.Request.Host

		// 将脚本中的默认服务地址替换为当前访问地址
		script := strings.ReplaceAll(uploadScriptTemplate, "http://localhost:8080", serverURL)

		c.Header("Content-Disposition", `attachment; filename="upload.sh"`)
		c.Data(http.StatusOK, "application/x-sh; charset=utf-8", []byte(script))
	}
}
