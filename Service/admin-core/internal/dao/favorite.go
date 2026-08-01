package dao

import (
	"admin-core/internal/model"
	"time"

	"gorm.io/gorm"
)

// FavoriteDAO 收藏数据访问对象
type FavoriteDAO struct {
	db *gorm.DB
}

// NewFavoriteDAO 创建FavoriteDAO
func NewFavoriteDAO(db *gorm.DB) *FavoriteDAO {
	return &FavoriteDAO{db: db}
}

// Create 创建收藏
func (d *FavoriteDAO) Create(fav *model.Favorite) error {
	fav.CreatedAt = time.Now()
	return d.db.Create(fav).Error
}

// GetByID 根据ID获取收藏
func (d *FavoriteDAO) GetByID(id, userID int64) (*model.Favorite, error) {
	var fav model.Favorite
	err := d.db.Where("id = ? AND user_id = ?", id, userID).First(&fav).Error
	if err != nil {
		return nil, err
	}
	return &fav, nil
}

// ListByUser 分页查询用户收藏列表
func (d *FavoriteDAO) ListByUser(userID int64, page, pageSize int, favType, keyword string) ([]model.Favorite, int64, error) {
	var favs []model.Favorite
	var total int64

	query := d.db.Model(&model.Favorite{}).Where("user_id = ?", userID)

	if favType != "" {
		query = query.Where("type = ?", favType)
	}
	if keyword != "" {
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&favs).Error
	return favs, total, err
}

// Update 更新收藏
func (d *FavoriteDAO) Update(id, userID int64, updates map[string]interface{}) error {
	return d.db.Model(&model.Favorite{}).Where("id = ? AND user_id = ?", id, userID).Updates(updates).Error
}

// Delete 删除收藏
func (d *FavoriteDAO) Delete(id, userID int64) error {
	return d.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Favorite{}).Error
}

// DeleteBatch 批量删除收藏
func (d *FavoriteDAO) DeleteBatch(ids []int64, userID int64) error {
	return d.db.Where("id IN ? AND user_id = ?", ids, userID).Delete(&model.Favorite{}).Error
}

// CountByUser 统计用户收藏数量
func (d *FavoriteDAO) CountByUser(userID int64) (int64, error) {
	var count int64
	err := d.db.Model(&model.Favorite{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

// ExistsByURL 检查用户是否已收藏某URL
func (d *FavoriteDAO) ExistsByURL(userID int64, url string) (bool, int64, error) {
	var fav model.Favorite
	err := d.db.Where("user_id = ? AND url = ?", userID, url).First(&fav).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, 0, nil
		}
		return false, 0, err
	}
	return true, fav.ID, nil
}

// HistoryDAO 历史记录数据访问对象
type HistoryDAO struct {
	db *gorm.DB
}

// NewHistoryDAO 创建HistoryDAO
func NewHistoryDAO(db *gorm.DB) *HistoryDAO {
	return &HistoryDAO{db: db}
}

// Create 创建历史记录
func (d *HistoryDAO) Create(history *model.History) error {
	history.CreatedAt = time.Now()
	return d.db.Create(history).Error
}

// GetByID 根据ID获取历史记录
func (d *HistoryDAO) GetByID(id, userID int64) (*model.History, error) {
	var history model.History
	err := d.db.Where("id = ? AND user_id = ?", id, userID).First(&history).Error
	if err != nil {
		return nil, err
	}
	return &history, nil
}

// ListByUser 分页查询用户历史记录列表
func (d *HistoryDAO) ListByUser(userID int64, page, pageSize int, historyType, keyword string) ([]model.History, int64, error) {
	var histories []model.History
	var total int64

	query := d.db.Model(&model.History{}).Where("user_id = ?", userID)

	if historyType != "" {
		query = query.Where("type = ?", historyType)
	}
	if keyword != "" {
		query = query.Where("title LIKE ? OR url LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&histories).Error
	return histories, total, err
}

// Update 更新历史记录
func (d *HistoryDAO) Update(id, userID int64, updates map[string]interface{}) error {
	return d.db.Model(&model.History{}).Where("id = ? AND user_id = ?", id, userID).Updates(updates).Error
}

// Delete 删除历史记录
func (d *HistoryDAO) Delete(id, userID int64) error {
	return d.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.History{}).Error
}

// DeleteBatch 批量删除历史记录
func (d *HistoryDAO) DeleteBatch(ids []int64, userID int64) error {
	return d.db.Where("id IN ? AND user_id = ?", ids, userID).Delete(&model.History{}).Error
}

// ClearByUser 清空用户历史记录
func (d *HistoryDAO) ClearByUser(userID int64) error {
	return d.db.Where("user_id = ?", userID).Delete(&model.History{}).Error
}

// CountByUser 统计用户历史记录数量
func (d *HistoryDAO) CountByUser(userID int64) (int64, error) {
	var count int64
	err := d.db.Model(&model.History{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

// CleanOld 清理指定天数前的历史记录
func (d *HistoryDAO) CleanOld(days int) (int64, error) {
	result := d.db.Where("created_at < ?", time.Now().AddDate(0, 0, -days)).Delete(&model.History{})
	return result.RowsAffected, result.Error
}

// UserDataDAO 用户通用数据 DAO
type UserDataDAO struct {
	db *gorm.DB
}

// NewUserDataDAO 创建 UserDataDAO
func NewUserDataDAO(db *gorm.DB) *UserDataDAO {
	return &UserDataDAO{db: db}
}

// Get 按 key 获取单条用户数据
func (d *UserDataDAO) Get(userID int64, key string) (*model.UserData, error) {
	var ud model.UserData
	err := d.db.Where("user_id = ? AND key = ?", userID, key).First(&ud).Error
	if err != nil {
		return nil, err
	}
	return &ud, nil
}

// List 列出用户所有数据
func (d *UserDataDAO) List(userID int64) ([]model.UserData, error) {
	var uds []model.UserData
	err := d.db.Where("user_id = ?", userID).Find(&uds).Error
	return uds, err
}

// Upsert 插入或更新用户数据
func (d *UserDataDAO) Upsert(userID int64, key, value string) (*model.UserData, error) {
	var ud model.UserData
	now := time.Now()
	err := d.db.Where("user_id = ? AND key = ?", userID, key).First(&ud).Error
	if err == nil {
		// 存在则更新
		ud.Value = value
		ud.UpdatedAt = now
		if err := d.db.Save(&ud).Error; err != nil {
			return nil, err
		}
		return &ud, nil
	}
	// 不存在则创建
	ud = model.UserData{
		UserID:    userID,
		Key:       key,
		Value:     value,
		UpdatedAt: now,
	}
	if err := d.db.Create(&ud).Error; err != nil {
		return nil, err
	}
	return &ud, nil
}

// Delete 删除指定 key 的数据
func (d *UserDataDAO) Delete(userID int64, key string) error {
	return d.db.Where("user_id = ? AND key = ?", userID, key).Delete(&model.UserData{}).Error
}

// DeleteAll 清空用户所有数据
func (d *UserDataDAO) DeleteAll(userID int64) error {
	return d.db.Where("user_id = ?", userID).Delete(&model.UserData{}).Error
}
