# 阶段 1: 构建
FROM golang:1.24-alpine AS builder

WORKDIR /build

# 安装构建依赖
RUN apk add --no-cache git

# 复制 go.mod 和 go.sum（如果有）
COPY go.mod go.sum* ./
RUN go mod download

# 复制源码
COPY *.go ./
COPY web/ ./web/

# 构建二进制文件（静态链接）
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags '-w -s' -o smart-cat .

# 阶段 2: 运行
FROM alpine:latest

# 安装 smartmontools 和 ca-certificates
RUN apk add --no-cache smartmontools ca-certificates tzdata

# 创建非 root 用户（但实际运行需要 root 权限访问设备）
RUN addgroup -g 1000 smartcat && \
    adduser -D -u 1000 -G smartcat smartcat

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/smart-cat .

# 创建数据目录
RUN mkdir -p /app/data && chown -R smartcat:smartcat /app

# 暴露端口
EXPOSE 10044

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:10044/ || exit 1

# 注意：虽然我们创建了 smartcat 用户，但由于需要访问 /dev/sdX，
# 实际运行时容器需要 privileged 模式或挂载设备，因此需要 root 权限
# USER smartcat

ENTRYPOINT ["/app/smart-cat"]
