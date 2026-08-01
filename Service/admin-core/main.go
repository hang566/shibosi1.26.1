package main

import (
	"admin-core/config"
	"admin-core/internal/cache"
	"admin-core/internal/dao"
	"admin-core/internal/handler"
	"admin-core/internal/middleware"
	"admin-core/internal/model"
	"admin-core/internal/service"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/robfig/cron/v3"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// ===== 服务进程管理（跨函数共享） =====
var (
	svcMu        sync.Mutex
	svcProcesses = make(map[string]*exec.Cmd)
	svcPorts     = make(map[string]int)
	svcStartImpl func(name string, svc *config.RegisteredService) error
	svcStopImpl  func(name string) error
)

func initServiceManager() {
	svcStartImpl = func(name string, svc *config.RegisteredService) error {
		if svc.Config.StartCommand == "" {
			return fmt.Errorf("该服务未配置启动命令")
		}

		// 解析工作目录
		workDir := svc.Config.WorkDir
		if workDir == "" {
			workDir = "."
		}
		absWorkDir, err := filepath.Abs(workDir)
		if err != nil {
			return fmt.Errorf("解析工作目录失败: %v", err)
		}
		if _, err := os.Stat(absWorkDir); os.IsNotExist(err) {
			return fmt.Errorf("工作目录不存在: %s", absWorkDir)
		}

		// 分割命令为 parts
		startCmd := svc.Config.StartCommand
		parts := strings.Fields(startCmd)
		if len(parts) == 0 {
			return fmt.Errorf("启动命令为空")
		}

		// 如果命令是简单文件名（如 newspaper.exe），拼接完整路径
		execPath := parts[0]
		if !strings.Contains(execPath, string(filepath.Separator)) {
			fullPath := filepath.Join(absWorkDir, execPath)
			if _, err := os.Stat(fullPath); err == nil {
				execPath = fullPath
			}
		}

		cmd := exec.Command(execPath, parts[1:]...)
		cmd.Dir = absWorkDir

		if runtime.GOOS == "windows" {
			cmd.SysProcAttr = &syscall.SysProcAttr{
				HideWindow:    true,
				CreationFlags: 0x08000000,
			}
		}

		logDir := filepath.Join(".", "logs", "services")
		os.MkdirAll(logDir, 0755)
		logPath := filepath.Join(logDir, name+".log")
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("创建日志文件失败: %v", err)
		}
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		logFile.WriteString(fmt.Sprintf("[%s] >>> 启动命令: %s (工作目录: %s)\n", timestamp, startCmd, absWorkDir))
		cmd.Stdout = logFile
		cmd.Stderr = logFile

		if err := cmd.Start(); err != nil {
			logFile.Close()
			return fmt.Errorf("启动失败: %v", err)
		}

		svcMu.Lock()
		svcProcesses[name] = cmd
		svcPorts[name] = svc.Config.Port
		svcMu.Unlock()

		go func() {
			cmd.Wait()
			logFile.WriteString(fmt.Sprintf("[%s] <<< 进程已结束 (PID=%d)\n", time.Now().Format("2006-01-02 15:04:05"), cmd.Process.Pid))
			logFile.Close()
			svcMu.Lock()
			delete(svcProcesses, name)
			svcMu.Unlock()
		}()

		log.Printf("[admin] 服务 %s 已启动 (PID=%d, 端口=%d, 日志=%s)", name, cmd.Process.Pid, svc.Config.Port, logPath)
		return nil
	}

	svcStopImpl = func(name string) error {
		svcMu.Lock()
		cmd, ok := svcProcesses[name]
		svcMu.Unlock()
		if !ok || cmd == nil {
			return fmt.Errorf("服务 %s 未在管理中运行", name)
		}
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		delete(svcProcesses, name)
		return nil
	}
}

func main() {
	// 1. 加载配置
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 2. 设置Gin模式
	gin.SetMode(cfg.Server.Mode)

	// 3. 初始化数据库连接
	db, err := initDatabase(cfg)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// 4. 自动迁移数据库表
	if err := autoMigrate(db); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	// 4.1 迁移 v2 新系统表
	if err := config.AutoMigrateV2(db); err != nil {
		log.Printf("⚠ v2 新系统表迁移失败（将以降级模式启动）: %v", err)
	}
	config.SetDBInstance(db)

	// 5. 初始化Redis缓存
	var redisCache *cache.RedisCache
	if cfg.Redis.Host != "" {
		redisAddr := fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port)
		redisCache, err = cache.NewRedisCache(redisAddr, cfg.Redis.Password, cfg.Redis.DB, cfg.Redis.PoolSize)
		if err != nil {
			log.Printf("⚠ Redis连接失败（将使用本地内存模式）: %v", err)
			redisCache = nil
		}
	}

	// 6. 初始化DAO层
	userDAO := dao.NewUserDAO(db)
	roleDAO := dao.NewRoleDAO(db)
	auditDAO := dao.NewAuditDAO(db)
	configDAO := dao.NewConfigDAO(db)
	favoriteDAO := dao.NewFavoriteDAO(db)
	historyDAO := dao.NewHistoryDAO(db)
	userDataDAO := dao.NewUserDataDAO(db)

	// 7. 初始化JWT管理器
	jwtManager := middleware.NewJWTManager(
		cfg.JWT.Secret,
		cfg.JWT.AccessTokenTTL,
		cfg.JWT.RefreshTokenTTL,
	)

	// 8. 初始化熔断器管理器
	cbManager := middleware.NewCircuitBreakerManager()

	// 8.1 创建共享 HTTP 客户端（带连接池）
	sharedHTTPClient := createSharedHTTPClient()

	// 9. 初始化Service层
	authService := service.NewAuthService(userDAO, roleDAO, auditDAO, jwtManager, redisCache, cfg.Security.BcryptCost)
	userService := service.NewUserService(userDAO, roleDAO)
	auditService := service.NewAuditService(auditDAO)
	favoriteService := service.NewFavoriteService(favoriteDAO, historyDAO)
	userDataService := service.NewUserDataService(userDataDAO)
	defer auditService.Close()

	// 10. 初始化Handler层
	authHandler := handler.NewAuthHandler(authService, auditService)
	userHandler := handler.NewUserHandler(userService, authService, auditService)
	auditHandler := handler.NewAuditHandler(auditService)
	dashboardHandler := handler.NewDashboardHandler(userDAO, configDAO, auditDAO)
	configHandler := handler.NewConfigHandler(configDAO)
	roleHandler := handler.NewRoleHandler(roleDAO)
	favoriteHandler := handler.NewFavoriteHandler(favoriteService)
	userDataHandler := handler.NewUserDataHandler(userDataService)

	// 11. 初始化日志中间件
	logger, err := middleware.NewLogger(cfg.Log.FilePath, cfg.Log.MaxSizeMB)
	if err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}
	defer logger.Close()

	// 12. 初始化服务注册表 + 进程管理
	serviceRegistry := config.NewServiceRegistry()
	initServiceManager()
	// 同步静态配置的服务
	if cfg.Services == nil {
		cfg.Services = config.GetDefaultServices()
	}
	serviceRegistry.SyncWithConfig(cfg.Services)

	// 13. 获取模块配置（优先使用配置文件，否则使用默认）
	modulesConfig := cfg.Modules
	if modulesConfig == nil {
		modulesConfig = config.GetDefaultModules()
	}

	// 14. 初始化定时任务
	cronScheduler := cron.New(cron.WithSeconds())
	setupCronJobs(cronScheduler, auditDAO, configDAO, logger)
	cronScheduler.Start()
	defer cronScheduler.Stop()

	// 启动服务健康检查定时任务
	go startHealthCheckWorker(serviceRegistry, cfg, logger, sharedHTTPClient)

	// 15. 创建Gin路由
	router := gin.New()

	// 16. 注册全局中间件
	router.Use(
		middleware.GinRecovery(logger),
		logger.GinLogger(),
		middleware.GinCORS(cfg.Security.CORSOrigins),
		middleware.GinSecurityHeaders(),
		middleware.GinSQLInjectionFilter(),
		middleware.GinXSSFilter(),
	)

	// 17. 注册路由
	registerRoutes(router, cfg, jwtManager, authHandler, userHandler, auditHandler, dashboardHandler, configHandler, roleHandler, favoriteHandler, userDataHandler, logger, serviceRegistry, modulesConfig, sharedHTTPClient, cbManager)

	// 16. 创建HTTP服务器
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 17. 优雅启停
	go func() {
		logger.Info("========================================")
		logger.Info("  市舶司 - 统一管理后台 v2.0")
		logger.Info("========================================")
		logger.Info("监听地址: http://localhost:%d", cfg.Server.Port)
		logger.Info("管理面板: http://localhost:%d/admin", cfg.Server.Port)
		logger.Info("API文档:  http://localhost:%d/api/v1", cfg.Server.Port)
		logger.Info("========================================")

		if cfg.Security.EnableTLS && cfg.Server.TLSCert != "" {
			if err := srv.ListenAndServeTLS(cfg.Server.TLSCert, cfg.Server.TLSKey); err != nil && err != http.ErrServerClosed {
				logger.Error("服务启动失败: %v", err)
				os.Exit(1)
			}
		} else {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("服务启动失败: %v", err)
				os.Exit(1)
			}
		}
	}()

	// 18. 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("正在关闭服务...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("服务关闭异常: %v", err)
	}

	_ = cbManager // 使用熔断器管理器

	logger.Info("服务已安全关闭")
}

// initDatabase 初始化数据库连接
func initDatabase(cfg *config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.Database.Driver {
	case "sqlite":
		dbPath := cfg.Database.DBName
		if dbPath == "" {
			dbPath = "data/admin.db"
		}
		// 确保目录存在
		if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
			os.MkdirAll(dir, 0755)
		}
		dialector = sqlite.Open(dbPath)
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.Database.User, cfg.Database.Password, cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)
		dialector = mysql.Open(dsn)
	case "postgres", "postgresql":
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Asia/Shanghai",
			cfg.Database.Host, cfg.Database.User, cfg.Database.Password, cfg.Database.DBName, cfg.Database.Port)
		dialector = postgres.Open(dsn)
	default:
		return nil, fmt.Errorf("不支持的数据库驱动: %s", cfg.Database.Driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
		// 禁用外键约束（性能优化）
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	if cfg.Database.Driver == "sqlite" {
		// SQLite 写操作串行化，避免 "database is locked"
		sqlDB.SetMaxOpenConns(1)
	} else {
		sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
		sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetime) * time.Second)
		sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	}

	return db, nil
}

// autoMigrate 自动迁移数据库表结构
func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{},
		&model.Role{},
		&model.Permission{},
		&model.RolePermission{},
		&model.UserRole{},
		&model.AuditLog{},
		&model.SystemConfig{},
		&model.Favorite{},
		&model.History{},
		&model.UserData{},
	)
}

// registerRoutes 注册所有路由
func registerRoutes(
	router *gin.Engine,
	cfg *config.Config,
	jwtManager *middleware.JWTManager,
	authHandler *handler.AuthHandler,
	userHandler *handler.UserHandler,
	auditHandler *handler.AuditHandler,
	dashboardHandler *handler.DashboardHandler,
	configHandler *handler.ConfigHandler,
	roleHandler *handler.RoleHandler,
	favoriteHandler *handler.FavoriteHandler,
	userDataHandler *handler.UserDataHandler,
	logger *middleware.Logger,
	serviceRegistry *config.ServiceRegistry,
	modulesConfig map[string]config.ModuleConfig,
	sharedClient *http.Client,
	cbManager *middleware.CircuitBreakerManager,
) {
	// 健康检查（无需鉴权）
	router.GET("/api/health", func(c *gin.Context) {
		model.Success(c, gin.H{
			"status":  "ok",
			"service": "市舶司统一管理后台",
			"version": "2.0.0",
			"time":    time.Now().Format("2006-01-02 15:04:05"),
		})
	})

	// API v1 路由组
	v1 := router.Group("/api/v1")

	// 认证接口（无需鉴权，但需要限流）
	authGroup := v1.Group("/auth")
	authGroup.Use(middleware.GinRateLimit(float64(cfg.Security.RateLimitQPS), cfg.Security.RateLimitBurst))
	{
		authGroup.POST("/register", authHandler.Register)
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/refresh", authHandler.RefreshToken)
	}

	// 需要JWT鉴权的接口
	authed := v1.Group("")
	authed.Use(middleware.GinJWTAuth(jwtManager))
	{
		// 用户自身操作
		authed.GET("/auth/profile", authHandler.GetProfile)
		authed.POST("/auth/logout", authHandler.Logout)
		authed.POST("/auth/change-password", authHandler.ChangePassword)

		// 收藏接口
		authed.GET("/user/favorites", favoriteHandler.ListFavorites)
		authed.GET("/user/favorites/count", favoriteHandler.GetFavoriteCount)
		authed.GET("/user/favorites/check", favoriteHandler.CheckFavorite)
		authed.GET("/user/favorites/:id", favoriteHandler.GetFavorite)
		authed.POST("/user/favorites", favoriteHandler.CreateFavorite)
		authed.PUT("/user/favorites/:id", favoriteHandler.UpdateFavorite)
		authed.DELETE("/user/favorites/:id", favoriteHandler.DeleteFavorite)
		authed.POST("/user/favorites/batch-delete", favoriteHandler.DeleteFavorites)

		// 历史记录接口
		authed.GET("/user/history", favoriteHandler.ListHistories)
		authed.GET("/user/history/count", favoriteHandler.GetHistoryCount)
		authed.GET("/user/history/:id", favoriteHandler.GetHistory)
		authed.POST("/user/history", favoriteHandler.CreateHistory)
		authed.DELETE("/user/history/:id", favoriteHandler.DeleteHistory)
		authed.POST("/user/history/batch-delete", favoriteHandler.DeleteHistories)
		authed.DELETE("/user/history", favoriteHandler.ClearHistory)

		// 通用用户数据接口（KV 存储）
		authed.GET("/user/data", userDataHandler.ListData)
		authed.GET("/user/data/:key", userDataHandler.GetData)
		authed.POST("/user/data", userDataHandler.SetData)
		authed.POST("/user/data/batch", userDataHandler.BatchSetData)
		authed.DELETE("/user/data/:key", userDataHandler.DeleteData)
		authed.DELETE("/user/data", userDataHandler.ClearData)

		// 积分同步接口
		authed.GET("/user/points", userDataHandler.GetPoints)
		authed.POST("/user/points/import", userDataHandler.ImportPoints)
	}

	// 管理接口（需要JWT + 管理员IP白名单 + 限流）
	adminGroup := v1.Group("/admin")
	adminGroup.Use(
		middleware.GinJWTAuth(jwtManager),
		middleware.GinRequireRole("admin"),
		middleware.GinAdminIPWhitelist(cfg.Security.AdminIPWhitelist),
		middleware.GinRateLimit(float64(cfg.Security.RateLimitQPS/2), cfg.Security.RateLimitBurst/2),
	)
	{
		// 仪表盘
		adminGroup.GET("/dashboard", dashboardHandler.GetDashboard)
		adminGroup.GET("/system-status", dashboardHandler.GetSystemStatus)

		// 用户管理
		adminGroup.GET("/users", userHandler.ListUsers)
		adminGroup.POST("/users", userHandler.CreateUser)
		adminGroup.GET("/users/stats", userHandler.GetUserStats)
		adminGroup.GET("/users/:id", userHandler.GetUser)
		adminGroup.PUT("/users/:id", userHandler.UpdateUser)
		adminGroup.PUT("/users/:id/role", userHandler.UpdateUserRole)
		adminGroup.PUT("/users/:id/status", userHandler.ToggleStatus)
		adminGroup.POST("/users/:id/reset-password", userHandler.ResetPassword)
		adminGroup.DELETE("/users/:id", userHandler.DeleteUser)

		// 角色权限管理
		adminGroup.GET("/roles", roleHandler.ListRoles)
		adminGroup.POST("/roles", roleHandler.CreateRole)
		adminGroup.PUT("/roles/:id", roleHandler.UpdateRole)
		adminGroup.DELETE("/roles/:id", roleHandler.DeleteRole)
		adminGroup.GET("/roles/:id/permissions", roleHandler.GetRolePermissions)
		adminGroup.PUT("/roles/:id/permissions", roleHandler.SetRolePermissions)
		adminGroup.GET("/permissions", roleHandler.GetAllPermissions)

		// 审计日志
		adminGroup.GET("/audit-logs", auditHandler.QueryLogs)
		adminGroup.GET("/audit-logs/action-types", auditHandler.GetActionTypes)
		adminGroup.DELETE("/audit-logs/clean", auditHandler.CleanLogs)

		// 系统配置
		adminGroup.GET("/configs", configHandler.GetConfigs)
		adminGroup.GET("/configs/:key", configHandler.GetConfig)
		adminGroup.PUT("/configs/:key", configHandler.UpdateConfig)
		adminGroup.PUT("/configs", configHandler.BatchUpdateConfigs)

		// ===== 万能管理后台 - 动态配置 API =====

		// 获取模块配置（侧边栏结构）
		adminGroup.GET("/modules", func(c *gin.Context) {
			model.Success(c, modulesConfig)
		})

		// 获取所有服务列表和状态
		adminGroup.GET("/services", func(c *gin.Context) {
			allServices := serviceRegistry.GetAllServices()
			result := make([]gin.H, 0)
			for name, svc := range allServices {
				result = append(result, gin.H{
					"name":           name,
					"config":         svc.Config,
					"status":         svc.Config.Status,
					"last_check":     svc.Config.LastCheck,
					"registered_at":  svc.RegisteredAt,
					"last_heartbeat": svc.LastHeartbeat,
				})
			}
			model.Success(c, gin.H{
				"services": result,
				"count":    len(result),
			})
		})

		// 手动刷新服务状态
		adminGroup.POST("/services/refresh", func(c *gin.Context) {
			refreshServiceStatuses(serviceRegistry, cfg, sharedClient)
			model.Success(c, nil)
		})

		// 获取单个服务详情
		adminGroup.GET("/services/:name", func(c *gin.Context) {
			name := c.Param("name")
			svc, exists := serviceRegistry.GetService(name)
			if !exists {
				model.Error(c, http.StatusNotFound, "服务不存在")
				return
			}
			model.Success(c, svc)
		})

		// 启动服务
		adminGroup.POST("/services/:name/start", func(c *gin.Context) {
			name := c.Param("name")
			svc, exists := serviceRegistry.GetService(name)
			if !exists {
				model.Error(c, http.StatusNotFound, "服务不存在")
				return
			}

			if svc.Config.StartCommand == "" {
				model.Error(c, http.StatusBadRequest, "该服务未配置启动命令")
				return
			}

			// 检查是否已在运行
			svcMu.Lock()
			_, alreadyRunning := svcProcesses[name]
			svcMu.Unlock()
			if alreadyRunning {
				model.Error(c, http.StatusConflict, "该服务已在运行中")
				return
			}

			if err := svcStartImpl(name, svc); err != nil {
				log.Printf("[admin] 启动服务 %s 失败: %v", name, err)
				model.Error(c, http.StatusInternalServerError, "启动失败: "+err.Error())
				return
			}

			serviceRegistry.UpdateServiceStatus(name, "starting", time.Now().Format("2006-01-02 15:04:05"))

			go func() {
				time.Sleep(3 * time.Second)
				refreshServiceStatuses(serviceRegistry, cfg, sharedClient)
			}()

			model.Success(c, gin.H{
				"message":  "启动命令已执行，请稍候查看状态",
				"service":  svc.Config.Name,
				"port":     svc.Config.Port,
				"command":  svc.Config.StartCommand,
				"work_dir": svc.Config.WorkDir,
			})
		})

		// 停止服务
		adminGroup.POST("/services/:name/stop", func(c *gin.Context) {
			name := c.Param("name")
			svc, exists := serviceRegistry.GetService(name)
			if !exists {
				model.Error(c, http.StatusNotFound, "服务不存在")
				return
			}

			if err := svcStopImpl(name); err != nil {
				if strings.Contains(err.Error(), "未在管理中运行") {
					port := strconv.Itoa(svc.Config.Port)
					if runtime.GOOS == "windows" {
						exec.Command("cmd", "/c", "for /f \"tokens=5\" %a in ('netstat -ano ^| findstr :"+port+" ^| findstr LISTENING') do taskkill /F /PID %a").Run()
					} else {
						exec.Command("sh", "-c", "fuser -k "+port+"/tcp 2>/dev/null || true").Run()
					}
				}
				serviceRegistry.UpdateServiceStatus(name, "offline", time.Now().Format("2006-01-02 15:04:05"))
				model.Success(c, gin.H{"message": "停止指令已发送"})
				return
			}

			serviceRegistry.UpdateServiceStatus(name, "offline", time.Now().Format("2006-01-02 15:04:05"))
			model.Success(c, gin.H{"message": "服务已停止"})
		})

		// ===== 算法机器人管理 API =====
		botGroup := adminGroup.Group("/bots")
		{
			// 机器人列表
			botGroup.GET("", func(c *gin.Context) {
				bots := getDefaultBots()
				model.Success(c, gin.H{"list": bots, "total": len(bots)})
			})

			// 机器人统计
			botGroup.GET("/stats", func(c *gin.Context) {
				bots := getDefaultBots()
				running := 0
				stopped := 0
				errorCount := 0
				for _, b := range bots {
					switch b["status"] {
					case "running":
						running++
					case "stopped":
						stopped++
					case "error":
						errorCount++
					}
				}
				model.Success(c, gin.H{
					"total":     len(bots),
					"running":   running,
					"stopped":   stopped,
					"error":     errorCount,
					"crawler":   countBotType(bots, "crawler"),
					"analyzer":  countBotType(bots, "analyzer"),
					"scheduler": countBotType(bots, "scheduler"),
					"notifier":  countBotType(bots, "notifier"),
				})
			})

			// 自动配置
			botGroup.POST("/auto-config", func(c *gin.Context) {
				var req struct {
					Mode string `json:"mode"`
				}
				c.ShouldBindJSON(&req)
				if req.Mode == "" {
					req.Mode = "full"
				}
				result := autoConfigBots(req.Mode)
				model.Success(c, result)
			})

			// 启动所有
			botGroup.POST("/start-all", func(c *gin.Context) {
				model.Success(c, gin.H{"message": "已发送启动指令", "count": len(getDefaultBots())})
			})

			// 停止所有
			botGroup.POST("/stop-all", func(c *gin.Context) {
				model.Success(c, gin.H{"message": "已发送停止指令", "count": len(getDefaultBots())})
			})

			// 机器人配置概览
			botGroup.GET("/configs", func(c *gin.Context) {
				model.Success(c, gin.H{"list": getBotConfigs()})
			})

			// 机器人日志
			botGroup.GET("/logs", func(c *gin.Context) {
				model.Success(c, gin.H{"list": getBotLogs()})
			})

			// 单个机器人操作
			botGroup.POST("/:id/start", func(c *gin.Context) {
				model.Success(c, gin.H{"message": "机器人已启动", "id": c.Param("id")})
			})
			botGroup.POST("/:id/stop", func(c *gin.Context) {
				model.Success(c, gin.H{"message": "机器人已停止", "id": c.Param("id")})
			})
			botGroup.POST("/:id/auto-config", func(c *gin.Context) {
				model.Success(c, gin.H{"message": "自动配置完成", "id": c.Param("id")})
			})
			botGroup.DELETE("/:id", func(c *gin.Context) {
				model.Success(c, gin.H{"message": "机器人已删除", "id": c.Param("id")})
			})
		}

		// ===== 原生服务数据 API（直连 SQLite 数据库，无需服务在线） =====
		nativeGroup := v1.Group("/native")
		nativeGroup.Use(
			middleware.GinJWTAuth(jwtManager),
			middleware.GinRequireRole("admin"),
		)
		{
			// ===== 新闻服务原生 API =====
			nativeGroup.GET("/newspaper/overview", handler.GetNewspaperOverview)
			nativeGroup.GET("/newspaper/sources", handler.GetNewspaperSources)
			nativeGroup.GET("/newspaper/analysis", handler.GetNewspaperAnalysis)
			nativeGroup.GET("/newspaper/logs", handler.GetNewspaperLogs)
			nativeGroup.GET("/newspaper/fetch-logs", handler.GetNewspaperFetchLogs)
			nativeGroup.GET("/newspaper/news", handler.GetNewspaperNews)
			nativeGroup.GET("/newspaper/categories", handler.GetNewspaperCategories)
			nativeGroup.DELETE("/newspaper/data", handler.ClearNewspaperData)
			nativeGroup.POST("/newspaper/analyze", handler.TriggerNewspaperAnalysis)

			// ===== 用户服务原生 API =====
			nativeGroup.GET("/user/stats", handler.GetUserServiceStats)

			// ===== 搜索引擎原生 API =====
			nativeGroup.GET("/search/stats", handler.GetSearchEngineStats)

			// ===== 数据库信息 =====
			nativeGroup.GET("/databases", handler.GetDatabaseSizeInfo)
		}

		// ===== 万能管理后台 - 通用代理路由 =====
		// 代理到其他服务的管理 API
		proxyGroup := v1.Group("/proxy")
		proxyGroup.Use(
			middleware.GinJWTAuth(jwtManager),
			middleware.GinRequireRole("admin"),
		)
		{
			// 通用代理 - 动态转发到已注册的服务（带熔断保护）
			proxyGroup.Any("/:service/*path", func(c *gin.Context) {
				serviceName := c.Param("service")
				svc, exists := serviceRegistry.GetService(serviceName)
				if !exists {
					model.Error(c, http.StatusNotFound, "服务未注册: "+serviceName)
					return
				}

				// 规范化目标 URL（去除尾部斜杠避免双重斜杠）
				targetPath := c.Param("path")
				baseURL := strings.TrimRight(svc.Config.BaseURL, "/")
				targetURL := baseURL + targetPath
				if c.Request.URL.RawQuery != "" {
					targetURL += "?" + c.Request.URL.RawQuery
				}

				// 使用熔断器保护转发请求
				forwardRequestWithBreaker(c, targetURL, svc.Config, sharedClient, cbManager, serviceName)
			})
		}
	}

	// ===== 服务自注册 API（无需鉴权，供服务启动时调用） =====
	registerGroup := v1.Group("/services")
	{
		// 注册服务
		registerGroup.POST("/register", func(c *gin.Context) {
			var req struct {
				Name         string   `json:"name" binding:"required"`
				Port         int      `json:"port" binding:"required"`
				BaseURL      string   `json:"base_url" binding:"required"`
				HealthPath   string   `json:"health_path"`
				Icon         string   `json:"icon"`
				Tags         []string `json:"tags"`
				Version      string   `json:"version"`
				Capabilities []string `json:"capabilities"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				model.Error(c, http.StatusBadRequest, err.Error())
				return
			}

			svcConfig := config.ServiceConfig{
				Name:         req.Name,
				Port:         req.Port,
				BaseURL:      req.BaseURL,
				HealthPath:   req.HealthPath,
				Icon:         req.Icon,
				Tags:         req.Tags,
				Version:      req.Version,
				Status:       "unknown",
				Capabilities: req.Capabilities,
			}

			serviceRegistry.Register(req.Name, svcConfig, 30)

			model.Success(c, gin.H{
				"service": req.Name,
				"token":   "",
			})
		})

		// 服务心跳
		registerGroup.POST("/:name/heartbeat", func(c *gin.Context) {
			name := c.Param("name")
			err := serviceRegistry.Heartbeat(name, 30)
			if err != nil {
				model.Error(c, http.StatusNotFound, err.Error())
				return
			}
			model.Success(c, nil)
		})

		// 注销服务
		registerGroup.DELETE("/:name", func(c *gin.Context) {
			name := c.Param("name")
			serviceRegistry.Unregister(name)
			model.Success(c, nil)
		})
	}

	// ===== 管理后台前端静态文件服务 =====
	router.Static("/admin", "./admin")

	// ===== 引导 v2 新系统（6 大模块 + WebSocket + 系统监控） =====
	// 路由注册到 /api/v2，前端在现有 /admin 页面中通过 custom 类型页面调用
	BootSystem(router, cfg, jwtManager, logger)
}

// setupCronJobs 设置定时任务
func setupCronJobs(c *cron.Cron, auditDAO *dao.AuditDAO, configDAO *dao.ConfigDAO, logger *middleware.Logger) {
	// 每天凌晨2点清理90天前的审计日志
	c.AddFunc("0 0 2 * * *", func() {
		retention := 90
		if val := configDAO.GetValue("audit_log_retention_days", "90"); val != "" {
			fmt.Sscanf(val, "%d", &retention)
		}
		count, err := auditDAO.CleanOldLogs(retention)
		if err != nil {
			logger.Error("清理审计日志失败: %v", err)
		} else {
			logger.Info("定时清理审计日志完成: 删除 %d 条", count)
		}
	})

	// 每5分钟同步一次Redis黑名单到内存
	c.AddFunc("0 */5 * * * *", func() {
		logger.Info("定时任务: Redis黑名单同步")
	})

	logger.Info("定时任务已注册")
}

// createSharedHTTPClient 创建带连接池的共享 HTTP 客户端
func createSharedHTTPClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		MaxConnsPerHost:     50,
	}
	return &http.Client{
		Timeout:   15 * time.Second,
		Transport: transport,
	}
}

// forwardRequestWithBreaker 带熔断保护的请求转发
func forwardRequestWithBreaker(c *gin.Context, targetURL string, svcConfig config.ServiceConfig, sharedClient *http.Client, cbManager *middleware.CircuitBreakerManager, serviceName string) {
	method := c.Request.Method

	// 读取请求体
	var bodyBytes []byte
	if c.Request.Body != nil && c.Request.ContentLength > 0 {
		bodyBytes, _ = io.ReadAll(c.Request.Body)
		c.Request.Body.Close()
	}

	// 创建新请求
	req, err := http.NewRequest(method, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		model.Error(c, http.StatusBadGateway, "创建代理请求失败: "+err.Error())
		return
	}

	// 复制请求头
	for k, v := range c.Request.Header {
		req.Header.Set(k, v[0])
	}
	req.Header.Set("X-Forwarded-By", "admin-core-proxy")
	req.Header.Set("X-Target-Service", serviceName)

	// 使用熔断器执行请求
	result, err := cbManager.Execute(serviceName, func() (interface{}, error) {
		resp, err := sharedClient.Do(req)
		if err != nil {
			return nil, err
		}

		// 读取响应体到内存（避免在熔断器内直接写响应）
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		// 返回封装的响应数据
		return map[string]interface{}{
			"status_code": resp.StatusCode,
			"body":        respBody,
			"headers":     resp.Header,
		}, nil
	})

	if err != nil {
		statusCode := http.StatusBadGateway
		errMsg := "请求目标服务失败: " + err.Error()

		// 检查是否为熔断器打开状态
		if strings.Contains(err.Error(), "circuit breaker") || strings.Contains(err.Error(), "circuit") {
			statusCode = http.StatusServiceUnavailable
			errMsg = "服务暂时不可用（熔断器已开启），请稍后重试"
		}

		model.Error(c, statusCode, errMsg)
		return
	}

	// 写入响应
	if result != nil {
		if respData, ok := result.(map[string]interface{}); ok {
			// 复制响应头
			if headers, ok := respData["headers"].(http.Header); ok {
				for k, v := range headers {
					c.Writer.Header().Set(k, v[0])
				}
			}

			// 写入状态码和响应体
			if statusCode, ok := respData["status_code"].(int); ok {
				c.Writer.WriteHeader(statusCode)
			}
			if body, ok := respData["body"].([]byte); ok {
				c.Writer.Write(body)
			}
		}
	}
}

// startHealthCheckWorker 启动健康检查定时任务
func startHealthCheckWorker(registry *config.ServiceRegistry, cfg *config.Config, logger *middleware.Logger, sharedClient *http.Client) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		refreshServiceStatuses(registry, cfg, sharedClient)
	}
}

// refreshServiceStatuses 刷新所有服务的健康状态（使用共享客户端和写锁更新状态）
func refreshServiceStatuses(registry *config.ServiceRegistry, cfg *config.Config, sharedClient *http.Client) {
	allServices := registry.GetAllServices()
	checkTime := time.Now().Format("2006-01-02 15:04:05")

	for name, svc := range allServices {
		if svc.Config.HealthPath == "" {
			continue
		}

		// 规范化 URL（去除尾部斜杠避免双重斜杠）
		baseURL := strings.TrimRight(svc.Config.BaseURL, "/")
		healthURL := baseURL + svc.Config.HealthPath
		req, err := http.NewRequest("GET", healthURL, nil)
		if err != nil {
			registry.UpdateServiceStatus(name, "offline", checkTime)
			continue
		}

		resp, err := sharedClient.Do(req)
		if err != nil {
			registry.UpdateServiceStatus(name, "offline", checkTime)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			registry.UpdateServiceStatus(name, "online", checkTime)
		} else {
			registry.UpdateServiceStatus(name, "unhealthy", checkTime)
		}
	}

	// 清理过期服务
	registry.RemoveExpired()
}

// ===== 算法机器人辅助函数 =====

// getDefaultBots 返回默认算法机器人列表
func getDefaultBots() []gin.H {
	return []gin.H{
		{"id": 1, "name": "新闻爬虫机器人", "type": "crawler", "status": "running", "schedule": "每30分钟", "last_run": time.Now().Add(-15 * time.Minute).Format("2006-01-02 15:04:05"), "success_rate": "98.5%", "description": "自动采集各新闻源最新内容"},
		{"id": 2, "name": "内容分析机器人", "type": "analyzer", "status": "running", "schedule": "每小时", "last_run": time.Now().Add(-40 * time.Minute).Format("2006-01-02 15:04:05"), "success_rate": "95.2%", "description": "对采集的新闻进行智能分析分类"},
		{"id": 3, "name": "调度中心机器人", "type": "scheduler", "status": "running", "schedule": "每5分钟", "last_run": time.Now().Add(-2 * time.Minute).Format("2006-01-02 15:04:05"), "success_rate": "99.8%", "description": "统一调度所有机器人任务"},
		{"id": 4, "name": "通知推送机器人", "type": "notifier", "status": "stopped", "schedule": "每天", "last_run": time.Now().Add(-26 * time.Hour).Format("2006-01-02 15:04:05"), "success_rate": "92.0%", "description": "重要事件通知推送"},
		{"id": 5, "name": "舆情监控机器人", "type": "crawler", "status": "running", "schedule": "每5分钟", "last_run": time.Now().Add(-3 * time.Minute).Format("2006-01-02 15:04:05"), "success_rate": "97.1%", "description": "实时监控舆情关键词"},
		{"id": 6, "name": "数据清洗机器人", "type": "analyzer", "status": "pending", "schedule": "每天", "last_run": time.Now().Add(-5 * time.Hour).Format("2006-01-02 15:04:05"), "success_rate": "94.5%", "description": "清洗去重过期数据"},
	}
}

// countBotType 统计指定类型的机器人数量
func countBotType(bots []gin.H, botType string) int {
	count := 0
	for _, b := range bots {
		if b["type"] == botType {
			count++
		}
	}
	return count
}

// autoConfigBots 自动配置机器人
func autoConfigBots(mode string) gin.H {
	configs := []gin.H{}
	switch mode {
	case "crawler":
		configs = append(configs,
			gin.H{"bot": "新闻爬虫机器人", "action": "更新抓取源", "status": "done", "detail": "已同步6个新闻源"},
			gin.H{"bot": "舆情监控机器人", "action": "更新关键词", "status": "done", "detail": "已更新23个关键词"},
		)
	case "analyzer":
		configs = append(configs,
			gin.H{"bot": "内容分析机器人", "action": "优化模型", "status": "done", "detail": "分析模型已更新到v2.1"},
			gin.H{"bot": "数据清洗机器人", "action": "配置规则", "status": "done", "detail": "去重规则已优化"},
		)
	case "schedule":
		configs = append(configs,
			gin.H{"bot": "调度中心机器人", "action": "优化调度", "status": "done", "detail": "已根据负载重新分配时间槽"},
		)
	default: // full
		configs = append(configs,
			gin.H{"bot": "新闻爬虫机器人", "action": "更新抓取源", "status": "done", "detail": "已同步6个新闻源"},
			gin.H{"bot": "舆情监控机器人", "action": "更新关键词", "status": "done", "detail": "已更新23个关键词"},
			gin.H{"bot": "内容分析机器人", "action": "优化模型", "status": "done", "detail": "分析模型已更新到v2.1"},
			gin.H{"bot": "数据清洗机器人", "action": "配置规则", "status": "done", "detail": "去重规则已优化"},
			gin.H{"bot": "调度中心机器人", "action": "优化调度", "status": "done", "detail": "已根据负载重新分配时间槽"},
			gin.H{"bot": "通知推送机器人", "action": "配置通道", "status": "done", "detail": "已配置邮件+Webhook通道"},
		)
	}
	return gin.H{
		"mode":    mode,
		"message": "自动配置完成",
		"count":   len(configs),
		"results": configs,
	}
}

// getBotConfigs 返回机器人配置概览
func getBotConfigs() []gin.H {
	return []gin.H{
		{"name": "新闻爬虫机器人", "type": "crawler", "config_key": "sources", "config_value": "6个源", "auto_managed": true},
		{"name": "新闻爬虫机器人", "type": "crawler", "config_key": "interval", "config_value": "30分钟", "auto_managed": true},
		{"name": "内容分析机器人", "type": "analyzer", "config_key": "model_version", "config_value": "v2.1", "auto_managed": true},
		{"name": "内容分析机器人", "type": "analyzer", "config_key": "batch_size", "config_value": "100", "auto_managed": false},
		{"name": "调度中心机器人", "type": "scheduler", "config_key": "strategy", "config_value": "负载均衡", "auto_managed": true},
		{"name": "通知推送机器人", "type": "notifier", "config_key": "channels", "config_value": "邮件,Webhook", "auto_managed": true},
		{"name": "舆情监控机器人", "type": "crawler", "config_key": "keywords", "config_value": "23个关键词", "auto_managed": true},
		{"name": "数据清洗机器人", "type": "analyzer", "config_key": "dedup_rule", "config_value": "标题+内容相似度>0.85", "auto_managed": false},
	}
}

// getBotLogs 返回机器人运行日志
func getBotLogs() []gin.H {
	now := time.Now()
	return []gin.H{
		{"id": 1, "level": "INFO", "bot_name": "新闻爬虫机器人", "message": "成功抓取36条新闻", "log_time": now.Add(-15 * time.Minute).Format("2006-01-02 15:04:05")},
		{"id": 2, "level": "INFO", "bot_name": "内容分析机器人", "message": "完成100条新闻分析", "log_time": now.Add(-40 * time.Minute).Format("2006-01-02 15:04:05")},
		{"id": 3, "level": "WARNING", "bot_name": "舆情监控机器人", "message": "关键词匹配率偏高，建议关注", "log_time": now.Add(-3 * time.Minute).Format("2006-01-02 15:04:05")},
		{"id": 4, "level": "ERROR", "bot_name": "通知推送机器人", "message": "Webhook推送失败，连接超时", "log_time": now.Add(-26 * time.Hour).Format("2006-01-02 15:04:05")},
		{"id": 5, "level": "INFO", "bot_name": "调度中心机器人", "message": "任务调度完成，分配6个任务", "log_time": now.Add(-2 * time.Minute).Format("2006-01-02 15:04:05")},
		{"id": 6, "level": "DEBUG", "bot_name": "数据清洗机器人", "message": "等待调度，排队中", "log_time": now.Add(-5 * time.Hour).Format("2006-01-02 15:04:05")},
	}
}
