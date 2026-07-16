# 文件接收服务

一个基于 Go + Gin 的简单 Web 文件接收服务，同时提供后端接口和静态 Web 界面。

## 功能特性

- 单文件上传接口
- 静态文件服务
- 带进度的文件上传接口
- SSE (Server-Sent Events) 实时进度推送
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

### 4. SSE 实时进度推送

**接口：** `GET /api/progress/sse`

**响应类型：** `text/event-stream`

**说明：** 使用 SSE 技术实时推送上传进度。当文件上传到 `/upload` 接口时，此接口会实时推送进度信息。

**事件类型：**
- `progress`: 上传进度更新
- `done`: 上传完成

**SSE 数据格式：**

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
const eventSource = new EventSource('http://localhost:8080/api/progress/sse');

eventSource.addEventListener('progress', (event) => {
  const data = JSON.parse(event.data);
  console.log(`上传进度: ${data.percent.toFixed(2)}%`);
});

eventSource.addEventListener('done', (event) => {
  const data = JSON.parse(event.data);
  console.log('上传完成:', data.filename);
  eventSource.close();
});

eventSource.onerror = (err) => {
  console.error('SSE 错误:', err);
  eventSource.close();
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

#### 监听 SSE 进度

```bash
curl -N http://localhost:8080/api/progress/sse
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

#### 监听 SSE 进度

```powershell
$uri = "http://localhost:8080/api/progress/sse"
$client = New-Object System.Net.Http.HttpClient
$client.Timeout = [System.TimeSpan]::FromMinutes(30)
$response = $client.GetAsync($uri, [System.Net.Http.HttpCompletionOption]::ResponseHeadersRead).Result
$stream = $response.Content.ReadAsStreamAsync().Result
$reader = New-Object System.IO.StreamReader($stream)

while ($null -ne ($line = $reader.ReadLine())) {
    if ($line -match 'data:') {
        $data = $line -replace 'data: ', ''
        Write-Host $data
    }
}
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

#### 监听 SSE 进度

1. 新建 GET 请求到 `http://localhost:8080/api/progress/sse`
2. 发送请求
3. 可以在响应中看到实时更新的进度数据
## 环境变量

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| PORT | 8080 | 服务监听端口 |
| UPLOAD_DIR | ./uploads | 文件保存目录 |

## 目录结构

```
file-receive/
├── main.go         # 主程序文件
├── go.mod          # Go 模块文件
├── go.sum          # 依赖锁定文件
├── static/         # 静态文件目录，包含 index.html 等
├── .gitignore      # Git 忽略文件
└── README.md       # 项目说明文档
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
