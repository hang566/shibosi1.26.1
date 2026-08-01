package config

import (
	"fmt"
	"sync"
	"time"
)

// ServiceRegistry 动态服务注册表 - 支持自注册和心跳
type ServiceRegistry struct {
	mu       sync.RWMutex
	services map[string]*RegisteredService
}

// RegisteredService 已注册的服务信息
type RegisteredService struct {
	Config        ServiceConfig `json:"config"`
	RegisteredAt  time.Time     `json:"registered_at"`
	LastHeartbeat time.Time     `json:"last_heartbeat"`
	ExpiresAt     time.Time     `json:"expires_at"`
}

// NewServiceRegistry 创建服务注册表
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[string]*RegisteredService),
	}
}

// Register 注册服务
func (r *ServiceRegistry) Register(name string, cfg ServiceConfig, heartbeatSeconds int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	expires := now.Add(time.Duration(heartbeatSeconds) * time.Second * 2)

	r.services[name] = &RegisteredService{
		Config:        cfg,
		RegisteredAt:  now,
		LastHeartbeat: now,
		ExpiresAt:     expires,
	}
}

// Heartbeat 服务心跳
func (r *ServiceRegistry) Heartbeat(name string, heartbeatSeconds int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	svc, exists := r.services[name]
	if !exists {
		return fmt.Errorf("服务 %s 未注册", name)
	}

	now := time.Now()
	svc.LastHeartbeat = now
	svc.ExpiresAt = now.Add(time.Duration(heartbeatSeconds) * time.Second * 2)
	return nil
}

// Unregister 注销服务
func (r *ServiceRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.services, name)
}

// GetService 获取服务
func (r *ServiceRegistry) GetService(name string) (*RegisteredService, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	svc, exists := r.services[name]
	return svc, exists
}

// GetAllServices 获取所有服务（返回深拷贝，防止外部修改）
func (r *ServiceRegistry) GetAllServices() map[string]*RegisteredService {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]*RegisteredService)
	for k, v := range r.services {
		clone := &RegisteredService{
			Config:        v.Config,
			RegisteredAt:  v.RegisteredAt,
			LastHeartbeat: v.LastHeartbeat,
			ExpiresAt:     v.ExpiresAt,
		}
		result[k] = clone
	}
	return result
}

// UpdateServiceStatus 更新服务状态和最后检查时间（使用写锁）
func (r *ServiceRegistry) UpdateServiceStatus(name, status, lastCheck string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if svc, exists := r.services[name]; exists {
		svc.Config.Status = status
		if lastCheck != "" {
			svc.Config.LastCheck = lastCheck
		}
	}
}

// RemoveExpired 移除过期服务
func (r *ServiceRegistry) RemoveExpired() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	now := time.Now()
	for name, svc := range r.services {
		if now.After(svc.ExpiresAt) {
			delete(r.services, name)
			count++
		}
	}
	return count
}

// SyncWithConfig 将静态配置同步到注册表
func (r *ServiceRegistry) SyncWithConfig(cfgServices map[string]ServiceConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for name, svcCfg := range cfgServices {
		if _, exists := r.services[name]; !exists {
			// 静态配置的服务默认有效期较长
			r.services[name] = &RegisteredService{
				Config:        svcCfg,
				RegisteredAt:  now,
				LastHeartbeat: now,
				ExpiresAt:     now.Add(365 * 24 * time.Hour), // 一年有效期
			}
		} else {
			// 更新配置（保留动态状态）
			r.services[name].Config = svcCfg
		}
	}
}
