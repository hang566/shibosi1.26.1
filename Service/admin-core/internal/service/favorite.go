package service

import (
	"admin-core/internal/dao"
	"admin-core/internal/model"
	"fmt"
)

// FavoriteService 收藏与历史服务
type FavoriteService struct {
	favoriteDAO *dao.FavoriteDAO
	historyDAO  *dao.HistoryDAO
}

// NewFavoriteService 创建FavoriteService
func NewFavoriteService(favoriteDAO *dao.FavoriteDAO, historyDAO *dao.HistoryDAO) *FavoriteService {
	return &FavoriteService{
		favoriteDAO: favoriteDAO,
		historyDAO:  historyDAO,
	}
}

// ============ 收藏操作 ============

// CreateFavorite 创建收藏
func (s *FavoriteService) CreateFavorite(userID int64, req *model.CreateFavoriteRequest) (*model.Favorite, error) {
	// 检查是否已收藏相同URL
	if req.URL != "" {
		exists, _, err := s.favoriteDAO.ExistsByURL(userID, req.URL)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, fmt.Errorf("已收藏过该链接")
		}
	}

	fav := &model.Favorite{
		UserID:    userID,
		Type:      req.Type,
		Source:    req.Source,
		Title:     req.Title,
		URL:       req.URL,
		Content:   req.Content,
		Thumbnail: req.Thumbnail,
		Tags:      req.Tags,
		IsPrivate: req.IsPrivate,
	}

	if fav.Type == "" {
		fav.Type = "website"
	}

	if err := s.favoriteDAO.Create(fav); err != nil {
		return nil, err
	}
	return fav, nil
}

// GetFavorite 获取单个收藏
func (s *FavoriteService) GetFavorite(id, userID int64) (*model.Favorite, error) {
	fav, err := s.favoriteDAO.GetByID(id, userID)
	if err != nil {
		return nil, fmt.Errorf("收藏不存在")
	}
	return fav, nil
}

// ListFavorites 分页查询收藏列表
func (s *FavoriteService) ListFavorites(userID int64, page, pageSize int, favType, keyword string) ([]model.Favorite, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.favoriteDAO.ListByUser(userID, page, pageSize, favType, keyword)
}

// UpdateFavorite 更新收藏
func (s *FavoriteService) UpdateFavorite(id, userID int64, req *model.UpdateFavoriteRequest) error {
	updates := make(map[string]interface{})

	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Content != nil {
		updates["content"] = *req.Content
	}
	if req.Thumbnail != nil {
		updates["thumbnail"] = *req.Thumbnail
	}
	if req.Tags != nil {
		updates["tags"] = *req.Tags
	}
	if req.IsPrivate != nil {
		updates["is_private"] = *req.IsPrivate
	}

	if len(updates) == 0 {
		return fmt.Errorf("没有需要更新的字段")
	}

	result := s.favoriteDAO.Update(id, userID, updates)
	if result == nil {
		// 检查是否存在
		_, err := s.favoriteDAO.GetByID(id, userID)
		if err != nil {
			return fmt.Errorf("收藏不存在")
		}
	}
	return result
}

// DeleteFavorite 删除收藏
func (s *FavoriteService) DeleteFavorite(id, userID int64) error {
	// 检查是否存在
	_, err := s.favoriteDAO.GetByID(id, userID)
	if err != nil {
		return fmt.Errorf("收藏不存在")
	}
	return s.favoriteDAO.Delete(id, userID)
}

// DeleteFavorites 批量删除收藏
func (s *FavoriteService) DeleteFavorites(ids []int64, userID int64) error {
	if len(ids) == 0 {
		return fmt.Errorf("请选择要删除的收藏")
	}
	return s.favoriteDAO.DeleteBatch(ids, userID)
}

// GetFavoriteCount 获取用户收藏数量
func (s *FavoriteService) GetFavoriteCount(userID int64) (int64, error) {
	return s.favoriteDAO.CountByUser(userID)
}

// CheckFavorite 检查是否已收藏
func (s *FavoriteService) CheckFavorite(userID int64, url string) (bool, int64, error) {
	if url == "" {
		return false, 0, nil
	}
	return s.favoriteDAO.ExistsByURL(userID, url)
}

// ============ 历史记录操作 ============

// CreateHistory 创建历史记录
func (s *FavoriteService) CreateHistory(userID int64, req *model.CreateHistoryRequest) (*model.History, error) {
	history := &model.History{
		UserID:    userID,
		Type:      req.Type,
		Source:    req.Source,
		Title:     req.Title,
		URL:       req.URL,
		Content:   req.Content,
		Thumbnail: req.Thumbnail,
		Duration:  req.Duration,
	}

	if history.Type == "" {
		history.Type = "website"
	}

	if err := s.historyDAO.Create(history); err != nil {
		return nil, err
	}
	return history, nil
}

// GetHistory 获取单个历史记录
func (s *FavoriteService) GetHistory(id, userID int64) (*model.History, error) {
	history, err := s.historyDAO.GetByID(id, userID)
	if err != nil {
		return nil, fmt.Errorf("历史记录不存在")
	}
	return history, nil
}

// ListHistories 分页查询历史记录列表
func (s *FavoriteService) ListHistories(userID int64, page, pageSize int, historyType, keyword string) ([]model.History, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 30
	}
	return s.historyDAO.ListByUser(userID, page, pageSize, historyType, keyword)
}

// DeleteHistory 删除历史记录
func (s *FavoriteService) DeleteHistory(id, userID int64) error {
	_, err := s.historyDAO.GetByID(id, userID)
	if err != nil {
		return fmt.Errorf("历史记录不存在")
	}
	return s.historyDAO.Delete(id, userID)
}

// DeleteHistories 批量删除历史记录
func (s *FavoriteService) DeleteHistories(ids []int64, userID int64) error {
	if len(ids) == 0 {
		return fmt.Errorf("请选择要删除的记录")
	}
	return s.historyDAO.DeleteBatch(ids, userID)
}

// ClearHistory 清空用户历史记录
func (s *FavoriteService) ClearHistory(userID int64) error {
	return s.historyDAO.ClearByUser(userID)
}

// GetHistoryCount 获取用户历史记录数量
func (s *FavoriteService) GetHistoryCount(userID int64) (int64, error) {
	return s.historyDAO.CountByUser(userID)
}

// ============ 通用用户数据操作 ============

// UserDataService 用户数据服务
type UserDataService struct {
	userDataDAO *dao.UserDataDAO
}

// NewUserDataService 创建 UserDataService
func NewUserDataService(userDataDAO *dao.UserDataDAO) *UserDataService {
	return &UserDataService{userDataDAO: userDataDAO}
}

// Get 获取单个数据
func (s *UserDataService) Get(userID int64, key string) (string, error) {
	ud, err := s.userDataDAO.Get(userID, key)
	if err != nil {
		return "", fmt.Errorf("数据不存在")
	}
	return ud.Value, nil
}

// List 列出所有数据
func (s *UserDataService) List(userID int64) ([]model.UserData, error) {
	return s.userDataDAO.List(userID)
}

// Set 插入或更新数据
func (s *UserDataService) Set(userID int64, key, value string) error {
	_, err := s.userDataDAO.Upsert(userID, key, value)
	return err
}

// BatchSet 批量设置数据
func (s *UserDataService) BatchSet(userID int64, items map[string]string) error {
	for k, v := range items {
		_, err := s.userDataDAO.Upsert(userID, k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

// Delete 删除数据
func (s *UserDataService) Delete(userID int64, key string) error {
	return s.userDataDAO.Delete(userID, key)
}

// DeleteAll 清空数据
func (s *UserDataService) DeleteAll(userID int64) error {
	return s.userDataDAO.DeleteAll(userID)
}
