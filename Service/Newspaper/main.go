package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var indexPage []byte
var adminPage []byte
var homePagePath string
var adminPagePath string
var projectRoot string

func init() {
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	projectRoot = filepath.Dir(filepath.Dir(exeDir))

	indexPath := filepath.Join(exeDir, "home.html")
	homePagePath = indexPath
	data, err := ioutil.ReadFile(indexPath)
	if err != nil {
		wd, _ := os.Getwd()
		candidates := []string{
			filepath.Join(wd, "home.html"),
			filepath.Join(wd, "Service", "Newspaper", "home.html"),
			filepath.Join(projectRoot, "Service", "Newspaper", "home.html"),
		}
		for _, candidate := range candidates {
			data, err = ioutil.ReadFile(candidate)
			if err == nil {
				homePagePath = candidate
				break
			}
		}
	}
	if err != nil {
		fmt.Printf("Warning: cannot read home.html: %v\n", err)
	} else {
		indexPage = data
		fmt.Printf("Loaded page from: %s\n", homePagePath)
	}

	adminPath := filepath.Join(exeDir, "admin.html")
	adminPagePath = adminPath
	adminData, err := ioutil.ReadFile(adminPath)
	if err != nil {
		wd, _ := os.Getwd()
		candidates := []string{
			filepath.Join(wd, "admin.html"),
			filepath.Join(wd, "Service", "Newspaper", "admin.html"),
			filepath.Join(projectRoot, "Service", "Newspaper", "admin.html"),
		}
		for _, candidate := range candidates {
			adminData, err = ioutil.ReadFile(candidate)
			if err == nil {
				adminPagePath = candidate
				break
			}
		}
	}
	if err != nil {
		fmt.Printf("Warning: cannot read admin.html: %v\n", err)
	} else {
		adminPage = adminData
		fmt.Printf("Loaded admin page from: %s\n", adminPath)
	}

	if err := initStorage(); err != nil {
		fmt.Printf("Warning: cannot initialize storage: %v\n", err)
	}

	if err := InitLogger(); err != nil {
		fmt.Printf("Warning: cannot initialize logger: %v\n", err)
	}
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	http.Error(w, msg, code)
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	data, err := ioutil.ReadFile(homePagePath)
	if err != nil {
		fmt.Printf("Warning: cannot read home.html: %v\n", err)
		w.Write(indexPage)
		return
	}
	w.Write(data)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	stats, _ := GetNewsStats()
	health := map[string]interface{}{
		"status": "ok",
		"stats":  stats,
	}
	writeJSON(w, health)
}

func newsListHandler(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	category := r.URL.Query().Get("category")
	keyword := r.URL.Query().Get("keyword")
	dateFrom := r.URL.Query().Get("date_from")
	dateTo := r.URL.Query().Get("date_to")

	resp, err := GetNews(page, perPage, category, keyword, dateFrom, dateTo)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, resp)
}

func newsDetailHandler(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	var idStr string
	for i, p := range pathParts {
		if p == "news" && i+1 < len(pathParts) {
			idStr = pathParts[i+1]
			break
		}
	}

	if idStr == "" {
		writeError(w, "缺少新闻ID", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, "无效的新闻ID", http.StatusBadRequest)
		return
	}

	news, err := GetNewsByID(id)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if news == nil {
		writeError(w, "新闻不存在", http.StatusNotFound)
		return
	}

	writeJSON(w, news)
}

func newsPermanentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID          int  `json:"id"`
		IsPermanent bool `json:"is_permanent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "无效的请求", http.StatusBadRequest)
		return
	}

	if req.ID <= 0 {
		writeError(w, "无效的新闻ID", http.StatusBadRequest)
		return
	}

	if err := SetNewsPermanent(req.ID, req.IsPermanent); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	statusText := "已标记为永久保存"
	if !req.IsPermanent {
		statusText = "已取消永久保存"
	}

	writeJSON(w, map[string]interface{}{
		"success":      true,
		"is_permanent": req.IsPermanent,
		"message":      statusText,
	})
}

func searchNewsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	categories := r.URL.Query().Get("categories")
	sources := r.URL.Query().Get("sources")
	dateFrom := r.URL.Query().Get("date_from")
	dateTo := r.URL.Query().Get("date_to")

	if query == "" {
		writeError(w, "搜索关键词不能为空", http.StatusBadRequest)
		return
	}

	var catList, srcList []string
	if categories != "" {
		catList = strings.Split(categories, ",")
	}
	if sources != "" {
		srcList = strings.Split(sources, ",")
	}

	newsList, err := SearchNewsAdvanced(query, catList, srcList, dateFrom, dateTo)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"total": len(newsList),
		"news":  newsList,
	})
}

func categoriesHandler(w http.ResponseWriter, r *http.Request) {
	categories, err := GetCategories()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, categories)
}

func sourcesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sources, err := GetSources()
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, sources)

	case http.MethodPost:
		var src NewsSource
		if err := json.NewDecoder(r.Body).Decode(&src); err != nil {
			writeError(w, "无效的请求", http.StatusBadRequest)
			return
		}
		id, err := AddSource(src)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "id": id})

	case http.MethodPut:
		var src NewsSource
		if err := json.NewDecoder(r.Body).Decode(&src); err != nil {
			writeError(w, "无效的请求", http.StatusBadRequest)
			return
		}
		if err := UpdateSource(src); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true})

	case http.MethodDelete:
		var req struct {
			ID int `json:"id"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.ID == 0 {
			pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			for i, p := range pathParts {
				if p == "sources" && i+1 < len(pathParts) {
					req.ID, _ = strconv.Atoi(pathParts[i+1])
					break
				}
			}
		}
		if req.ID == 0 {
			writeError(w, "缺少源ID", http.StatusBadRequest)
			return
		}
		if err := DeleteSource(req.ID); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true})

	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func fetchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sourceID := r.URL.Query().Get("source_id")

	if sourceID != "" {
		id, err := strconv.Atoi(sourceID)
		if err != nil {
			writeError(w, "无效的源ID", http.StatusBadRequest)
			return
		}
		saved, err := FetchAndSaveSource(id)
		if err != nil {
			errMsg := err.Error()
			if logger != nil {
				logger.Error("handler", "抓取单个源失败: %s", errMsg)
			}
			writeJSON(w, map[string]interface{}{
				"success": false,
				"saved":   0,
				"message": fmt.Sprintf("抓取失败: %s", errMsg),
				"error":   errMsg,
			})
			return
		}
		writeJSON(w, map[string]interface{}{
			"success": true,
			"saved":   saved,
			"message": fmt.Sprintf("成功抓取并保存 %d 条新闻", saved),
		})
		return
	}

	saved, err := FetchAndSaveAll()
	if err != nil {
		errMsg := err.Error()
		if logger != nil {
			logger.Error("handler", "抓取全部新闻源失败: %s", errMsg)
		}
		writeJSON(w, map[string]interface{}{
			"success": false,
			"saved":   0,
			"message": fmt.Sprintf("抓取失败: %s", errMsg),
			"error":   errMsg,
		})
		return
	}

	if saved == 0 {
		writeJSON(w, map[string]interface{}{
			"success": true,
			"saved":   0,
			"message": "没有获取到新的新闻，可能所有新闻源都没有更新",
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
		"saved":   saved,
		"message": fmt.Sprintf("成功抓取并保存 %d 条新闻", saved),
	})
}

func fetchLogsHandler(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	logs, err := GetFetchLogs(limit)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, logs)
}

func exportHandler(w http.ResponseWriter, r *http.Request) {
	var opts ExportOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		writeError(w, "无效的导出请求", http.StatusBadRequest)
		return
	}

	filename, content, err := ExportNews(opts)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filePath, _ := SaveExportToFile(filename, content)

	ext := filepath.Ext(filename)
	contentType := "application/octet-stream"
	switch ext {
	case ".md":
		contentType = "text/markdown; charset=utf-8"
	case ".json":
		contentType = "application/json; charset=utf-8"
	case ".html":
		contentType = "text/html; charset=utf-8"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("X-Export-Path", filePath)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(content))
}

func exportFileHandler(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		writeError(w, "缺少文件名", http.StatusBadRequest)
		return
	}

	exportDir := filepath.Join("..", "..", "db", "exports")
	filePath := filepath.Join(exportDir, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		writeError(w, "文件不存在", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	http.ServeFile(w, r, filePath)
}

func settingsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		settings, err := GetSettings()
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, settings)

	case http.MethodPost:
		var req struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, "无效的请求", http.StatusBadRequest)
			return
		}
		if err := UpdateSetting(req.Key, req.Value); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true})

	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func cleanHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	daysStr := r.URL.Query().Get("days")
	days, _ := strconv.Atoi(daysStr)
	if days <= 0 {
		days = 30
	}

	deleted, err := ClearOldNews(days)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
		"deleted": deleted,
		"message": fmt.Sprintf("已清理 %d 天前的 %d 条新闻", days, deleted),
	})
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	stats, err := GetNewsStats()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, stats)
}

func reanalyzeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 500
	count, err := ReanalyzeNews(limit)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
		"count":   count,
	})
}

func analysisStatsHandler(w http.ResponseWriter, r *http.Request) {
	stats, err := GetAnalysisStats()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, stats)
}

func adminPageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	data, err := ioutil.ReadFile(adminPagePath)
	if err != nil {
		fmt.Printf("Warning: cannot read admin.html: %v\n", err)
		if adminPage != nil {
			w.Write(adminPage)
			return
		}
		http.Error(w, "运维面板页面未加载", http.StatusInternalServerError)
		return
	}
	w.Write(data)
}

func favoritesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		if page < 1 {
			page = 1
		}
		if perPage <= 0 {
			perPage = 50
		}

		favs, total, err := GetFavorites(page, perPage)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]interface{}{
			"total":     total,
			"page":      page,
			"per_page":  perPage,
			"favorites": favs,
		})

	case http.MethodPost:
		var fav NewsFavorite
		if err := json.NewDecoder(r.Body).Decode(&fav); err != nil {
			writeError(w, "无效的请求", http.StatusBadRequest)
			return
		}

		if fav.URL == "" || fav.Title == "" {
			writeError(w, "缺少必要字段", http.StatusBadRequest)
			return
		}

		id, err := AddFavorite(fav)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]interface{}{
			"success": true,
			"id":      id,
			"message": "已添加到收藏",
		})

	case http.MethodDelete:
		var req struct {
			ID  int    `json:"id"`
			URL string `json:"url"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if req.ID > 0 {
			if err := RemoveFavorite(req.ID); err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else if req.URL != "" {
			if err := RemoveFavoriteByURL(req.URL); err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			writeError(w, "缺少收藏ID或URL", http.StatusBadRequest)
			return
		}

		writeJSON(w, map[string]interface{}{"success": true, "message": "已取消收藏"})

	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func favoriteDeleteHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodDelete, http.MethodPost:
		var req struct {
			ID  int    `json:"id"`
			URL string `json:"url"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if req.ID > 0 {
			if err := RemoveFavorite(req.ID); err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else if req.URL != "" {
			if err := RemoveFavoriteByURL(req.URL); err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			for i, p := range pathParts {
				if p == "favorites" && i+1 < len(pathParts) {
					id, _ := strconv.Atoi(pathParts[i+1])
					if id > 0 {
						req.ID = id
					}
					break
				}
			}
			if req.ID > 0 {
				if err := RemoveFavorite(req.ID); err != nil {
					writeError(w, err.Error(), http.StatusInternalServerError)
					return
				}
			} else {
				writeError(w, "缺少收藏ID或URL", http.StatusBadRequest)
				return
			}
		}

		writeJSON(w, map[string]interface{}{"success": true, "message": "已取消收藏"})

	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func favoriteCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	urlParam := r.URL.Query().Get("url")
	if urlParam == "" {
		writeError(w, "缺少URL参数", http.StatusBadRequest)
		return
	}

	isFav, id := IsFavorite(urlParam)
	writeJSON(w, map[string]interface{}{
		"is_favorite": isFav,
		"id":          id,
	})
}

func favoritesClearHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := ClearAllFavorites(); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{"success": true, "message": "已清空所有收藏"})
}

func favoriteCountHandler(w http.ResponseWriter, r *http.Request) {
	count, err := GetFavoriteCount()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"count": count})
}

func searchEngineProxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, "搜索关键词不能为空", http.StatusBadRequest)
		return
	}

	searchEngineURL := "http://localhost:8081/api/search"
	params := url.Values{}
	params.Set("q", query)

	if limit := r.URL.Query().Get("limit"); limit != "" {
		params.Set("limit", limit)
	} else {
		params.Set("limit", "10")
	}

	if page := r.URL.Query().Get("page"); page != "" {
		params.Set("page", page)
	}

	fullURL := searchEngineURL + "?" + params.Encode()

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		writeError(w, "构建请求失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	req.Header.Set("User-Agent", "Newspaper-Service/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"success": false,
			"error":   "SearchEngine 服务不可用，请确保 http://localhost:8081 正在运行",
			"details": err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, "读取响应失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(body)
}

func searchEngineHealthHandler(w http.ResponseWriter, r *http.Request) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://localhost:8081/api/health")
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"available": false,
			"message":   "SearchEngine 服务不可用",
		})
		return
	}
	defer resp.Body.Close()

	writeJSON(w, map[string]interface{}{
		"available": true,
		"message":   "SearchEngine 服务正常",
	})
}

func main() {
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	http.HandleFunc("/", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/home.html" {
			homeHandler(w, r)
			return
		}
		// 运维面板已迁移到独立端口 127.0.0.1:8092/admin
		if r.URL.Path == "/admin" || r.URL.Path == "/admin.html" {
			writeJSON(w, map[string]interface{}{
				"error":     "admin_moved",
				"message":   "运维面板已迁移到独立端口，请访问 http://127.0.0.1:8092/admin",
				"admin_url": "http://127.0.0.1:8092/admin",
			})
			return
		}
		http.NotFound(w, r)
	}))

	http.HandleFunc("/api/health", corsMiddleware(healthHandler))
	http.HandleFunc("/api/news", corsMiddleware(newsListHandler))
	http.HandleFunc("/api/news/", corsMiddleware(newsDetailHandler))
	http.HandleFunc("/api/search", corsMiddleware(searchNewsHandler))
	http.HandleFunc("/api/categories", corsMiddleware(categoriesHandler))
	http.HandleFunc("/api/sources", corsMiddleware(sourcesHandler))
	http.HandleFunc("/api/sources/", corsMiddleware(sourcesHandler))
	http.HandleFunc("/api/fetch", corsMiddleware(fetchHandler))
	http.HandleFunc("/api/fetch-logs", corsMiddleware(fetchLogsHandler))
	http.HandleFunc("/api/export", corsMiddleware(exportHandler))
	http.HandleFunc("/api/export/file", corsMiddleware(exportFileHandler))
	http.HandleFunc("/api/settings", corsMiddleware(settingsHandler))
	http.HandleFunc("/api/clean", corsMiddleware(cleanHandler))
	http.HandleFunc("/api/stats", corsMiddleware(statsHandler))
	http.HandleFunc("/api/reanalyze", corsMiddleware(reanalyzeHandler))
	http.HandleFunc("/api/analysis-stats", corsMiddleware(analysisStatsHandler))

	http.HandleFunc("/api/news/permanent", corsMiddleware(newsPermanentHandler))
	// 运维 API 已迁移到独立端口 127.0.0.1:8092，受 token 保护

	http.HandleFunc("/api/favorites", corsMiddleware(favoritesHandler))
	http.HandleFunc("/api/favorites/", corsMiddleware(favoriteDeleteHandler))
	http.HandleFunc("/api/favorites/check", corsMiddleware(favoriteCheckHandler))
	http.HandleFunc("/api/favorites/clear", corsMiddleware(favoritesClearHandler))
	http.HandleFunc("/api/favorites/count", corsMiddleware(favoriteCountHandler))

	http.HandleFunc("/api/search-engine/search", corsMiddleware(searchEngineProxyHandler))
	http.HandleFunc("/api/search-engine/health", corsMiddleware(searchEngineHealthHandler))

	fs := http.FileServer(http.Dir(exeDir))
	http.HandleFunc("/newspaper.css", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		fs.ServeHTTP(w, r)
	}))
	http.HandleFunc("/app.js", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		fs.ServeHTTP(w, r)
	}))

	port := "8082"
	fmt.Printf("========================================\n")
	fmt.Printf("  市舶司 - 昨日晚报服务\n")
	fmt.Printf("========================================\n")
	fmt.Printf("访问地址: http://localhost:%s\n", port)
	fmt.Printf("健康检查: http://localhost:%s/api/health\n", port)
	fmt.Printf("运维面板: http://127.0.0.1:8092/admin（独立端口+Token）\n")
	fmt.Printf("API列表:\n")
	fmt.Printf("  GET  /api/news          - 获取新闻列表\n")
	fmt.Printf("  GET  /api/news/:id      - 获取新闻详情\n")
	fmt.Printf("  GET  /api/search?q=..   - 搜索新闻\n")
	fmt.Printf("  GET  /api/categories    - 获取分类列表\n")
	fmt.Printf("  GET  /api/sources       - 获取新闻源\n")
	fmt.Printf("  POST /api/fetch         - 抓取新闻\n")
	fmt.Printf("  POST /api/fetch?source_id=X - 抓取指定源\n")
	fmt.Printf("  POST /api/export        - 导出新闻\n")
	fmt.Printf("  GET  /api/stats         - 获取统计信息\n")
	fmt.Printf("  POST /api/clean?days=30 - 清理旧数据\n")
	fmt.Printf("========================================\n")

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      nil,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 启动运维面板（独立端口 127.0.0.1:8092 + Token 认证）
	startAdminServer()

	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("服务启动失败: %v\n", err)
		os.Exit(1)
	}
}
