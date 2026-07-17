# 构建阶段
FROM golang:1.26-alpine AS builder

WORKDIR /app

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 编译
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o file-receive .

# 运行阶段
FROM alpine:3.18

# 安装必要的包
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# 从构建阶段复制编译好的二进制文件
COPY --from=builder /app/file-receive .


# 声明端口
EXPOSE 8080

# 声明上传目录作为卷
VOLUME ["/uploads"]

# 设置环境变量
ENV PORT=8080
ENV UPLOAD_DIR=/uploads

# 运行应用
CMD ["./file-receive"]
