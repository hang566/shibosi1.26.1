# 构建阶段
FROM golang:1.23-alpine AS builder

# 安装必要的构建工具
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 编译（静态链接，优化二进制大小）
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.Version=2.0.0" \
    -o /build/shibosi-admin .

# 运行阶段
FROM alpine:3.20

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata curl && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/shibosi-admin .

# 复制管理面板静态文件
COPY admin/ ./admin/

# 复制配置文件模板
COPY config/config.yaml ./config/config.yaml

# 创建非root用户
RUN addgroup -g 1000 shibosi && \
    adduser -D -u 1000 -G shibosi shibosi && \
    chown -R shibosi:shibosi /app

USER shibosi

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/api/health || exit 1

# 启动
ENTRYPOINT ["./shibosi-admin"]
CMD ["--config", "config/config.yaml"]