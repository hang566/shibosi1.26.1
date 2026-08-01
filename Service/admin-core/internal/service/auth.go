package service

import (
	"admin-core/internal/cache"
	"admin-core/internal/dao"
	"admin-core/internal/middleware"
	"admin-core/internal/model"
	"context"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// AuthService 认证服务
type AuthService struct {
	userDAO    *dao.UserDAO
	roleDAO    *dao.RoleDAO
	auditDAO   *dao.AuditDAO
	jwtManager *middleware.JWTManager
	redisCache *cache.RedisCache
	bcryptCost int
}

// NewAuthService 创建AuthService
func NewAuthService(
	userDAO *dao.UserDAO,
	roleDAO *dao.RoleDAO,
	auditDAO *dao.AuditDAO,
	jwtManager *middleware.JWTManager,
	redisCache *cache.RedisCache,
	bcryptCost int,
) *AuthService {
	return &AuthService{
		userDAO:    userDAO,
		roleDAO:    roleDAO,
		auditDAO:   auditDAO,
		jwtManager: jwtManager,
		redisCache: redisCache,
		bcryptCost: bcryptCost,
	}
}

// HashPassword 使用bcrypt哈希密码
func (s *AuthService) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return "", fmt.Errorf("密码哈希失败: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword 验证密码
func (s *AuthService) CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// Register 用户注册
func (s *AuthService) Register(ctx context.Context, req *model.LoginRequest) (*model.User, error) {
	// 检查用户名是否已存在
	existing, _ := s.userDAO.GetByUsername(req.Username)
	if existing != nil {
		return nil, fmt.Errorf("用户名已存在")
	}

	// 哈希密码
	hashedPassword, err := s.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Username: req.Username,
		Password: hashedPassword,
		Nickname: req.Username,
		Role:     "user",
		Status:   1,
	}

	if err := s.userDAO.Create(user); err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	return user, nil
}

// Login 用户登录
func (s *AuthService) Login(ctx context.Context, req *model.LoginRequest) (*model.LoginResponse, error) {
	// 查询用户
	user, err := s.userDAO.GetByUsername(req.Username)
	if err != nil {
		return nil, fmt.Errorf("用户名或密码错误")
	}

	// 检查用户状态
	if user.Status == 0 {
		return nil, fmt.Errorf("账户已被禁用")
	}

	// 验证密码
	if !s.CheckPassword(req.Password, user.Password) {
		return nil, fmt.Errorf("用户名或密码错误")
	}

	// 生成Token
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, fmt.Errorf("生成令牌失败: %w", err)
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, fmt.Errorf("生成刷新令牌失败: %w", err)
	}

	// 缓存Token到Redis
	if s.redisCache != nil {
		ctx := context.Background()
		accessKey := cache.KeyUserToken + fmt.Sprintf("access:%d", user.ID)
		refreshKey := cache.KeyUserToken + fmt.Sprintf("refresh:%d", user.ID)

		_ = s.redisCache.Set(ctx, accessKey, accessToken, 30*time.Minute)
		_ = s.redisCache.Set(ctx, refreshKey, refreshToken, 7*24*time.Hour)
	}

	// 更新最后登录时间
	_ = s.userDAO.UpdateLastLogin(user.ID)

	// 清除密码字段
	user.Password = ""

	return &model.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(30 * 60), // 30分钟
		User:         user,
	}, nil
}

// RefreshToken 刷新令牌
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*model.LoginResponse, error) {
	claims, err := s.jwtManager.ParseToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("无效的刷新令牌")
	}

	// 从Subject中提取userID
	userID := claims.UserID

	// 将旧RefreshToken加入黑名单
	if exp, err := claims.GetExpirationTime(); err == nil {
		s.jwtManager.AddToBlacklist(refreshToken, exp.Time)
	}

	// 查询用户
	user, err := s.userDAO.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("用户不存在")
	}

	if user.Status == 0 {
		return nil, fmt.Errorf("账户已被禁用")
	}

	// 生成新Token
	accessToken, _ := s.jwtManager.GenerateAccessToken(user.ID, user.Username, user.Role)
	newRefreshToken, _ := s.jwtManager.GenerateRefreshToken(user.ID, user.Username, user.Role)

	// 更新缓存
	if s.redisCache != nil {
		accessKey := cache.KeyUserToken + fmt.Sprintf("access:%d", user.ID)
		refreshKey := cache.KeyUserToken + fmt.Sprintf("refresh:%d", user.ID)
		_ = s.redisCache.Set(ctx, accessKey, accessToken, 30*time.Minute)
		_ = s.redisCache.Set(ctx, refreshKey, newRefreshToken, 7*24*time.Hour)
	}

	user.Password = ""

	return &model.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int64(30 * 60),
		User:         user,
	}, nil
}

// Logout 登出
func (s *AuthService) Logout(ctx context.Context, userID int64, token string) error {
	// 将当前Token加入黑名单
	claims, err := s.jwtManager.ParseToken(token)
	if err == nil {
		if exp, err := claims.GetExpirationTime(); err == nil {
			s.jwtManager.AddToBlacklist(token, exp.Time)
		}
	}

	// 清除Redis中的Token缓存
	if s.redisCache != nil {
		accessKey := cache.KeyUserToken + fmt.Sprintf("access:%d", userID)
		refreshKey := cache.KeyUserToken + fmt.Sprintf("refresh:%d", userID)
		_ = s.redisCache.Delete(ctx, accessKey, refreshKey)
	}

	return nil
}

// ResetPassword 重置密码（管理员操作）
func (s *AuthService) ResetPassword(userID int64, newPassword string) error {
	hashedPassword, err := s.HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.userDAO.UpdatePassword(userID, hashedPassword)
}

// ChangePassword 修改密码（用户自主操作）
func (s *AuthService) ChangePassword(userID int64, oldPassword, newPassword string) error {
	user, err := s.userDAO.GetByID(userID)
	if err != nil {
		return fmt.Errorf("用户不存在")
	}

	if !s.CheckPassword(oldPassword, user.Password) {
		return fmt.Errorf("原密码错误")
	}

	hashedPassword, err := s.HashPassword(newPassword)
	if err != nil {
		return err
	}

	return s.userDAO.UpdatePassword(userID, hashedPassword)
}