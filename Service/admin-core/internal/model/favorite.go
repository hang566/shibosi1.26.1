package model

import "time"

// Favorite 收藏模型
type Favorite struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID    int64     `json:"user_id" gorm:"index;not null"`
	Type      string    `json:"type" gorm:"size:32;default:website;index;comment:website/article/video/image"`
	Source    string    `json:"source" gorm:"size:64;index;comment:来源模块"`
	Title     string    `json:"title" gorm:"size:512;not null"`
	URL       string    `json:"url" gorm:"size:1024;index"`
	Content   string    `json:"content" gorm:"type:text"`
	Thumbnail string    `json:"thumbnail" gorm:"size:512"`
	Tags      string    `json:"tags" gorm:"size:512;comment:逗号分隔的标签"`
	IsPrivate bool      `json:"is_private" gorm:"default:false"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

// TableName 指定表名
func (Favorite) TableName() string {
	return "favorites"
}

// History 浏览历史模型
type History struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID    int64     `json:"user_id" gorm:"index;not null"`
	Type      string    `json:"type" gorm:"size:32;default:website;index;comment:website/article/video/image"`
	Source    string    `json:"source" gorm:"size:64;index;comment:来源模块"`
	Title     string    `json:"title" gorm:"size:512"`
	URL       string    `json:"url" gorm:"size:1024;index"`
	Content   string    `json:"content" gorm:"type:text"`
	Thumbnail string    `json:"thumbnail" gorm:"size:512"`
	Duration  int       `json:"duration" gorm:"default:0;comment:停留时间(秒)"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

// TableName 指定表名
func (History) TableName() string {
	return "history"
}

// CreateFavoriteRequest 创建收藏请求
type CreateFavoriteRequest struct {
	Type      string `json:"type" binding:"omitempty,oneof=website article video image"`
	Source    string `json:"source" binding:"omitempty,max=64"`
	Title     string `json:"title" binding:"required,max=512"`
	URL       string `json:"url" binding:"omitempty,max=1024"`
	Content   string `json:"content"`
	Thumbnail string `json:"thumbnail" binding:"omitempty,max=512"`
	Tags      string `json:"tags" binding:"omitempty,max=512"`
	IsPrivate bool   `json:"is_private"`
}

// CreateHistoryRequest 创建历史记录请求
type CreateHistoryRequest struct {
	Type      string `json:"type" binding:"omitempty,oneof=website article video image"`
	Source    string `json:"source" binding:"omitempty,max=64"`
	Title     string `json:"title" binding:"omitempty,max=512"`
	URL       string `json:"url" binding:"required,max=1024"`
	Content   string `json:"content"`
	Thumbnail string `json:"thumbnail" binding:"omitempty,max=512"`
	Duration  int    `json:"duration"`
}

// UpdateFavoriteRequest 更新收藏请求
type UpdateFavoriteRequest struct {
	Title     *string `json:"title" binding:"omitempty,max=512"`
	Content   *string `json:"content"`
	Thumbnail *string `json:"thumbnail" binding:"omitempty,max=512"`
	Tags      *string `json:"tags" binding:"omitempty,max=512"`
	IsPrivate *bool   `json:"is_private"`
}

// UserData 通用用户数据模型（KV 存储，用于积分、设置等）
type UserData struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID    int64     `json:"user_id" gorm:"index;not null"`
	Key       string    `json:"key" gorm:"size:64;index;not null"`
	Value     string    `json:"value" gorm:"type:text"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (UserData) TableName() string {
	return "user_data"
}

// UpdateUserDataRequest 更新用户数据请求
type UpdateUserDataRequest struct {
	Key   string `json:"key" binding:"required,max=64"`
	Value string `json:"value"`
}
