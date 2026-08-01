package middleware

import (
	"fmt"
	"sync"
	"time"

	"github.com/sony/gobreaker"
)

// CircuitBreakerManager 熔断器管理器
type CircuitBreakerManager struct {
	breakers map[string]*gobreaker.CircuitBreaker
	mu       sync.RWMutex
}

// NewCircuitBreakerManager 创建熔断器管理器
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*gobreaker.CircuitBreaker),
	}
}

// GetOrCreate 获取或创建熔断器
func (cbm *CircuitBreakerManager) GetOrCreate(name string) *gobreaker.CircuitBreaker {
	cbm.mu.RLock()
	cb, exists := cbm.breakers[name]
	cbm.mu.RUnlock()

	if exists {
		return cb
	}

	cbm.mu.Lock()
	defer cbm.mu.Unlock()

	// 双重检查
	if cb, exists = cbm.breakers[name]; exists {
		return cb
	}

	settings := gobreaker.Settings{
		Name:        name,
		MaxRequests: 3,                // 半开状态最大请求数
		Interval:    60 * time.Second, // 统计周期
		Timeout:     30 * time.Second, // 熔断超时（进入半开状态）
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// 请求数 >= 10 且 失败率 >= 50% 时熔断
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 10 && failureRatio >= 0.5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			fmt.Printf("[CircuitBreaker] %s: %s -> %s\n", name, from, to)
		},
	}

	cb = gobreaker.NewCircuitBreaker(settings)
	cbm.breakers[name] = cb
	return cb
}

// Execute 通过熔断器执行函数
func (cbm *CircuitBreakerManager) Execute(name string, fn func() (interface{}, error)) (interface{}, error) {
	cb := cbm.GetOrCreate(name)
	return cb.Execute(fn)
}

// GetStatus 获取所有熔断器状态
func (cbm *CircuitBreakerManager) GetStatus() map[string]interface{} {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	status := make(map[string]interface{})
	for name, cb := range cbm.breakers {
		status[name] = map[string]interface{}{
			"state":   cb.State().String(),
			"counts":  cb.Counts(),
			"name":    name,
		}
	}
	return status
}