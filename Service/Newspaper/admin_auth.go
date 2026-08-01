package main

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

// 默认 token，仅在未设置环境变量 ADMIN_TOKEN 时使用
const defaultAdminToken = "shibosi-admin-2026"

// adminPort 运维端口（仅监听 127.0.0.1）
const adminPort = "8092"

// getAdminToken 读取运维 token：优先环境变量 ADMIN_TOKEN，否则使用默认值
func getAdminToken() string {
	if t := os.Getenv("ADMIN_TOKEN"); t != "" {
		return t
	}
	return defaultAdminToken
}

// adminAuthMiddleware 校验 X-Admin-Token header 或 ?token=xxx 参数
func adminAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Admin-Token")
			w.WriteHeader(http.StatusOK)
			return
		}

		token := r.Header.Get("X-Admin-Token")
		if token == "" {
			token = r.URL.Query().Get("token")
		}

		expected := getAdminToken()
		if token == "" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("WWW-Authenticate", "X-Admin-Token")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"unauthorized","message":"缺少运维 token"}`)
			return
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("WWW-Authenticate", "X-Admin-Token")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"forbidden","message":"运维 token 错误"}`)
			return
		}

		next(w, r)
	}
}

// startAdminServer 启动运维面板服务（仅监听 127.0.0.1:8092）
func startAdminServer() {
	mux := http.NewServeMux()

	// 运维面板 UI（页面本身不需要 token，但所有 API 调用需要）
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		data, err := os.ReadFile(adminPagePath)
		if err != nil {
			if adminPage != nil {
				w.Write(adminPage)
				return
			}
			http.Error(w, "运维面板页面未加载", http.StatusInternalServerError)
			return
		}
		w.Write(data)
	})
	mux.HandleFunc("/admin.html", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin", http.StatusMovedPermanently)
	})

	// 运维 API（全部需要 token 认证）
	mux.HandleFunc("/api/admin/overview", adminAuthMiddleware(adminOverviewHandler))
	mux.HandleFunc("/api/admin/logs", adminAuthMiddleware(adminLogsHandler))
	mux.HandleFunc("/api/admin/logs/level", adminAuthMiddleware(adminLogLevelHandler))
	mux.HandleFunc("/api/admin/analysis", adminAuthMiddleware(adminAnalysisHandler))
	mux.HandleFunc("/api/admin/fetch", adminAuthMiddleware(adminFetchHandler))
	mux.HandleFunc("/api/admin/database", adminAuthMiddleware(adminDatabaseHandler))
	mux.HandleFunc("/api/admin/sources", adminAuthMiddleware(adminSourcesHandler))
	mux.HandleFunc("/api/admin/sources/", adminAuthMiddleware(adminSourcesHandler))
	mux.HandleFunc("/api/admin/backup", adminAuthMiddleware(adminBackupHandler))

	// 运维面板需要的静态资源
	mux.HandleFunc("/newspaper.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, max-age=0")
		cssPath := filepath.Join(filepath.Dir(adminPagePath), "newspaper.css")
		if data, err := os.ReadFile(cssPath); err == nil {
			w.Write(data)
		}
	})

	addr := "127.0.0.1:" + adminPort
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	token := getAdminToken()
	tokenDisplay := "已通过环境变量 ADMIN_TOKEN 设置"
	if os.Getenv("ADMIN_TOKEN") == "" {
		tokenDisplay = "使用默认值: " + token + "（建议设置环境变量 ADMIN_TOKEN 加强安全）"
	}

	fmt.Printf("========================================\n")
	fmt.Printf("  Newspaper 运维面板（受限访问）\n")
	fmt.Printf("========================================\n")
	fmt.Printf("运维地址: http://%s/admin\n", addr)
	fmt.Printf("Token: %s\n", tokenDisplay)
	fmt.Printf("仅监听 127.0.0.1，外部无法访问\n")
	fmt.Printf("========================================\n")

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			fmt.Printf("运维面板启动失败: %v\n", err)
		}
	}()
}
