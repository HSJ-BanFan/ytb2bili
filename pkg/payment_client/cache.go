package payment_client

import (
	"sync"
	"time"
)

// ProductCache 商品缓存服务
type ProductCache struct {
	client            *Client
	allowedProductIDs []string      // 允许的商品 ID 白名单
	products          []Product     // 缓存的商品列表
	lastUpdate        time.Time     // 上次更新时间
	cacheDuration     time.Duration // 缓存有效期
	mu                sync.RWMutex
}

// NewProductCache 创建商品缓存
// allowedIDs: 只允许获取的商品 ID 列表
// cacheDuration: 缓存有效期（建议 5-10 分钟）
func NewProductCache(client *Client, allowedIDs []string, cacheDuration time.Duration) *ProductCache {
	return &ProductCache{
		client:            client,
		allowedProductIDs: allowedIDs,
		cacheDuration:     cacheDuration,
	}
}

// GetAllowedProducts 获取允许的商品列表（带缓存）
func (pc *ProductCache) GetAllowedProducts() ([]Product, error) {
	pc.mu.RLock()
	if time.Since(pc.lastUpdate) < pc.cacheDuration && len(pc.products) > 0 {
		products := pc.products
		pc.mu.RUnlock()
		return products, nil
	}
	pc.mu.RUnlock()

	// 缓存过期，重新获取
	pc.mu.Lock()
	defer pc.mu.Unlock()

	// 双重检查
	if time.Since(pc.lastUpdate) < pc.cacheDuration && len(pc.products) > 0 {
		return pc.products, nil
	}

	products, err := pc.client.GetProductsByIDs(pc.allowedProductIDs)
	if err != nil {
		return nil, err
	}

	pc.products = products
	pc.lastUpdate = time.Now()
	return products, nil
}

// RefreshCache 强制刷新缓存
func (pc *ProductCache) RefreshCache() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	products, err := pc.client.GetProductsByIDs(pc.allowedProductIDs)
	if err != nil {
		return err
	}

	pc.products = products
	pc.lastUpdate = time.Now()
	return nil
}

// GetProduct 获取单个商品（从缓存）
func (pc *ProductCache) GetProduct(productID string) (*Product, error) {
	products, err := pc.GetAllowedProducts()
	if err != nil {
		return nil, err
	}

	for _, p := range products {
		if p.ProductID == productID {
			return &p, nil
		}
	}

	return nil, NewPaymentError("PRODUCT_NOT_FOUND", "商品不存在或不在白名单中", nil)
}

// IsProductAllowed 检查商品是否在白名单中
func (pc *ProductCache) IsProductAllowed(productID string) bool {
	for _, id := range pc.allowedProductIDs {
		if id == productID {
			return true
		}
	}
	return false
}

// UpdateAllowedIDs 更新允许的商品 ID 列表
func (pc *ProductCache) UpdateAllowedIDs(ids []string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.allowedProductIDs = ids
	pc.products = nil // 清空缓存
	pc.lastUpdate = time.Time{}
}
