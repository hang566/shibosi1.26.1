#!/bin/bash
# ============================================
# 市舶司 admin-core v2 环境初始化脚本
# 支持 CentOS / Ubuntu / Debian
# ============================================

set -e

COLOR_GREEN='\033[0;32m'
COLOR_YELLOW='\033[0;33m'
COLOR_RED='\033[0;31m'
NC='\033[0m'

log_info()  { echo -e "${COLOR_GREEN}[INFO]${NC} $*"; }
log_warn()  { echo -e "${COLOR_YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${COLOR_RED}[ERROR]${NC} $*"; }

detect_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$ID
        OS_VERSION=$VERSION_ID
    elif [ -f /etc/redhat-release ]; then
        OS="centos"
        OS_VERSION=$(rpm -q --qf "%{VERSION}" $(rpm -q --whatprovides redhat-release) 2>/dev/null || echo 7)
    else
        OS="unknown"
    fi
    log_info "检测到系统: $OS $OS_VERSION"
}

install_base() {
    log_info "安装基础工具..."
    case $OS in
        ubuntu|debian)
            apt-get update -y
            apt-get install -y curl wget vim unzip tar gzip net-tools iptables sudo rsync
            ;;
        centos|rhel|amzn)
            yum install -y curl wget vim unzip tar gzip net-tools iptables sudo rsync
            ;;
        *)
            log_warn "未知系统，请手动安装基础依赖"
            return 1
            ;;
    esac
}

install_runtime() {
    log_info "安装 Go / Redis / MySQL 等运行时..."
    case $OS in
        ubuntu|debian)
            apt-get install -y redis-server mysql-server nginx
            ;;
        centos|rhel|amzn)
            yum install -y redis mariadb-server nginx
            ;;
    esac
    log_info "运行时组件已安装"
}

setup_firewall() {
    log_info "配置防火墙..."
    if command -v firewall-cmd >/dev/null 2>&1; then
        systemctl enable firewalld --now 2>/dev/null || true
        firewall-cmd --permanent --add-port=8080/tcp
        firewall-cmd --permanent --add-port=8081/tcp
        firewall-cmd --permanent --add-port=8082/tcp
        firewall-cmd --permanent --add-port=8083/tcp
        firewall-cmd --permanent --add-port=8084/tcp
        firewall-cmd --permanent --add-port=22/tcp
        firewall-cmd --reload
        log_info "firewalld 规则已写入"
    elif command -v iptables >/dev/null 2>&1; then
        for port in 22 8080 8081 8082 8083 8084; do
            iptables -I INPUT -p tcp --dport $port -j ACCEPT
        done
        iptables-save > /etc/iptables.rules 2>/dev/null || true
        log_info "iptables 规则已写入"
    fi
}

setup_ssh_security() {
    log_info "配置 SSH 安全..."
    # 禁用 root 密码登录
    if [ -f /etc/ssh/sshd_config ]; then
        cp /etc/ssh/sshd_config /etc/ssh/sshd_config.bak.$(date +%Y%m%d)
        sed -i 's/#PermitRootLogin yes/PermitRootLogin no/' /etc/ssh/sshd_config
        sed -i 's/PermitRootLogin yes/PermitRootLogin no/' /etc/ssh/sshd_config
        # 禁用密码登录，强制密钥
        sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config
        # 限制最大尝试次数
        sed -i 's/#MaxAuthTries 6/MaxAuthTries 3/' /etc/ssh/sshd_config
        log_info "SSH 安全加固完成"
        if command -v sshd >/dev/null 2>&1; then
            systemctl restart sshd 2>/dev/null || systemctl restart ssh 2>/dev/null || true
        fi
    fi
}

install_go() {
    if command -v go >/dev/null 2>&1; then
        log_info "Go 已安装: $(go version)"
        return
    fi
    log_info "安装 Go 1.22..."
    GO_VERSION="1.22.5"
    case $(uname -m) in
        x86_64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        armv7l) ARCH="armv6l" ;;
        *) ARCH="amd64" ;;
    esac
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" -o /tmp/go.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' > /etc/profile.d/go.sh
    chmod +x /etc/profile.d/go.sh
    export PATH=$PATH:/usr/local/go/bin
    go version
    log_info "Go 已安装"
}

init_directories() {
    log_info "创建目录结构..."
    PROJECT_ROOT="${PROJECT_ROOT:-/opt/shibosi}"
    mkdir -p "$PROJECT_ROOT"/{admin-core/{data,logs,workspace/{wwwroot,backup,packages,scripts,logs}},services,backups}
    chown -R root:root "$PROJECT_ROOT"
    chmod -R 755 "$PROJECT_ROOT"
    log_info "项目目录: $PROJECT_ROOT"
    echo "PROJECT_ROOT=$PROJECT_ROOT" > /etc/shibosi.env
}

setup_crontab() {
    log_info "配置基础 Cron 任务..."
    # 每日 3 点自动备份数据库
    if crontab -l 2>/dev/null | grep -q "shibosi-backup"; then
        log_info "备份任务已存在，跳过"
    else
        (crontab -l 2>/dev/null; echo "0 3 * * * /opt/shibosi/admin-core/scripts/backup-db.sh >> /var/log/shibosi-backup.log 2>&1 # shibosi-backup") | crontab -
        log_info "数据库备份任务已添加（每日 3 点）"
    fi
}

install_admin_core() {
    log_info "编译 admin-core..."
    cd "${PROJECT_ROOT:-/opt/shibosi}/admin-core"
    if [ -f go.mod ]; then
        go mod tidy
        go build -o admin-core ./...
        if [ -f admin-core ]; then
            log_info "admin-core 编译成功"
        fi
    fi
}

setup_systemd() {
    log_info "配置 systemd 服务..."
    cat > /etc/systemd/system/shibosi-admin.service <<EOF
[Unit]
Description=Shibosi Admin Core v2
After=network.target redis.service

[Service]
Type=simple
User=root
WorkingDirectory=${PROJECT_ROOT:-/opt/shibosi}/admin-core
ExecStart=${PROJECT_ROOT:-/opt/shibosi}/admin-core/admin-core
Restart=on-failure
RestartSec=5
Environment=SHIBOSI_ENV=production
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable shibosi-admin 2>/dev/null || true
    log_info "systemd 服务已配置"
}

setup_nginx() {
    if ! command -v nginx >/dev/null 2>&1; then
        return
    fi
    log_info "配置 Nginx 反代..."
    cat > /etc/nginx/conf.d/shibosi-admin.conf <<EOF
server {
    listen 8443 ssl http2;
    server_name _;

    ssl_certificate     /etc/nginx/ssl/admin.crt;
    ssl_certificate_key /etc/nginx/ssl/admin.key;

    location / {
        proxy_pass http://127.0.0.1:8084;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
    }
}
EOF
    nginx -t 2>/dev/null && log_info "Nginx 配置有效" || log_warn "Nginx 配置校验失败（HTTPS 证书可能未生成）"
}

show_menu() {
    cat <<EOF
╔══════════════════════════════════════════════════════╗
║     市舶司 Admin Core v2 - 环境初始化                ║
║     宝塔对标 高安全 易运维                           ║
╠══════════════════════════════════════════════════════╣
║  1) 全自动初始化（推荐）                              ║
║  2) 安装基础工具                                     ║
║  3) 安装运行时（Redis / MySQL / Nginx）              ║
║  4) 安装 Go 编译环境                                  ║
║  5) 配置防火墙（放行 8080-8084）                     ║
║  6) SSH 安全加固                                      ║
║  7) 创建目录结构                                     ║
║  8) 编译 admin-core                                  ║
║  9) 配置 systemd + Nginx 反代                        ║
║  0) 全部执行（1+2+3+4+5+6+7+8+9）                    ║
║  q) 退出                                             ║
╚══════════════════════════════════════════════════════╝
EOF
}

main() {
    echo
    log_info "市舶司 Admin Core v2 环境初始化脚本"
    detect_os
    echo

    if [ "$1" == "--all" ]; then
        log_info "全自动模式"
        install_base
        install_runtime
        install_go
        setup_firewall
        setup_ssh_security
        init_directories
        install_admin_core
        setup_systemd
        setup_nginx
        setup_crontab
        log_info "全部初始化完成 🎉"
        return
    fi

    show_menu
    read -p "请选择: " choice
    case $choice in
        1|0)
            install_base; install_runtime; install_go
            setup_firewall; setup_ssh_security
            init_directories; install_admin_core
            setup_systemd; setup_nginx; setup_crontab
            ;;
        2) install_base ;;
        3) install_runtime ;;
        4) install_go ;;
        5) setup_firewall ;;
        6) setup_ssh_security ;;
        7) init_directories ;;
        8) install_admin_core ;;
        9) setup_systemd; setup_nginx ;;
        q) exit 0 ;;
        *) log_warn "无效选项" ;;
    esac
}

main "$@"
