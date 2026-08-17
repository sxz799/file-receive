# 文件接收服务

一个基于 Go + Gin 的简单 Web 文件接收服务，同时提供后端接口和静态 Web 界面。

## 功能特性

- 单文件上传接口
- 静态文件服务
- 带进度的文件上传接口
- WebSocket 实时进度推送
- 可配置的上传目录
- 可配置的监听端口
- 健康检查接口
- 简单的错误处理

## 快速开始

### 前置条件

- Go 1.16+

### 安装

1. 克隆或下载项目到本地

2. 安装依赖

```bash
go mod tidy
```

### 运行

#### 使用默认配置运行

```bash
go run main.go
```

#### 使用环境变量配置运行

```bash
# Windows (PowerShell)
$env:PORT = "8080"; $env:UPLOAD_DIR = "./my-uploads"; go run main.go

# Windows (CMD)
set PORT=8080 && set UPLOAD_DIR=./my-uploads && go run main.go

# Linux/Mac
PORT=8080 UPLOAD_DIR=./my-uploads go run main.go
```

#### 编译运行

```bash
# 编译
go build -o file-receive.exe

# 运行（默认配置）
.\file-receive.exe

# 运行（配置环境变量）
$env:PORT = "8080"; $env:UPLOAD_DIR = "./uploads"; .\file-receive.exe
```

### 访问 Web 界面

打开浏览器访问 `http://localhost:8080` 即可使用文件上传功能。

## API 接口

### 1. 健康检查

**接口：** `GET /health`

**响应示例：**

```json
{
  "status": "ok"
}
```

### 2. 文件上传

**接口：** `POST /upload`

**请求类型：** `multipart/form-data`

**参数：**
- `file` (必填): 要上传的文件

**响应示例（成功）：**

```json
{
  "message": "文件上传成功",
  "record": {
    "id": "rec-1",
    "filename": "example.txt",
    "size": 1024,
    "path": "./uploads/example.txt",
    "uploaded_at": "2026-07-16T12:00:00Z"
  }
}
```

**响应示例（失败）：**

```json
{
  "error": "请选择要上传的文件"
}
```

### 3. 获取已上传文件记录

**接口：** `GET /api/records`

**响应示例：**

```json
{
  "records": [
    {
      "id": "rec-1",
      "filename": "example.txt",
      "size": 1024,
      "path": "./uploads/example.txt",
      "uploaded_at": "2026-07-16T12:00:00Z"
    }
  ]
}
```

### 4. WebSocket 实时进度推送

**接口：** `GET /ws/upload-progress`

**说明：** 使用 WebSocket 技术实时推送上传进度。当文件上传到 `/upload` 接口时，此接口会向所有连接的客户端推送进度信息。当收到 `done: true` 的进度消息后，服务端会自动断开连接。

**数据格式（每条消息为一个 JSON 对象）：**

```json
{
  "filename": "example.txt",
  "total": 1024000,
  "current": 512000,
  "percent": 50,
  "done": false,
  "error": ""
}
```

**使用示例（JavaScript）：**

```javascript
const ws = new WebSocket('ws://localhost:8080/ws/upload-progress');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log(`上传进度: ${data.percent.toFixed(2)}%`);
  if (data.done) {
    console.log('上传完成:', data.filename);
    ws.close();
  }
};

ws.onerror = (err) => {
  console.error('WebSocket 错误:', err);
  ws.close();
};
```

## 命令使用方式

### 使用 curl

#### 文件上传

```bash
curl -X POST -F "file=@/path/to/your/file.txt" http://localhost:8080/upload
```

#### 获取文件记录

```bash
curl http://localhost:8080/api/records
```

#### 监听 WebSocket 进度

`curl` 不支持 WebSocket 协议，推荐使用 [websocat](https://github.com/vi/websocat) 等工具监听：

```bash
websocat ws://localhost:8080/ws/upload-progress
```

### 使用 PowerShell

#### 文件上传

```powershell
$uri = "http://localhost:8080/upload"
$filePath = "D:\path\to\your\file.txt"
$form = @{ file = Get-Item -Path $filePath }
Invoke-RestMethod -Uri $uri -Method Post -Form $form
```

#### 获取文件记录

```powershell
Invoke-RestMethod -Uri "http://localhost:8080/api/records" -Method Get
```

#### 监听 WebSocket 进度

```powershell
$uri = "ws://localhost:8080/ws/upload-progress"
$ws = New-Object System.Net.WebSockets.ClientWebSocket
$ws.ConnectAsync([System.Uri]$uri, [System.Threading.CancellationToken]::None).Wait()
$buffer = New-Object byte[] 4096

while ($ws.State -eq 'Open') {
    $segment = New-Object System.ArraySegment[byte] -ArgumentList (, $buffer)
    $result = $ws.ReceiveAsync($segment, [System.Threading.CancellationToken]::None).Result
    $text = [System.Text.Encoding]::UTF8.GetString($buffer, 0, $result.Count)
    Write-Host $text
    if ($text -match '"done":true') { break }
}

$ws.Dispose()
```

### 使用 Postman

#### 文件上传

1. 新建 POST 请求到 `http://localhost:8080/upload`
2. 选择 Body 标签
3. 选择 form-data
4. Key 填 `file`，类型选择 File，然后选择要上传的文件
5. 点击 Send

#### 获取文件记录

1. 新建 GET 请求到 `http://localhost:8080/api/records`
2. 点击 Send

#### 监听 WebSocket 进度

1. 新建 WebSocket 请求，地址填 `ws://localhost:8080/ws/upload-progress`
2. 点击 Connect
3. 可以在响应区域看到实时更新的进度数据
## 环境变量

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| PORT | 8080 | 服务监听端口 |
| UPLOAD_DIR | ./uploads | 文件保存目录 |

## 目录结构

```
file-receive/
├── main.go                    # 主程序文件
├── go.mod                     # Go 模块文件
├── go.sum                     # 依赖锁定文件
├── internal/                  # 内部包
│   ├── appstate/              # 应用状态管理（上传记录存储 + 进度广播）
│   ├── handlers/              # HTTP 请求处理器（上传、记录、WebSocket、健康检查）
│   ├── models/                # 数据模型
│   └── utils/                 # 工具函数
├── static/                    # 静态文件目录，包含 index.html 等
├── .gitignore                 # Git 忽略文件
├── Dockerfile                 # Docker 构建文件
├── docker-compose.yml         # Docker Compose 配置
└── README.md                  # 项目说明文档
```

## 技术栈

- Go
- Gin Web Framework

## Docker 部署

### 构建镜像

```bash
docker build -t file-receive .
```

### 运行容器

**基本运行：**
```bash
docker run -d -p 8080:8080 --name file-receive file-receive
```

**带数据卷（推荐）：**
```bash
# 创建数据卷
docker volume create file-receive-data

# 运行容器并挂载卷
docker run -d \
  -p 8080:8080 \
  -v file-receive-data:/uploads \
  --name file-receive \
  file-receive
```

**使用自定义端口：**
```bash
docker run -d \
  -p 9000:8080 \
  -e PORT=8080 \
  -v file-receive-data:/uploads \
  --name file-receive \
  file-receive
```

**使用自定义上传目录：**
```bash
docker run -d \
  -p 8080:8080 \
  -e UPLOAD_DIR=/data \
  -v /path/on/host:/data \
  --name file-receive \
  file-receive
```

### 查看日志

```bash
docker logs -f file-receive
```

### 停止和删除容器

```bash
# 停止容器
docker stop file-receive

# 删除容器
docker rm file-receive

# 删除镜像
docker rmi file-receive
```

### 使用 docker-compose

创建 `docker-compose.yml`：

```yaml
version: '3.8'

services:
  file-receive:
    build: .
    image: file-receive
    container_name: file-receive
    ports:
      - "8080:8080"
    volumes:
      - file-receive-data:/uploads
    environment:
      - PORT=8080
      - UPLOAD_DIR=/uploads
    restart: unless-stopped

volumes:
  file-receive-data:
```

启动服务：

```bash
docker-compose up -d
```

查看日志：

```bash
docker-compose logs -f
```

停止服务：

```bash
docker-compose down
```
