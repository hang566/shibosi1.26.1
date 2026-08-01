package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var searcher *MultiSearcher
var indexPage []byte
var projectRoot string

func init() {
	searcher = NewMultiSearcher(
		&DuckDuckGo{},
		&Bing{},
		&Baidu{},
		&Sogou{},
		&Google{},
	)

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	projectRoot = filepath.Dir(filepath.Dir(exeDir))

	seIndex := filepath.Join(exeDir, "index.html")
	data, err := ioutil.ReadFile(seIndex)
	if err != nil {
		wd, _ := os.Getwd()
		candidates := []string{
			filepath.Join(wd, "index.html"),
			filepath.Join(wd, "Service", "SearchEngine", "index.html"),
			filepath.Join(projectRoot, "Service", "SearchEngine", "index.html"),
		}
		for _, candidate := range candidates {
			data, err = ioutil.ReadFile(candidate)
			if err == nil {
				seIndex = candidate
				break
			}
		}
	}
	if err != nil {
		fmt.Printf("Warning: cannot read index.html: %v\n", err)
	} else {
		indexPage = data
		fmt.Printf("Loaded index from: %s\n", seIndex)
	}

	err = initStorage()
	if err != nil {
		fmt.Printf("Warning: cannot initialize storage: %v\n", err)
	}
}

func validateQuery(query string) error {
	query = strings.TrimSpace(query)
	if query == "" {
		return fmt.Errorf("搜索关键词不能为空")
	}
	if len(query) > 200 {
		return fmt.Errorf("搜索关键词过长（最多200字符）")
	}
	dangerous := []string{"<script", "javascript:", "onerror", "onload"}
	lowerQuery := strings.ToLower(query)
	for _, d := range dangerous {
		if strings.Contains(lowerQuery, d) {
			return fmt.Errorf("搜索关键词包含非法字符")
		}
	}
	return nil
}

type SearchResponse struct {
	Query           string           `json:"query"`
	Total           int              `json:"total"`
	Page            int              `json:"page"`
	PerPage         int              `json:"per_page"`
	Results         []SearchResult   `json:"results"`
	KeywordFeedback *KeywordFeedback `json:"keyword_feedback,omitempty"`
	Facets          *FacetResponse   `json:"facets,omitempty"`
	NLCommands      []NLCommand      `json:"nl_commands,omitempty"`
	SiteGroups      []SiteGroup      `json:"site_groups,omitempty"`
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
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

func searchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rawQuery := r.URL.Query().Get("q")
	if err := validateQuery(rawQuery); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 解析自然语言指令
	nlCommands, query := ParseNLCommand(rawQuery)
	nlFilter := NLCommandsToFilterParams(nlCommands)

	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 || limit > 50 {
			limit = 10
		}
	}

	pageStr := r.URL.Query().Get("page")
	page := 1
	if pageStr != "" {
		var err error
		page, err = strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			page = 1
		}
	}

	forceRefresh := r.URL.Query().Get("force") == "1"

	// 解析前端筛选参数
	var fp FilterParams
	if sources := r.URL.Query().Get("sources"); sources != "" {
		fp.Sources = strings.Split(sources, ",")
	}
	if domains := r.URL.Query().Get("domains"); domains != "" {
		fp.Domains = strings.Split(domains, ",")
	}
	if exclude := r.URL.Query().Get("exclude_domains"); exclude != "" {
		fp.ExcludeDomains = strings.Split(exclude, ",")
	}
	if fileTypes := r.URL.Query().Get("file_types"); fileTypes != "" {
		fp.FileTypes = strings.Split(fileTypes, ",")
	}
	if r.URL.Query().Get("multi_source") == "1" {
		fp.MultiSource = true
	}
	fp.Keyword = r.URL.Query().Get("fk")
	if minScoreStr := r.URL.Query().Get("min_score"); minScoreStr != "" {
		fp.MinScore, _ = strconv.ParseFloat(minScoreStr, 64)
	}
	siteGroupName := r.URL.Query().Get("site_group")

	// 合并 NL 指令筛选
	if len(nlFilter.Sources) > 0 && len(fp.Sources) == 0 {
		fp.Sources = nlFilter.Sources
	}
	if len(nlFilter.ExcludeDomains) > 0 {
		fp.ExcludeDomains = append(fp.ExcludeDomains, nlFilter.ExcludeDomains...)
	}
	if len(nlFilter.Domains) > 0 {
		fp.Domains = append(fp.Domains, nlFilter.Domains...)
	}
	if nlFilter.MultiSource {
		fp.MultiSource = true
	}
	if len(nlFilter.FileTypes) > 0 && len(fp.FileTypes) == 0 {
		fp.FileTypes = nlFilter.FileTypes
	}

	siteResults, _ := SearchIndexedSites(query)

	engineResults, err := searcher.Search(query, limit*page, forceRefresh)
	var allResults []SearchResult
	if err != nil {
		allResults = siteResults
	} else {
		// 使用 MergeResults 合并收录站点和搜索引擎结果，按 URL 去重
		allResults = MergeResults(query, siteResults, engineResults)
	}

	// 应用站点分组
	var appliedGroup *SiteGroup
	if siteGroupName != "" {
		for _, g := range GetSiteGroups() {
			if g.Name == siteGroupName {
				allResults = ApplySiteGroup(allResults, g)
				appliedGroup = &g
				break
			}
		}
	}

	// 构建分面响应（在筛选前，基于全量结果）
	var facetResp *FacetResponse
	if page == 1 {
		fr := BuildFacetResponse(allResults, query)
		facetResp = &fr
	}

	// 应用筛选
	allResults = ApplyFilters(allResults, fp)

	start := (page - 1) * limit
	end := start + limit
	var pagedResults []SearchResult
	if start < len(allResults) {
		if end > len(allResults) {
			end = len(allResults)
		}
		pagedResults = allResults[start:end]
	}

	if page == 1 {
		go SaveSearchResults(query, pagedResults)
	}

	var kf *KeywordFeedback
	if page == 1 {
		kf, _ = MatchKeywordFeedback(query)
	}

	response := SearchResponse{
		Query:           query,
		Total:           len(allResults),
		Page:            page,
		PerPage:         limit,
		Results:         pagedResults,
		KeywordFeedback: kf,
		Facets:          facetResp,
		NLCommands:      nlCommands,
	}

	// 附带站点分组列表
	if page == 1 && siteGroupName == "" {
		response.SiteGroups = GetSiteGroups()
	}
	if appliedGroup != nil {
		response.SiteGroups = []SiteGroup{*appliedGroup}
	}

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	writeJSON(w, response)
}

func searchPageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write(indexPage)
}

func historyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			limit = 20
		}
	}

	history, err := GetSearchHistory(limit)
	if err != nil {
		writeError(w, fmt.Sprintf("Failed to get history: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, history)
}

func clearHistoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := ClearSearchHistory()
	if err != nil {
		writeError(w, fmt.Sprintf("Failed to clear history: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"success": true})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	total, expired := searcher.cache.Stats()
	health := map[string]interface{}{
		"status":        "ok",
		"cache_total":   total,
		"cache_expired": expired,
		"engines":       len(searcher.searchers),
	}
	writeJSON(w, health)
}

// ========== 收藏 API ==========
func favoritesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	switch r.Method {
	case http.MethodGet:
		favs, err := GetFavorites()
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, favs)
	case http.MethodPost:
		var rq SearchResult
		if err := json.NewDecoder(r.Body).Decode(&rq); err != nil {
			writeError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		id, err := AddFavorite(rq)
		if err != nil {
			writeJSON(w, map[string]interface{}{"success": false, "error": err.Error(), "id": id})
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "id": id})
	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func favoriteDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	var idStr string
	for i, p := range pathParts {
		if p == "favorites" && i+1 < len(pathParts) {
			idStr = pathParts[i+1]
			break
		}
	}
	if idStr == "" {
		var rq struct {
			URL string `json:"url"`
			ID  int    `json:"id"`
		}
		json.NewDecoder(r.Body).Decode(&rq)
		if rq.ID > 0 {
			RemoveFavorite(rq.ID)
		} else if rq.URL != "" {
			RemoveFavoriteByURL(rq.URL)
		} else {
			writeError(w, "Missing id or url", http.StatusBadRequest)
			return
		}
	} else {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			writeError(w, "Invalid id", http.StatusBadRequest)
			return
		}
		RemoveFavorite(id)
	}
	writeJSON(w, map[string]interface{}{"success": true})
}

func favoriteCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	url := r.URL.Query().Get("url")
	fav, id := IsFavorite(url)
	writeJSON(w, map[string]interface{}{"favorite": fav, "id": id})
}

// ========== 关键词反馈 API ==========
func keywordFeedbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	switch r.Method {
	case http.MethodGet:
		list, err := GetAllKeywordFeedback()
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, list)
	case http.MethodPost:
		var rq KeywordFeedback
		if err := json.NewDecoder(r.Body).Decode(&rq); err != nil {
			writeError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		id, err := AddKeywordFeedback(&rq)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "id": id})
	case http.MethodPut:
		var rq KeywordFeedback
		if err := json.NewDecoder(r.Body).Decode(&rq); err != nil {
			writeError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if err := UpdateKeywordFeedback(&rq); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true})
	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func keywordFeedbackDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rq struct {
		ID int `json:"id"`
	}
	json.NewDecoder(r.Body).Decode(&rq)
	if rq.ID == 0 {
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		for i, p := range pathParts {
			if p == "keyword-feedback" && i+1 < len(pathParts) {
				id, _ := strconv.Atoi(pathParts[i+1])
				rq.ID = id
				break
			}
		}
	}
	if rq.ID == 0 {
		writeError(w, "Missing id", http.StatusBadRequest)
		return
	}
	DeleteKeywordFeedback(rq.ID)
	writeJSON(w, map[string]interface{}{"success": true})
}

// ========== 收录站点 API ==========
func indexedSitesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	switch r.Method {
	case http.MethodGet:
		approvedOnly := r.URL.Query().Get("approved") != "all"
		list, err := GetAllIndexedSites(approvedOnly)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, list)
	case http.MethodPost:
		var rq IndexedSite
		if err := json.NewDecoder(r.Body).Decode(&rq); err != nil {
			writeError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		rq.Enabled = true
		rq.Approved = false
		id, err := AddIndexedSite(&rq)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "id": id, "message": "提交成功，等待管理员审核"})
	case http.MethodPut:
		var rq IndexedSite
		if err := json.NewDecoder(r.Body).Decode(&rq); err != nil {
			writeError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if err := UpdateIndexedSite(&rq); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true})
	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func indexedSitesDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rq struct {
		ID int `json:"id"`
	}
	json.NewDecoder(r.Body).Decode(&rq)
	if rq.ID == 0 {
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		for i, p := range pathParts {
			if p == "indexed-sites" && i+1 < len(pathParts) {
				id, _ := strconv.Atoi(pathParts[i+1])
				rq.ID = id
				break
			}
		}
	}
	if rq.ID == 0 {
		writeError(w, "Missing id", http.StatusBadRequest)
		return
	}
	DeleteIndexedSite(rq.ID)
	writeJSON(w, map[string]interface{}{"success": true})
}

// ========== 搜索统计 API ==========
func searchStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	switch r.Method {
	case http.MethodGet:
		limitStr := r.URL.Query().Get("limit")
		limit := 50
		if limitStr != "" {
			limit, _ = strconv.Atoi(limitStr)
		}
		list, err := GetTopSearchStats(limit)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, list)
	case http.MethodPost:
		if strings.Contains(r.URL.Path, "clear") {
			ClearSearchStats()
			writeJSON(w, map[string]interface{}{"success": true})
			return
		}
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ========== 自定义搜索源 API ==========
func customEnginesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	switch r.Method {
	case http.MethodGet:
		enabledOnly := r.URL.Query().Get("enabled") != "all"
		list, err := GetCustomEngines(enabledOnly)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, list)
	case http.MethodPost:
		var rq CustomEngine
		if err := json.NewDecoder(r.Body).Decode(&rq); err != nil {
			writeError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		id, err := AddCustomEngine(&rq)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "id": id})
	case http.MethodPut:
		var rq CustomEngine
		if err := json.NewDecoder(r.Body).Decode(&rq); err != nil {
			writeError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if err := UpdateCustomEngine(&rq); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true})
	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func customEnginesDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rq struct {
		ID int `json:"id"`
	}
	json.NewDecoder(r.Body).Decode(&rq)
	if rq.ID == 0 {
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		for i, p := range pathParts {
			if p == "custom-engines" && i+1 < len(pathParts) {
				id, _ := strconv.Atoi(pathParts[i+1])
				rq.ID = id
				break
			}
		}
	}
	if rq.ID == 0 {
		writeError(w, "Missing id", http.StatusBadRequest)
		return
	}
	DeleteCustomEngine(rq.ID)
	writeJSON(w, map[string]interface{}{"success": true})
}

// ============ 搜索会话 API ============
func searchSessionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	switch r.Method {
	case http.MethodGet:
		limitStr := r.URL.Query().Get("limit")
		limit := 50
		if limitStr != "" {
			limit, _ = strconv.Atoi(limitStr)
		}
		sessions, err := GetSearchSessions(limit)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, sessions)
	case http.MethodPost:
		var rq struct {
			Query   string         `json:"query"`
			Results []SearchResult `json:"results"`
		}
		if err := json.NewDecoder(r.Body).Decode(&rq); err != nil {
			writeError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		id, err := SaveSearchSession(rq.Query, rq.Results)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "id": id})
	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func searchSessionDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "clear") {
		ClearSearchSessions()
		writeJSON(w, map[string]interface{}{"success": true})
		return
	}
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rq struct {
		ID int `json:"id"`
	}
	json.NewDecoder(r.Body).Decode(&rq)
	if rq.ID == 0 {
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		for i, p := range pathParts {
			if p == "search-sessions" && i+1 < len(pathParts) {
				rq.ID, _ = strconv.Atoi(pathParts[i+1])
				break
			}
		}
	}
	if rq.ID == 0 {
		writeError(w, "Missing id", http.StatusBadRequest)
		return
	}
	DeleteSearchSession(rq.ID)
	writeJSON(w, map[string]interface{}{"success": true})
}

func main() {
	http.HandleFunc("/api/search", searchHandler)
	http.HandleFunc("/api/history", historyHandler)
	http.HandleFunc("/api/history/clear", clearHistoryHandler)
	http.HandleFunc("/api/health", healthHandler)
	http.HandleFunc("/api/favorites", favoritesHandler)
	http.HandleFunc("/api/favorites/", favoriteDeleteHandler)
	http.HandleFunc("/api/favorites/check", favoriteCheckHandler)
	http.HandleFunc("/api/keyword-feedback", keywordFeedbackHandler)
	http.HandleFunc("/api/keyword-feedback/", keywordFeedbackDeleteHandler)
	http.HandleFunc("/api/indexed-sites", indexedSitesHandler)
	http.HandleFunc("/api/indexed-sites/", indexedSitesDeleteHandler)
	http.HandleFunc("/api/search-stats", searchStatsHandler)
	http.HandleFunc("/api/search-stats/clear", searchStatsHandler)
	http.HandleFunc("/api/custom-engines", customEnginesHandler)
	http.HandleFunc("/api/custom-engines/", customEnginesDeleteHandler)
	http.HandleFunc("/api/search-sessions", searchSessionsHandler)
	http.HandleFunc("/api/search-sessions/", searchSessionDeleteHandler)

	fs := http.FileServer(http.Dir(projectRoot))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// 运维面板已迁移到独立端口 127.0.0.1:8091
		if path == "/admin" || path == "/admin.html" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusMovedPermanently)
			fmt.Fprint(w, `{"error":"admin_moved","message":"运维面板已迁移到独立端口","admin_url":"http://127.0.0.1:8091/admin"}`)
			return
		}
		// 根据扩展名设置缓存策略（开发期防刷不出新内容）
		setDevCacheControl(w, path)
		if path == "/" {
			// 首页使用 searchPageHandler 输出
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			searchPageHandler(w, r)
			return
		}
		fs.ServeHTTP(w, r)
	})

	fmt.Println("Search engine service starting...")
	fmt.Println("Access address: http://localhost:8081")
	fmt.Println("Health check: http://localhost:8081/api/health")
	fmt.Println("Admin panel: http://127.0.0.1:8091/admin (独立端口+Token)")
	fmt.Printf("Project root: %s\n", projectRoot)

	// 启动运维面板（独立端口 127.0.0.1:8091 + Token 认证）
	startAdminServer()

	err := http.ListenAndServe(":8081", nil)
	if err != nil {
		fmt.Printf("Service start failed: %v\n", err)
	}
}

// setDevCacheControl 给静态资源设置开发友好的缓存策略（避免浏览器刷不出新内容）
func setDevCacheControl(w http.ResponseWriter, urlPath string) {
	lower := strings.ToLower(urlPath)
	switch {
	case strings.HasSuffix(lower, ".html"), strings.HasSuffix(lower, ".htm"):
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	case strings.HasSuffix(lower, ".js"), strings.HasSuffix(lower, ".mjs"),
		strings.HasSuffix(lower, ".css"), strings.HasSuffix(lower, ".json"),
		strings.HasSuffix(lower, ".map"), strings.HasSuffix(lower, ".svg"),
		strings.HasSuffix(lower, ".wasm"):
		w.Header().Set("Cache-Control", "no-cache, max-age=0")
	default:
		// 图片/字体等：默认给一个短的 max-age 避免每次都下载
		w.Header().Set("Cache-Control", "public, max-age=60, must-revalidate")
	}
}
