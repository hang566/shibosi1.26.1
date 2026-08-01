# SearchEngine 搜索引擎服务

## 一、原理概述

### 1.1 架构设计

SearchEngine 是一个基于 Go 语言实现的多引擎聚合搜索引擎服务，采用以下核心架构：

```
┌─────────────────────────────────────────────────────────┐
│                    HTTP Server (:8081)                   │
│  ┌───────────────────────────────────────────────────┐  │
│  │              请求路由与 API 处理                    │  │
│  └───────────────────────────────────────────────────┘  │
│                          │                               │
│  ┌───────────────────────▼───────────────────────────┐  │
│  │            MultiSearcher 核心搜索引擎              │  │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ │  │
│  │  │ Google  │ │  Bing   │ │  Baidu  │ │DuckDuckGo│ │  │
│  │  └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘ │  │
│  │       │           │           │           │       │  │
│  │  ┌────▼───────────▼───────────▼───────────▼────┐  │  │
│  │  │         并发搜索 + 结果合并去重               │  │  │
│  │  └─────────────────────────────────────────────┘  │  │
│  │                          │                        │  │
│  │  ┌──────────────────────▼─────────────────────┐  │  │
│  │  │        加权评分 + 内存缓存 (2分钟TTL)        │  │  │
│  │  └─────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────┘  │
│                          │                               │
│  ┌───────────────────────▼───────────────────────────┐  │
│  │           SQLite 数据持久化 (search_history.db)    │  │
│  │  · 搜索历史 · 收藏 · 关键词反馈 · 收录站点         │  │
│  │  · 搜索统计 · 自定义搜索引擎 · 搜索会话           │  │
│  └───────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

### 1.2 核心工作流程

1. **请求接收**：通过 HTTP GET `/api/search?q=关键词` 接收搜索请求
2. **参数验证**：检查关键词非空、长度限制（≤200字符）、XSS 防护
3. **自然语言解析**：识别自然语言指令（如"排除CSDN"、"只要PDF"）
4. **并发搜索**：同时向 5 个搜索引擎发起请求（Google/Bing/Baidu/Sogou/DuckDuckGo）
5. **HTML 解析**：使用 goquery 解析各引擎的 HTML 结果页面
6. **结果合并**：按 URL 去重，合并多引擎来源，计算加权评分
7. **动态分面**：生成来源/域名/品牌/文件类型等筛选维度
8. **筛选应用**：根据前端参数过滤结果
9. **数据库存储**：保存搜索历史和结果
10. **响应返回**：返回格式化的 JSON 结果

### 1.3 评分算法

搜索结果按以下权重计算评分：

| 评分因素 | 权重 | 说明 |
|---------|------|------|
| 引擎权重 | 0.4 | Google(1.0) > Bing(0.9) > Baidu(0.85) > DuckDuckGo(0.8) > Sogou(0.7) |
| 标题匹配 | 0.3 | 标题包含关键词加分 |
| 完全匹配 | +0.1 | 标题完全匹配关键词额外加分 |
| 摘要匹配 | 0.2 | 摘要包含关键词加分 |
| 可信域名 | 0.1 | 维基/GitHub/知乎等可信站点加分 |
| HTTPS | 0.05 | 安全连接加分 |
| 多引擎收录 | 0.25/来源 | 每多一个来源额外加分 |

### 1.4 功能模块

| 模块 | 说明 |
|------|------|
| 多引擎搜索 | 并发调用 5 个搜索引擎，结果合并去重 |
| 动态分面 | 根据结果自动生成筛选维度（来源/域名/品牌等） |
| 自然语言指令 | 支持"排除CSDN"、"只要PDF"等中文指令 |
| 关键词反馈 | 敏感词（如自杀）显示心理援助信息 |
| 收录站点 | 用户提交站点，管理员审核后可被搜索 |
| 收藏功能 | 保存感兴趣的搜索结果 |
| 搜索历史 | 记录所有搜索行为 |
| 自定义引擎 | 用户可添加自定义搜索引擎 |
| 搜索会话 | 保存完整搜索结果供后续查看 |
| 站点分组 | 预设技术社区/学术资源等分组 |

---

## 二、服务依赖

### 2.1 运行环境

| 依赖 | 版本要求 | 说明 |
|------|---------|------|
| Go | ≥ 1.25.0 | 编程语言运行时 |
| 操作系统 | Windows / Linux / macOS | 跨平台支持 |

### 2.2 Go 模块依赖

```
module searchengine
go 1.25.0

require github.com/PuerkitoBio/goquery v1.12.0
require modernc.org/sqlite v1.54.0 (indirect)
require golang.org/x/net v0.52.0 (indirect)
require golang.org/x/sys v0.46.0 (indirect)
```

### 2.3 依赖说明

| 依赖包 | 用途 |
|--------|------|
| `github.com/PuerkitoBio/goquery` | HTML 文档解析，用于解析各搜索引擎返回的 HTML 结果 |
| `modernc.org/sqlite` | SQLite 数据库驱动，纯 Go 实现无需 CGO |
| `database/sql` (标准库) | 数据库操作接口 |
| `net/http` (标准库) | HTTP 服务器与客户端 |
| `encoding/json` (标准库) | JSON 序列化/反序列化 |
| `sync` (标准库) | 并发控制（WaitGroup、RWMutex） |
| `time` (标准库) | 时间处理、缓存 TTL |

### 2.4 文件结构

```
Service/SearchEngine/
├── main.go          # 程序入口 + API 路由
├── search.go        # 搜索引擎核心逻辑
├── storage.go       # SQLite 数据库操作
├── filter.go        # 筛选器与分面引擎
├── cache.go         # 内存缓存实现
├── logger.go        # 日志记录器
├── index.html       # 前端页面（内嵌 CSS/JS）
├── go.mod           # Go 模块定义
├── go.sum           # 依赖校验
├── searchengine.exe # Windows 可执行文件
├── search_history.db# SQLite 数据库文件（运行时生成）
└── logs/            # 日志目录
    └── search_*.log # 每日生成的日志文件
```

### 2.5 数据库表结构

| 表名 | 说明 |
|------|------|
| `search_history` | 搜索历史记录 |
| `search_results` | 搜索结果详情 |
| `favorites` | 收藏夹 |
| `keyword_feedback` | 关键词反馈（含默认敏感词配置） |
| `indexed_sites` | 收录站点 |
| `search_stats` | 搜索统计 |
| `custom_engines` | 自定义搜索引擎 |
| `search_sessions` | 搜索会话（保存完整结果） |

---

## 三、启动代码

### 3.1 程序入口（main.go）

```go
package main

func main() {
    // 注册 API 路由
    http.HandleFunc("/api/search", searchHandler)
    http.HandleFunc("/api/history", historyHandler)
    http.HandleFunc("/api/health", healthHandler)
    // ... 更多路由

    // 启动 HTTP 服务
    fmt.Println("Search engine service starting...")
    fmt.Println("Access address: http://localhost:8081")
    err := http.ListenAndServe(":8081", nil)
    if err != nil {
        fmt.Printf("Service start failed: %v\n", err)
    }
}
```

### 3.2 初始化流程

```go
func init() {
    // 1. 创建多引擎搜索器
    searcher = NewMultiSearcher(
        &DuckDuckGo{},
        &Bing{},
        &Baidu{},
        &Sogou{},
        &Google{},
    )

    // 2. 读取前端页面
    indexPage, err = ioutil.ReadFile("index.html")

    // 3. 初始化数据库
    err = initStorage()
}
```

### 3.3 启动服务

#### 方式一：直接运行二进制文件

**Windows:**
```bash
cd Service/SearchEngine
.\searchengine.exe
```

**Linux/macOS:**
```bash
cd Service/SearchEngine
./searchengine
```

#### 方式二：从源码运行

```bash
cd Service/SearchEngine
go run .
```

#### 方式三：编译后运行

```bash
cd Service/SearchEngine
go build -o searchengine .
./searchengine
```

### 3.4 常用 API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/` | GET | 前端页面 |
| `/api/search?q=关键词` | GET | 执行搜索 |
| `/api/history` | GET | 获取搜索历史 |
| `/api/history/clear` | POST | 清空搜索历史 |
| `/api/health` | GET | 健康检查 |
| `/api/favorites` | GET/POST | 收藏列表/添加收藏 |
| `/api/favorites/{id}` | DELETE | 删除收藏 |
| `/api/keyword-feedback` | GET/POST/PUT | 关键词反馈管理 |
| `/api/indexed-sites` | GET/POST/PUT | 收录站点管理 |
| `/api/search-stats` | GET | 搜索统计 |
| `/api/search-sessions` | GET/POST | 搜索会话管理 |

---

## 四、不同系统的迁移方法

### 4.1 Windows 系统

#### 要求
- Windows 10/11
- Go 1.25+（如需从源码构建）

#### 快速部署
```powershell
# 方式一：使用已编译的 exe
cd F:\projects\市舶司 1.x\市舶司 1.26.1\Service\SearchEngine
.\searchengine.exe

# 方式二：从源码构建
cd F:\projects\市舶司 1.x\市舶司 1.26.1\Service\SearchEngine
go build -o searchengine.exe .
.\searchengine.exe
```

#### 后台运行（可选）
```powershell
# 使用 start 命令后台运行
start /b searchengine.exe

# 或创建 Windows 服务（需 NSSM 工具）
# nssm install SearchEngine "C:\path\to\searchengine.exe"
# nssm start SearchEngine
```

### 4.2 Linux 系统

#### 要求
- Linux kernel 3.10+
- Go 1.25+（如需从源码构建）

#### 快速部署
```bash
# 安装 Go（如未安装）
# Ubuntu/Debian:
sudo apt update && sudo apt install -y golang-go

# CentOS/RHEL:
sudo yum install -y golang

# 从源码构建
cd /path/to/Service/SearchEngine
go build -o searchengine .

# 运行服务
./searchengine
```

#### 后台运行
```bash
# 方式一：使用 nohup
nohup ./searchengine > /dev/null 2>&1 &

# 方式二：创建 systemd 服务
# /etc/systemd/system/searchengine.service
[Unit]
Description=SearchEngine Service
After=network.target

[Service]
Type=simple
ExecStart=/path/to/searchengine
WorkingDirectory=/path/to/Service/SearchEngine
Restart=always

[Install]
WantedBy=multi-user.target

# 启动服务
sudo systemctl enable searchengine
sudo systemctl start searchengine
```

### 4.3 macOS 系统

#### 要求
- macOS 10.13+
- Go 1.25+（如需从源码构建）

#### 快速部署
```bash
# 安装 Go（如未安装）
# 使用 Homebrew:
brew install go

# 从源码构建
cd /path/to/Service/SearchEngine
go build -o searchengine .

# 运行服务
./searchengine
```

#### 后台运行
```bash
# 使用 launchd 创建服务
# ~/Library/LaunchAgents/com.searchengine.service.plist
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.searchengine.service</string>
    <key>ProgramArguments</key>
    <array>
        <string>/path/to/searchengine</string>
    </array>
    <key>WorkingDirectory</key>
    <string>/path/to/Service/SearchEngine</string>
    <key>RunAtLoad</key>
    <true/>
</dict>
</plist>

# 加载服务
launchctl load ~/Library/LaunchAgents/com.searchengine.service.plist
```

### 4.4 跨平台编译

使用 Go 的交叉编译能力，可在一个平台上编译出其他平台的可执行文件：

```bash
# 在任意平台编译 Linux 版本
cd Service/SearchEngine
GOOS=linux GOARCH=amd64 go build -o searchengine_linux .

# 编译 Linux ARM 版本（树莓派等）
GOOS=linux GOARCH=arm64 go build -o searchengine_linux_arm64 .

# 编译 Windows 版本
GOOS=windows GOARCH=amd64 go build -o searchengine_windows.exe .

# 编译 macOS 版本
GOOS=darwin GOARCH=amd64 go build -o searchengine_macos .
GOOS=darwin GOARCH=arm64 go build -o searchengine_macos_arm64 .
```

#### 目标平台参数

| 目标平台 | GOOS | GOARCH |
|---------|------|--------|
| Windows amd64 | windows | amd64 |
| Windows arm64 | windows | arm64 |
| Linux amd64 | linux | amd64 |
| Linux arm64 | linux | arm64 |
| Linux armv7 | linux | arm |
| macOS Intel | darwin | amd64 |
| macOS Apple Silicon | darwin | arm64 |

### 4.5 Docker 部署（可选）

创建 Dockerfile：

```dockerfile
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o searchengine .

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/searchengine .
COPY --from=builder /app/index.html .

EXPOSE 8081

CMD ["./searchengine"]
```

构建并运行：

```bash
# 构建镜像
docker build -t searchengine .

# 运行容器
docker run -d -p 8081:8081 \
  -v /path/to/data:/app \
  --name search-engine \
  searchengine

# 查看日志
docker logs -f search-engine
```

### 4.6 迁移注意事项

1. **数据库兼容**：`search_history.db` 是 SQLite 文件，可跨平台直接复制使用
2. **日志路径**：服务运行时会自动创建 `logs/` 目录和 `search_history.db` 文件
3. **端口冲突**：默认使用 8081 端口，如需修改请修改 `main.go` 末尾的端口号
4. **网络访问**：确保目标服务器可以访问外网（调用搜索引擎需要）
5. **防火墙**：需要开放 8081 端口供外部访问
6. **文件权限**：Linux/macOS 下需给可执行文件添加运行权限：`chmod +x searchengine`
7. **大小写敏感**：Linux/macOS 文件系统区分大小写，复制文件时注意保持文件名一致

### 4.7 验证服务

服务启动后，通过以下方式验证：

```bash
# 健康检查
curl http://localhost:8081/api/health

# 测试搜索
curl "http://localhost:8081/api/search?q=test"

# 浏览器访问
# 打开 http://localhost:8081
```

健康检查响应示例：
```json
{
  "status": "ok",
  "cache_total": 15,
  "cache_expired": 3,
  "engines": 5
}
```