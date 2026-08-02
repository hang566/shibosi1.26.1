package main

import (
	"crypto/subtle"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

// 默认 token，仅在未设置环境变量 ADMIN_TOKEN 时使用
const defaultAdminToken = "shibosi-admin-2026"

// adminPort 运维端口（仅监听 127.0.0.1）
const adminPort = "8091"

// adminPage 运维面板 HTML 内容
var adminPage []byte

// adminPagePath 运维面板 HTML 文件路径
var adminPagePath string

// initAdminPage 加载运维面板 HTML
func initAdminPage() {
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	adminPath := filepath.Join(exeDir, "admin.html")
	adminPagePath = adminPath
	data, err := ioutil.ReadFile(adminPath)
	if err != nil {
		wd, _ := os.Getwd()
		candidates := []string{
			filepath.Join(wd, "admin.html"),
			filepath.Join(wd, "Service", "SearchEngine", "admin.html"),
			filepath.Join(projectRoot, "Service", "SearchEngine", "admin.html"),
		}
		for _, candidate := range candidates {
			data, err = ioutil.ReadFile(candidate)
			if err == nil {
				adminPagePath = candidate
				break
			}
		}
	}
	if err != nil {
		fmt.Printf("Warning: cannot read admin.html: %v\n", err)
	} else {
		adminPage = data
		fmt.Printf("Loaded admin page from: %s\n", adminPagePath)
	}
}

// getAdminToken 读取运维 token
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

// startAdminServer 启动运维面板服务（仅监听 127.0.0.1:8091）
func startAdminServer() {
	initAdminPage()

	mux := http.NewServeMux()

	// 运维面板 UI
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		data, err := ioutil.ReadFile(adminPagePath)
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
	mux.HandleFunc("/api/admin/cache/clear", adminAuthMiddleware(adminCacheClearHandler))
	mux.HandleFunc("/api/admin/database", adminAuthMiddleware(adminDatabaseHandler))
	mux.HandleFunc("/api/admin/history", adminAuthMiddleware(adminHistoryHandler))
	mux.HandleFunc("/api/admin/sessions", adminAuthMiddleware(adminSessionsHandler))
	mux.HandleFunc("/api/admin/stats", adminAuthMiddleware(adminStatsHandler))
	mux.HandleFunc("/api/admin/indexed-sites", adminAuthMiddleware(adminIndexedSitesHandler))
	mux.HandleFunc("/api/admin/indexed-sites/", adminAuthMiddleware(adminIndexedSitesHandler))
	mux.HandleFunc("/api/admin/engines", adminAuthMiddleware(adminEnginesHandler))
	mux.HandleFunc("/api/admin/engines/toggle", adminAuthMiddleware(adminEngineToggleHandler))
	// 关键词反馈管理
	mux.HandleFunc("/api/admin/keyword-feedback", adminAuthMiddleware(adminKeywordFeedbackHandler))
	// 自定义搜索源管理
	mux.HandleFunc("/api/admin/custom-engines", adminAuthMiddleware(adminCustomEnginesHandler))
	// 爬虫管理
	mux.HandleFunc("/api/admin/crawler/status", adminAuthMiddleware(adminCrawlerStatusHandler))
	mux.HandleFunc("/api/admin/crawler/start", adminAuthMiddleware(adminCrawlerStartHandler))
	mux.HandleFunc("/api/admin/crawler/stop", adminAuthMiddleware(adminCrawlerStopHandler))
	mux.HandleFunc("/api/admin/crawler/pages", adminAuthMiddleware(adminCrawlerPagesHandler))
	mux.HandleFunc("/api/admin/crawler/pages/", adminAuthMiddleware(adminCrawlerPagesHandler))
	mux.HandleFunc("/api/admin/crawler/seeds", adminAuthMiddleware(adminCrawlerSeedsHandler))
	mux.HandleFunc("/api/admin/crawler/seeds/", adminAuthMiddleware(adminCrawlerSeedsHandler))

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
	fmt.Printf("  SearchEngine 运维面板（受限访问）\n")
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
