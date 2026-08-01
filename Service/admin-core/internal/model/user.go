package model

import "time"

// User 用户模型
type User struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Username  string    `json:"username" gorm:"uniqueIndex;size:64;not null"`
	Password  string    `json:"-" gorm:"size:256;not null"`
	Nickname  string    `json:"nickname" gorm:"size:128"`
	Email     string    `json:"email" gorm:"size:128"`
	Phone     string    `json:"phone" gorm:"size:32"`
	Role      string    `json:"role" gorm:"size:32;default:user;index"`
	Avatar    string    `json:"avatar" gorm:"size:512"`
	Bio       string    `json:"bio" gorm:"size:512"`
	Status    int       `json:"status" gorm:"default:1;comment:1=正常 0=禁用"`
	LastLogin time.Time `json:"last_login"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// Role 角色模型
type Role struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string    `json:"name" gorm:"uniqueIndex;size:64;not null"`
	Code        string    `json:"code" gorm:"uniqueIndex;size:64;not null"`
	Description string    `json:"description" gorm:"size:256"`
	Status      int       `json:"status" gorm:"default:1"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName 指定表名
func (Role) TableName() string {
	return "roles"
}

// Permission 权限模型（接口级+按钮级）
type Permission struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string    `json:"name" gorm:"size:64;not null"`
	Code        string    `json:"code" gorm:"uniqueIndex;size:128;not null"`
	Type        string    `json:"type" gorm:"size:16;comment:api=接口级 button=按钮级 menu=菜单级"`
	Method      string    `json:"method" gorm:"size:16;comment:GET/POST/PUT/DELETE"`
	Path        string    `json:"path" gorm:"size:256"`
	ParentID    int64     `json:"parent_id" gorm:"default:0"`
	Sort        int       `json:"sort" gorm:"default:0"`
	Description string    `json:"description" gorm:"size:256"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName 指定表名
func (Permission) TableName() string {
	return "permissions"
}

// RolePermission 角色-权限关联
type RolePermission struct {
	ID           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	RoleID       int64     `json:"role_id" gorm:"index;not null"`
	PermissionID int64     `json:"permission_id" gorm:"index;not null"`
	CreatedAt    time.Time `json:"created_at"`
}

// TableName 指定表名
func (RolePermission) TableName() string {
	return "role_permissions"
}

// UserRole 用户-角色关联（支持多角色）
type UserRole struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID    int64     `json:"user_id" gorm:"index;not null"`
	RoleID    int64     `json:"role_id" gorm:"index;not null"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 指定表名
func (UserRole) TableName() string {
	return "user_roles"
}

// AuditLog 操作审计日志
type AuditLog struct {
	ID         int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID     int64     `json:"user_id" gorm:"index"`
	Username   string    `json:"username" gorm:"size:64;index"`
	Action     string    `json:"action" gorm:"size:128;index"`
	Resource   string    `json:"resource" gorm:"size:256"`
	ResourceID string    `json:"resource_id" gorm:"size:64"`
	Method     string    `json:"method" gorm:"size:16"`
	Path       string    `json:"path" gorm:"size:512"`
	IP         string    `json:"ip" gorm:"size:64"`
	UserAgent  string    `json:"user_agent" gorm:"size:512"`
	Detail     string    `json:"detail" gorm:"type:text"`
	Status     int       `json:"status" gorm:"default:1;comment:1=成功 0=失败"`
	CreatedAt  time.Time `json:"created_at" gorm:"index"`
}

// TableName 指定表名
func (AuditLog) TableName() string {
	return "audit_logs"
}

// SystemConfig 系统配置（支持热更新）
type SystemConfig struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Key         string    `json:"key" gorm:"uniqueIndex;size:128;not null"`
	Value       string    `json:"value" gorm:"type:text"`
	Type        string    `json:"type" gorm:"size:32;default:string;comment:string/int/bool/json"`
	Description string    `json:"description" gorm:"size:256"`
	Editable    bool      `json:"editable" gorm:"default:1"`
	UpdatedAt   time.Time `json:"updated_at"`
	UpdatedBy   string    `json:"updated_by" gorm:"size:64"`
}

// TableName 指定表名
func (SystemConfig) TableName() string {
	return "system_configs"
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=6,max=128"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	User         *User  `json:"user"`
}

// RefreshTokenRequest 刷新Token请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}