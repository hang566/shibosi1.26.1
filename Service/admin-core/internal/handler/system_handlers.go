// Package handler HTTP 处理层（6 大模块）
package handler

import (
	"admin-core/internal/middleware"
	"admin-core/internal/model"
	"admin-core/internal/service"
	"admin-core/internal/terminal"
	"admin-core/internal/ws"
	"fmt"

	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// NewSystemHandlers 6 大模块处理器集合
type SystemHandlers struct {
	Firewall   *service.FirewallService
	Crontab    *service.CrontabService
	File       *service.FileService
	Log        *service.LogService
	Runtime    *service.RuntimeService
	Terminal   *terminal.Manager
	Hub        *ws.Hub
	BotHandler *BotHandler
	UploadDir  string
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// RegisterAll 注册所有 6 大模块路由
func (h *SystemHandlers) RegisterAll(g *gin.RouterGroup) {
	// 仪表盘 & 实时系统状态
	g.GET("/dashboard/system-stat", h.HandleSystemLatest)
	g.GET("/dashboard/system-history", h.HandleSystemHistory)
	g.GET("/ws/system", h.HandleSystemWS)

	// 2. 防火墙
	fw := g.Group("/firewall")
	{
		fw.GET("/rules", h.ListFirewallRules)
		fw.POST("/rules", h.CreateFirewallRule)
		fw.DELETE("/rules/:id", h.DeleteFirewallRule)
		fw.PUT("/rules/:id/toggle", h.ToggleFirewallRule)
		fw.GET("/blocked", h.ListBlocked)
		fw.DELETE("/blocked/:ip", h.UnblockIP)
	}

	// 3. 文件管理器
	fm := g.Group("/files")
	{
		fm.GET("/tree", h.ListFiles)
		fm.GET("/read", h.ReadFile)
		fm.POST("/write", h.WriteFile)
		fm.POST("/mkdir", h.CreateDir)
		fm.POST("/rename", h.RenameFile)
		fm.POST("/copy", h.CopyFile)
		fm.POST("/move", h.MoveFile)
		fm.DELETE("", h.DeleteFile)
		fm.POST("/chmod", h.ChmodFile)
		fm.POST("/zip", h.ZipFile)
		fm.POST("/tar", h.TarGzFile)
		fm.POST("/extract", h.ExtractArchive)
		fm.GET("/search", h.SearchFiles)
		fm.GET("/download", h.DownloadFile)
		fm.POST("/upload", h.UploadFile)
	}

	// 4. 计划任务
	ct := g.Group("/crontabs")
	{
		ct.GET("", h.ListCrontabs)
		ct.POST("", h.CreateCrontab)
		ct.PUT("/:id", h.UpdateCrontab)
		ct.DELETE("/:id", h.DeleteCrontab)
		ct.POST("/:id/trigger", h.TriggerCrontab)
		ct.GET("/:id/logs", h.ListCrontabLogs)
		ct.GET("/translate", h.TranslateCronExpr)
	}

	// 5. 终端 & 日志
	tl := g.Group("/terminal")
	{
		tl.GET("/sessions", h.ListTerminalSessions)
		tl.DELETE("/sessions/:id", h.CloseTerminalSession)
		tl.GET("/ws", h.HandleTerminalWS)
	}
	lg := g.Group("/logs")
	{
		lg.GET("", h.ListLogFiles)
		lg.GET("/tail", h.TailLogFile)
	}

	// 6. 算法机器人
	bot := g.Group("/bots")
	{
		bot.GET("", h.BotHandler.ListBots)
		bot.GET("/stats", h.BotHandler.GetBotStats)
		bot.GET("/:id", h.BotHandler.GetBot)
		bot.POST("", h.BotHandler.CreateBot)
		bot.PUT("/:id", h.BotHandler.UpdateBot)
		bot.DELETE("/:id", h.BotHandler.DeleteBot)
		bot.POST("/:id/start", h.BotHandler.StartBot)
		bot.POST("/:id/stop", h.BotHandler.StopBot)
		bot.POST("/:id/trigger", h.BotHandler.TriggerBot)
		bot.POST("/start-all", h.BotHandler.StartAllBots)
		bot.POST("/stop-all", h.BotHandler.StopAllBots)
		bot.POST("/auto-config", h.BotHandler.AutoConfig)
		bot.GET("/logs/list", h.BotHandler.ListBotLogs)
		bot.POST("/logs/clean", h.BotHandler.CleanBotLogs)
		bot.GET("/configs/list", h.BotHandler.ListBotConfigs)
		bot.POST("/configs/save", h.BotHandler.SaveBotConfig)
		bot.DELETE("/configs/:id", h.BotHandler.DeleteBotConfig)
	}
}

// ===== Dashboard =====

func (h *SystemHandlers) HandleSystemLatest(c *gin.Context) {
	stat := h.Runtime.Latest()
	if stat == nil {
		model.Success(c, nil)
		return
	}
	model.Success(c, stat)
}

func (h *SystemHandlers) HandleSystemHistory(c *gin.Context) {
	n, _ := strconv.Atoi(c.DefaultQuery("n", "60"))
	model.Success(c, h.Runtime.Snapshot(n))
}

func (h *SystemHandlers) HandleSystemWS(c *gin.Context) {
	// 此接口为文档预留；真实系统状态走系统监控 WS
	model.Success(c, gin.H{"message": "use /ws/system-upgrade instead"})
}

// ===== Firewall =====

func (h *SystemHandlers) ListFirewallRules(c *gin.Context) {
	rules, err := h.Firewall.ListRules()
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, rules)
}

func (h *SystemHandlers) CreateFirewallRule(c *gin.Context) {
	var r model.FirewallRule
	if err := c.ShouldBindJSON(&r); err != nil {
		model.Fail(c, 400, err.Error())
		return
	}
	if err := h.Firewall.CreateRule(&r); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, r)
}

func (h *SystemHandlers) DeleteFirewallRule(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.Firewall.DeleteRule(id); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, nil)
}

func (h *SystemHandlers) ToggleFirewallRule(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var body struct {
		Enabled bool `json:"enabled"`
	}
	c.ShouldBindJSON(&body)
	if err := h.Firewall.ToggleRule(id, body.Enabled); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, nil)
}

func (h *SystemHandlers) ListBlocked(c *gin.Context) {
	model.Success(c, h.Firewall.ListBlocked())
}

func (h *SystemHandlers) UnblockIP(c *gin.Context) {
	ok := h.Firewall.UnblockIP(c.Param("ip"))
	if !ok {
		model.Fail(c, 404, "未找到该 IP")
		return
	}
	model.Success(c, nil)
}

// ===== Files =====

func (h *SystemHandlers) ListFiles(c *gin.Context) {
	path := c.DefaultQuery("path", "/")
	depth, _ := strconv.Atoi(c.DefaultQuery("depth", "1"))
	nodes, err := h.File.List(path, depth)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, nodes)
}

func (h *SystemHandlers) ReadFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		model.Fail(c, 400, "缺少 path 参数")
		return
	}
	data, err := h.File.Read(path)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, gin.H{"path": path, "content": data})
}

func (h *SystemHandlers) WriteFile(c *gin.Context) {
	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"` // base64
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		model.Fail(c, 400, err.Error())
		return
	}
	if err := h.File.Write(body.Path, body.Content); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, nil)
}

func (h *SystemHandlers) CreateDir(c *gin.Context) {
	var body struct {
		Path string `json:"path"`
	}
	c.ShouldBindJSON(&body)
	if err := h.File.CreateDir(body.Path); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, nil)
}

func (h *SystemHandlers) RenameFile(c *gin.Context) {
	var body struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	c.ShouldBindJSON(&body)
	if err := h.File.Rename(body.Old, body.New); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, nil)
}

func (h *SystemHandlers) CopyFile(c *gin.Context) {
	var body struct {
		Src string `json:"src"`
		Dst string `json:"dst"`
	}
	c.ShouldBindJSON(&body)
	if err := h.File.Copy(body.Src, body.Dst); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, nil)
}

func (h *SystemHandlers) MoveFile(c *gin.Context) {
	var body struct {
		Src string `json:"src"`
		Dst string `json:"dst"`
	}
	c.ShouldBindJSON(&body)
	if err := h.File.Move(body.Src, body.Dst); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, nil)
}

func (h *SystemHandlers) DeleteFile(c *gin.Context) {
	var body struct {
		Path string `json:"path"`
	}
	path := c.Query("path")
	if path == "" {
		c.ShouldBindJSON(&body)
		path = body.Path
	}
	if path == "" {
		model.Fail(c, 400, "缺少 path")
		return
	}
	if err := h.File.Delete(path); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, nil)
}

func (h *SystemHandlers) ChmodFile(c *gin.Context) {
	var body struct {
		Path string `json:"path"`
		Mode string `json:"mode"`
	}
	c.ShouldBindJSON(&body)
	if err := h.File.Chmod(body.Path, body.Mode); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, nil)
}

func (h *SystemHandlers) ZipFile(c *gin.Context) {
	var body struct {
		Src  string `json:"src"`
		Dest string `json:"dest"`
	}
	c.ShouldBindJSON(&body)
	out, err := h.File.Zip(body.Src, body.Dest)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, gin.H{"path": out})
}

func (h *SystemHandlers) TarGzFile(c *gin.Context) {
	var body struct {
		Src  string `json:"src"`
		Dest string `json:"dest"`
	}
	c.ShouldBindJSON(&body)
	out, err := h.File.TarGz(body.Src, body.Dest)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, gin.H{"path": out})
}

func (h *SystemHandlers) ExtractArchive(c *gin.Context) {
	var body struct {
		Archive string `json:"archive"`
		Dest    string `json:"dest"`
	}
	c.ShouldBindJSON(&body)
	if err := h.File.Extract(body.Archive, body.Dest); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, nil)
}

func (h *SystemHandlers) SearchFiles(c *gin.Context) {
	kw := c.Query("keyword")
	if kw == "" {
		model.Fail(c, 400, "缺少 keyword")
		return
	}
	nodes, err := h.File.Search(kw)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, nodes)
}

func (h *SystemHandlers) DownloadFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		model.Fail(c, 400, "缺少 path")
		return
	}
	abs := filepath.Join(h.File.BaseDir(), path)
	if _, err := os.Stat(abs); err != nil {
		model.Fail(c, 404, "文件不存在")
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+filepath.Base(abs)+`"`)
	c.File(abs)
}

func (h *SystemHandlers) UploadFile(c *gin.Context) {
	path := c.PostForm("path")
	if path == "" {
		model.Fail(c, 400, "缺少 path")
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		model.Fail(c, 400, err.Error())
		return
	}
	abs := filepath.Join(h.File.BaseDir(), path, file.Filename)
	os.MkdirAll(filepath.Dir(abs), 0755)
	if err := c.SaveUploadedFile(file, abs); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, gin.H{"path": abs})
}

// ===== Crontab =====

func (h *SystemHandlers) ListCrontabs(c *gin.Context) {
	list, err := h.Crontab.List()
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, list)
}

func (h *SystemHandlers) CreateCrontab(c *gin.Context) {
	var t model.Crontab
	if err := c.ShouldBindJSON(&t); err != nil {
		model.Fail(c, 400, err.Error())
		return
	}
	if err := h.Crontab.Create(&t); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, t)
}

func (h *SystemHandlers) UpdateCrontab(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var t model.Crontab
	c.ShouldBindJSON(&t)
	t.ID = id
	if err := h.Crontab.Update(&t); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, t)
}

func (h *SystemHandlers) DeleteCrontab(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.Crontab.Delete(id); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, nil)
}

func (h *SystemHandlers) TriggerCrontab(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.Crontab.Trigger(id); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, nil)
}

func (h *SystemHandlers) ListCrontabLogs(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	list, err := h.Crontab.ListLogs(id)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, list)
}

func (h *SystemHandlers) TranslateCronExpr(c *gin.Context) {
	expr := c.Query("expr")
	model.Success(c, gin.H{"expr": expr, "chinese": service.ToChinese(expr)})
}

// ===== Terminal & Logs =====

func (h *SystemHandlers) ListTerminalSessions(c *gin.Context) {
	model.Success(c, h.Terminal.ListSessions())
}

func (h *SystemHandlers) CloseTerminalSession(c *gin.Context) {
	h.Terminal.CloseSession(c.Param("id"))
	model.Success(c, nil)
}

func (h *SystemHandlers) HandleTerminalWS(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	user, _ := c.Get("username")
	userID, _ := c.Get("user_id")
	var uid int64
	switch v := userID.(type) {
	case float64:
		uid = int64(v)
	case int64:
		uid = v
	}
	_, err2 := h.Terminal.CreateShellSession(uid, toStringHelper(user), ws)
	if err2 != nil {
		log.Println("[terminal] create session failed:", err2)
		return
	}
	// 保持连接
	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *SystemHandlers) ListLogFiles(c *gin.Context) {
	list, err := h.Log.ListLogs()
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, list)
}

func (h *SystemHandlers) TailLogFile(c *gin.Context) {
	filename := c.Query("file")
	if filename == "" {
		model.Fail(c, 400, "缺少 file 参数")
		return
	}
	lines, _ := strconv.Atoi(c.DefaultQuery("lines", "200"))
	if err := h.Log.Tail(filename, lines, h.Hub); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, gin.H{"topic": "log:" + filename})
}

// ===== Helpers =====

func toStringHelper(v interface{}) string {
	switch s := v.(type) {
	case string:
		return s
	case fmt.Stringer:
		return s.String()
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// 引用 middleware 防止未使用
var _ = middleware.IsPrivateIP

// 引用 os 路径
var _ = os.ReadFile
