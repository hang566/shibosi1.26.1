package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config 全局配置结构
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Security SecurityConfig `mapstructure:"security"`
	Log      LogConfig      `mapstructure:"log"`

	// ===== 万能管理后台 - 动态配置 =====
	Services map[string]ServiceConfig `mapstructure:"services"` // 服务注册表
	Modules  map[string]ModuleConfig  `mapstructure:"modules"`  // 模块/页面定义
}

// ServiceConfig 服务配置 - 支持静态配置 + 动态注册
type ServiceConfig struct {
	Name         string   `json:"name" mapstructure:"name"`
	Port         int      `json:"port" mapstructure:"port"`
	BaseURL      string   `json:"base_url" mapstructure:"base_url"`
	HealthPath   string   `json:"health_path" mapstructure:"health_path"`
	WebURL       string   `json:"web_url,omitempty" mapstructure:"web_url"` // 前端访问地址（可嵌入iframe）
	Icon         string   `json:"icon" mapstructure:"icon"`
	Tags         []string `json:"tags" mapstructure:"tags"`
	Version      string   `json:"version,omitempty" mapstructure:"version"`
	Status       string   `json:"status" mapstructure:"status"` // online/offline/unknown
	LastCheck    string   `json:"last_check,omitempty" mapstructure:"last_check"`
	Capabilities []string `json:"capabilities,omitempty" mapstructure:"capabilities"`
	Description  string   `json:"description,omitempty" mapstructure:"description"`
	StartCommand string   `json:"start_command,omitempty" mapstructure:"start_command"` // 启动命令
	StopCommand  string   `json:"stop_command,omitempty" mapstructure:"stop_command"`   // 停止命令
	WorkDir      string   `json:"work_dir,omitempty" mapstructure:"work_dir"`           // 工作目录（相对 admin-core）
}

// ModuleConfig 模块配置 - 侧边栏分组
type ModuleConfig struct {
	Name  string       `json:"name" mapstructure:"name"`
	Icon  string       `json:"icon" mapstructure:"icon"`
	Order int          `json:"order" mapstructure:"order"`
	Pages []PageConfig `json:"pages" mapstructure:"pages"`
}

// PageConfig 页面配置 - 由 Widget 组合
type PageConfig struct {
	ID          string                 `json:"id" mapstructure:"id"`
	Title       string                 `json:"title" mapstructure:"title"`
	Type        string                 `json:"type" mapstructure:"type"` // overview/stats/table/logs/monitor/custom
	Description string                 `json:"description,omitempty" mapstructure:"description"`
	Icon        string                 `json:"icon,omitempty" mapstructure:"icon"`
	Widgets     []WidgetConfig         `json:"widgets,omitempty" mapstructure:"widgets"`
	DataSource  *DataSourceConfig      `json:"data_source,omitempty" mapstructure:"data_source"`
	Columns     []ColumnConfig         `json:"columns,omitempty" mapstructure:"columns"`
	Actions     []ActionConfig         `json:"actions,omitempty" mapstructure:"actions"`
	LogLevels   []string               `json:"log_levels,omitempty" mapstructure:"log_levels"`
	Fields      []string               `json:"fields,omitempty" mapstructure:"fields"`
	ServicesRef string                 `json:"services_ref,omitempty" mapstructure:"services_ref"`
	IFrameURL   string                 `json:"iframe_url,omitempty" mapstructure:"iframe_url"`
	HTML        string                 `json:"html,omitempty" mapstructure:"html"`
	Extra       map[string]interface{} `json:"extra,omitempty" mapstructure:"extra"`
}

// WidgetConfig Widget 配置 - 可组合的 UI 组件
type WidgetConfig struct {
	Type       string                 `json:"type" mapstructure:"type"` // stat-card/data-table/line-chart/bar-chart/log-viewer/service-card/action-group/iframe
	Title      string                 `json:"title,omitempty" mapstructure:"title"`
	Icon       string                 `json:"icon,omitempty" mapstructure:"icon"`
	DataSource string                 `json:"data_source,omitempty" mapstructure:"data_source"`
	Config     map[string]interface{} `json:"config,omitempty" mapstructure:"config"`
	Order      int                    `json:"order,omitempty" mapstructure:"order"`
}

// DataSourceConfig 数据源配置
type DataSourceConfig struct {
	URL        string `json:"url" mapstructure:"url"`
	Method     string `json:"method,omitempty" mapstructure:"method"`
	Transform  string `json:"transform,omitempty" mapstructure:"transform"`
	AutoUpdate int    `json:"auto_update,omitempty" mapstructure:"auto_update"` // 秒
}

// ColumnConfig 表格列配置
type ColumnConfig struct {
	Key    string            `json:"key" mapstructure:"key"`
	Label  string            `json:"label" mapstructure:"label"`
	Width  string            `json:"width,omitempty" mapstructure:"width"`
	Render string            `json:"render,omitempty" mapstructure:"render"` // text/tag/boolean/datetime/number/truncate
	TagMap map[string]string `json:"tag_map,omitempty" mapstructure:"tag_map"`
}

// ActionConfig 操作按钮配置
type ActionConfig struct {
	Type    string        `json:"type" mapstructure:"type"` // fetch/create/edit/delete/custom
	Label   string        `json:"label" mapstructure:"label"`
	Method  string        `json:"method,omitempty" mapstructure:"method"`
	URL     string        `json:"url,omitempty" mapstructure:"url"`
	Body    interface{}   `json:"body,omitempty" mapstructure:"body"`
	Confirm bool          `json:"confirm,omitempty" mapstructure:"confirm"`
	Danger  bool          `json:"danger,omitempty" mapstructure:"danger"`
	Fields  []FieldConfig `json:"fields,omitempty" mapstructure:"fields"`
}

// FieldConfig 表单字段配置
type FieldConfig struct {
	Key     string   `json:"key" mapstructure:"key"`
	Label   string   `json:"label" mapstructure:"label"`
	Type    string   `json:"type,omitempty" mapstructure:"type"` // text/number/select/textarea
	Options []string `json:"options,omitempty" mapstructure:"options"`
}

type ServerConfig struct {
	Port    int    `mapstructure:"port"`
	Mode    string `mapstructure:"mode"` // debug | release | test
	TLSCert string `mapstructure:"tls_cert"`
	TLSKey  string `mapstructure:"tls_key"`
}

type DatabaseConfig struct {
	Driver          string `mapstructure:"driver"` // sqlite | mysql | postgres
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	DBName          string `mapstructure:"dbname"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"` // 秒
	// 读写分离
	ReadHosts []string `mapstructure:"read_hosts"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type JWTConfig struct {
	Secret           string `mapstructure:"secret"`
	AccessTokenTTL   int    `mapstructure:"access_token_ttl"`  // 分钟
	RefreshTokenTTL  int    `mapstructure:"refresh_token_ttl"` // 分钟
	BlacklistEnabled bool   `mapstructure:"blacklist_enabled"`
}

type SecurityConfig struct {
	BcryptCost       int      `mapstructure:"bcrypt_cost"`
	RateLimitQPS     int      `mapstructure:"rate_limit_qps"`
	RateLimitBurst   int      `mapstructure:"rate_limit_burst"`
	AdminIPWhitelist []string `mapstructure:"admin_ip_whitelist"`
	CORSOrigins      []string `mapstructure:"cors_origins"`
	EnableTLS        bool     `mapstructure:"enable_tls"`
}

type LogConfig struct {
	Level      string `mapstructure:"level"` // debug | info | warn | error
	FilePath   string `mapstructure:"file_path"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAgeDays int    `mapstructure:"max_age_days"`
}

// Load 从配置文件和环境变量加载配置
// 优先级: 环境变量 > 配置文件 > 默认值
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// 默认配置
	setDefaults(v)

	// 读取配置文件
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./config")
		v.AddConfigPath(".")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
		// 配置文件不存在时使用默认值+环境变量
		fmt.Println("[config] 未找到配置文件，使用默认值与环境变量")
	}

	// 绑定环境变量 (支持敏感信息注入)
	v.SetEnvPrefix("SHIBOSI")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 关键敏感配置从环境变量读取
	bindEnvOverride(v, "database.password", "DB_PASSWORD")
	bindEnvOverride(v, "database.host", "DB_HOST")
	bindEnvOverride(v, "database.port", "DB_PORT")
	bindEnvOverride(v, "redis.password", "REDIS_PASSWORD")
	bindEnvOverride(v, "redis.host", "REDIS_HOST")
	bindEnvOverride(v, "jwt.secret", "JWT_SECRET")
	bindEnvOverride(v, "server.tls_cert", "TLS_CERT")
	bindEnvOverride(v, "server.tls_key", "TLS_KEY")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "release")
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.dbname", "data/admin.db")
	v.SetDefault("database.host", "127.0.0.1")
	v.SetDefault("database.port", 3306)
	v.SetDefault("database.max_open_conns", 100)
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("database.conn_max_lifetime", 3600)
	v.SetDefault("redis.host", "127.0.0.1")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 100)
	v.SetDefault("jwt.access_token_ttl", 30)
	v.SetDefault("jwt.refresh_token_ttl", 10080) // 7天
	v.SetDefault("jwt.blacklist_enabled", true)
	v.SetDefault("security.bcrypt_cost", 12)
	v.SetDefault("security.rate_limit_qps", 100)
	v.SetDefault("security.rate_limit_burst", 200)
	v.SetDefault("security.enable_tls", true)
	v.SetDefault("log.level", "info")
	v.SetDefault("log.max_size_mb", 100)
	v.SetDefault("log.max_backups", 10)
	v.SetDefault("log.max_age_days", 30)
}

// bindEnvOverride 绑定环境变量覆盖配置
func bindEnvOverride(v *viper.Viper, configKey, envVar string) {
	if val := os.Getenv(envVar); val != "" {
		v.Set(configKey, val)
	}
}
