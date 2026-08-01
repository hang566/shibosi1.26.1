package config

// GetDefaultModules 获取默认模块配置
// 这些配置可以被 config.yaml 覆盖
func GetDefaultModules() map[string]ModuleConfig {
	return map[string]ModuleConfig{
		// ========== 系统管理 ==========
		"system": {
			Name:  "系统管理",
			Icon:  "🔧",
			Order: 1,
			Pages: []PageConfig{
				{
					ID:    "dashboard",
					Title: "仪表盘",
					Type:  "stats",
					Icon:  "📊",
					Widgets: []WidgetConfig{
						{
							Type:       "stat-cards",
							Title:      "核心指标",
							DataSource: "/api/v1/admin/dashboard",
							Order:      1,
							Config: map[string]interface{}{
								"auto_update": 30,
							},
						},
						{
							Type:       "system-info",
							Title:      "系统状态",
							DataSource: "/api/v1/admin/system-status",
							Order:      2,
						},
					},
				},
				{
					ID:    "users",
					Title: "用户管理",
					Type:  "table",
					Icon:  "👥",
					Columns: []ColumnConfig{
						{Key: "id", Label: "ID", Width: "60px"},
						{Key: "username", Label: "用户名"},
						{Key: "nickname", Label: "昵称"},
						{Key: "email", Label: "邮箱", Render: "text"},
						{Key: "role", Label: "角色", Render: "tag", TagMap: map[string]string{
							"admin": "danger",
							"user":  "primary",
							"guest": "default",
						}},
						{Key: "status", Label: "状态", Render: "boolean"},
						{Key: "created_at", Label: "创建时间", Render: "datetime"},
					},
					Actions: []ActionConfig{
						{Type: "create", Label: "新增用户", Method: "POST", URL: "/api/v1/admin/users"},
						{Type: "toggle_status", Label: "启/禁用", Method: "PUT", URL: "/api/v1/admin/users/:id/status"},
						{Type: "delete", Label: "删除", Method: "DELETE", URL: "/api/v1/admin/users/:id", Confirm: true, Danger: true},
					},
					DataSource: &DataSourceConfig{URL: "/api/v1/admin/users"},
				},
				{
					ID:    "roles",
					Title: "角色权限",
					Type:  "table",
					Icon:  "🔐",
					Columns: []ColumnConfig{
						{Key: "id", Label: "ID", Width: "60px"},
						{Key: "name", Label: "名称"},
						{Key: "code", Label: "编码"},
						{Key: "description", Label: "描述"},
						{Key: "status", Label: "状态", Render: "boolean"},
					},
					Actions: []ActionConfig{
						{Type: "create", Label: "新增角色", Method: "POST", URL: "/api/v1/admin/roles"},
						{Type: "permissions", Label: "权限分配", Method: "PUT", URL: "/api/v1/admin/roles/:id/permissions"},
						{Type: "delete", Label: "删除", Method: "DELETE", URL: "/api/v1/admin/roles/:id", Confirm: true},
					},
					DataSource: &DataSourceConfig{URL: "/api/v1/admin/roles"},
				},
				{
					ID:         "audit",
					Title:      "操作日志",
					Type:       "logs",
					Icon:       "📋",
					DataSource: &DataSourceConfig{URL: "/api/v1/admin/audit-logs"},
					Fields:     []string{"id", "username", "action", "resource", "ip", "created_at", "status", "detail"},
				},
				{
					ID:    "config",
					Title: "系统配置",
					Type:  "table",
					Icon:  "⚙️",
					Columns: []ColumnConfig{
						{Key: "key", Label: "配置键", Render: "text"},
						{Key: "value", Label: "值"},
						{Key: "type", Label: "类型"},
						{Key: "description", Label: "描述"},
					},
					Actions: []ActionConfig{
						{Type: "update", Label: "保存", Method: "PUT", URL: "/api/v1/admin/configs/:key"},
					},
					DataSource: &DataSourceConfig{URL: "/api/v1/admin/configs"},
				},
			},
		},

		// ========== 新闻服务 ==========
		"news": {
			Name:  "新闻服务",
			Icon:  "📰",
			Order: 2,
			Pages: []PageConfig{
				{
					ID:    "news-overview",
					Title: "新闻总览",
					Type:  "overview",
					Icon:  "📊",
					Widgets: []WidgetConfig{
						{
							Type:       "stat-cards",
							Title:      "新闻统计",
							DataSource: "/api/v1/proxy/newspaper/api/admin/overview",
							Order:      1,
							Config: map[string]interface{}{
								"transform":   "news_overview",
								"auto_update": 30,
							},
						},
						{
							Type:       "system-info",
							Title:      "服务状态",
							DataSource: "/api/v1/proxy/newspaper/api/admin/overview",
							Order:      2,
						},
					},
				},
				{
					ID:    "news-analysis",
					Title: "新闻分析",
					Type:  "custom",
					Icon:  "📈",
					HTML:  "news-analysis", // 特殊类型，使用自定义渲染
					Widgets: []WidgetConfig{
						{
							Type:       "action-group",
							Title:      "分析操作",
							DataSource: "",
							Order:      1,
							Config: map[string]interface{}{
								"actions": []ActionConfig{
									{Type: "custom", Label: "分析最新100条", Method: "POST", URL: "/api/v1/proxy/newspaper/api/admin/analysis", Body: map[string]int{"limit": 100}},
									{Type: "custom", Label: "分析最新500条", Method: "POST", URL: "/api/v1/proxy/newspaper/api/admin/analysis", Body: map[string]int{"limit": 500}},
									{Type: "danger", Label: "清除所有分析", Method: "POST", URL: "/api/v1/proxy/newspaper/api/news", Body: map[string]string{"action": "clear_analysis"}, Confirm: true},
								},
							},
						},
						{
							Type:       "bar-chart",
							Title:      "分析结果分布",
							DataSource: "/api/v1/proxy/newspaper/api/admin/analysis",
							Order:      2,
							Config: map[string]interface{}{
								"chart_type": "analysis",
							},
						},
					},
				},
				{
					ID:    "news-sources",
					Title: "新闻源管理",
					Type:  "table",
					Icon:  "📡",
					Columns: []ColumnConfig{
						{Key: "id", Label: "ID"},
						{Key: "name", Label: "名称"},
						{Key: "url", Label: "URL", Render: "truncate"},
						{Key: "type", Label: "类型"},
						{Key: "category", Label: "分类"},
						{Key: "enabled", Label: "状态", Render: "boolean"},
						{Key: "last_fetch", Label: "最后抓取", Render: "datetime"},
					},
					Actions: []ActionConfig{
						{Type: "fetch", Label: "抓取所有源", Method: "POST", URL: "/api/v1/proxy/newspaper/api/admin/fetch"},
						{Type: "create", Label: "添加新闻源", Fields: []FieldConfig{
							{Key: "name", Label: "名称"},
							{Key: "url", Label: "URL"},
							{Key: "type", Label: "类型", Type: "select", Options: []string{"rss", "atom"}},
							{Key: "category", Label: "分类", Type: "select", Options: []string{"时政", "财经", "科技", "体育", "综合", "娱乐"}},
						}},
						{Type: "delete", Label: "删除", Method: "DELETE", URL: "/api/v1/proxy/newspaper/api/admin/sources/:id", Danger: true, Confirm: true},
					},
					DataSource: &DataSourceConfig{URL: "/api/v1/proxy/newspaper/api/admin/sources"},
				},
				{
					ID:         "news-logs",
					Title:      "系统日志",
					Type:       "logs",
					Icon:       "📝",
					DataSource: &DataSourceConfig{URL: "/api/v1/proxy/newspaper/api/admin/logs"},
					LogLevels:  []string{"DEBUG", "INFO", "WARNING", "ERROR", "FATAL"},
					Fields:     []string{"id", "level", "module", "message", "log_time"},
				},
				{
					ID:    "news-database",
					Title: "数据库维护",
					Type:  "custom",
					Icon:  "💾",
					Widgets: []WidgetConfig{
						{
							Type:  "action-group",
							Title: "数据库操作",
							Order: 1,
							Config: map[string]interface{}{
								"actions": []ActionConfig{
									{Type: "create", Label: "创建备份", Method: "POST", URL: "/api/v1/proxy/newspaper/api/admin/backup"},
									{Type: "custom", Label: "清理7天前新闻", Method: "POST", URL: "/api/v1/proxy/newspaper/api/admin/clean", Body: map[string]int{"days": 7}, Confirm: true},
									{Type: "custom", Label: "清理30天前新闻", Method: "POST", URL: "/api/v1/proxy/newspaper/api/admin/clean", Body: map[string]int{"days": 30}, Confirm: true},
									{Type: "custom", Label: "VACUUM 回收空间", Method: "POST", URL: "/api/v1/proxy/newspaper/api/admin/database", Body: map[string]string{"action": "vacuum"}},
									{Type: "danger", Label: "清空所有数据", Method: "POST", URL: "/api/v1/proxy/newspaper/api/admin/clean", Body: map[string]string{"action": "all"}, Confirm: true, Danger: true},
								},
							},
						},
						{
							Type:       "data-table",
							Title:      "数据库状态",
							DataSource: "/api/v1/proxy/newspaper/api/admin/database",
							Order:      2,
							Config: map[string]interface{}{
								"readonly": true,
							},
						},
					},
				},
			},
		},

		// ========== 算法机器人 ==========
		"algorithm-bots": {
			Name:  "算法机器人",
			Icon:  "🤖",
			Order: 3,
			Pages: []PageConfig{
				{
					ID:    "bot-overview",
					Title: "机器人总览",
					Type:  "custom",
					Icon:  "📊",
					Extra: map[string]interface{}{"renderer": "renderBotOverviewPage"},
				},
				{
					ID:    "bot-manage",
					Title: "机器人管理",
					Type:  "custom",
					Icon:  "🤖",
					Extra: map[string]interface{}{"renderer": "renderBotManagePage"},
				},
				{
					ID:    "bot-config",
					Title: "自动配置",
					Type:  "custom",
					Icon:  "⚙️",
					Extra: map[string]interface{}{"renderer": "renderBotConfigPage"},
				},
				{
					ID:    "bot-logs",
					Title: "运行日志",
					Type:  "custom",
					Icon:  "📝",
					Extra: map[string]interface{}{"renderer": "renderBotLogsPage"},
				},
			},
		},

		// ========== 服务监控 ==========
		"monitoring": {
			Name:  "服务监控",
			Icon:  "🖥️",
			Order: 99,
			Pages: []PageConfig{
				{
					ID:          "service-portal",
					Title:       "服务门户",
					Type:        "portal",
					Icon:        "🚪",
					Description: "点击卡片打开对应服务",
				},
				{
					ID:          "service-monitor",
					Title:       "服务状态",
					Type:        "monitor",
					Icon:        "🖥️",
					ServicesRef: "services",
					Widgets: []WidgetConfig{
						{
							Type:       "service-cards",
							Title:      "服务概览",
							DataSource: "/api/v1/admin/services",
							Order:      1,
							Config: map[string]interface{}{
								"enable_actions": true,
								"batch_actions":  true,
								"auto_update":    10,
							},
						},
					},
				},
			},
		},
	}
}

// GetDefaultServices 获取默认服务配置
func GetDefaultServices() map[string]ServiceConfig {
	return map[string]ServiceConfig{
		"admin-core": {
			Name:        "管理后台",
			Port:        8084,
			BaseURL:     "http://localhost:8084",
			WebURL:      "http://localhost:8084/admin",
			HealthPath:  "/api/health",
			Icon:        "⚙️",
			Tags:        []string{"core", "admin"},
			Status:      "unknown",
			Description: "统一管理后台核心服务",
		},
		"newspaper": {
			Name:        "新闻服务",
			Port:        8082,
			BaseURL:     "http://localhost:8082",
			WebURL:      "http://localhost:8082",
			HealthPath:  "/api/health",
			Icon:        "📰",
			Tags:        []string{"news"},
			Status:      "unknown",
			Description: "新闻采集与分析服务",
		},
		"search-engine": {
			Name:        "搜索服务",
			Port:        8081,
			BaseURL:     "http://localhost:8081",
			WebURL:      "http://localhost:8081",
			HealthPath:  "/api/health",
			Icon:        "🔍",
			Tags:        []string{"search"},
			Status:      "unknown",
			Description: "全文搜索引擎",
		},
		"user-service": {
			Name:        "用户中心",
			Port:        8083,
			BaseURL:     "http://localhost:8083",
			WebURL:      "http://localhost:8083",
			HealthPath:  "/api/health",
			Icon:        "👤",
			Tags:        []string{"user"},
			Status:      "unknown",
			Description: "用户认证与中心服务",
		},
		"static": {
			Name:        "前端门户",
			Port:        8080,
			BaseURL:     "http://localhost:8080",
			WebURL:      "http://localhost:8080",
			HealthPath:  "/",
			Icon:        "🌐",
			Tags:        []string{"frontend"},
			Status:      "unknown",
			Description: "静态前端门户页面",
		},
		"algorithm-bot": {
			Name:        "算法机器人",
			Port:        8085,
			BaseURL:     "http://localhost:8085",
			WebURL:      "http://localhost:8085",
			HealthPath:  "/api/health",
			Icon:        "🤖",
			Tags:        []string{"ai", "bot"},
			Status:      "unknown",
			Description: "算法机器人管理与调度服务",
		},
	}
}
