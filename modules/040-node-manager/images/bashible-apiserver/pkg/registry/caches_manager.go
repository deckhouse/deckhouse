package registry

import (
	"sync"

	"k8s.io/client-go/tools/cache"

	"bashible-apiserver/pkg/template"

)

type CachesManager interface {
	template.UpdateHandler
	GetCache() cache.ThreadSafeStore
}

func NewCachesManager() CachesManager {
	return &threadSafeCachesManager{
		caches: make([]cache.ThreadSafeStore, 0),
	}
}

type threadSafeCachesManager struct {
	// lock need because we pass manager in none thread safe place (Template Context)
	// and OnUpdate may calling in one moment with getCache
	lock   sync.Mutex
	caches []cache.ThreadSafeStore
}

// getCache return storage for cache and save it in itself for handle OnUpdate
func (m *threadSafeCachesManager) GetCache() cache.ThreadSafeStore {
	m.lock.Lock()
	defer m.lock.Unlock()

	cacheStore := cache.NewThreadSafeStore(cache.Indexers{}, cache.Indices{})
	m.caches = append(m.caches, cacheStore)

	return cacheStore
}

func (m *threadSafeCachesManager) clearCaches() {
	m.lock.Lock()
	defer m.lock.Unlock()

	for _, c := range m.caches {
		// clear cache
		s := make(map[string]interface{})
		c.Replace(s, "")
	}
}

// OnUpdate handle update event and clear all caches
func (m *threadSafeCachesManager) OnUpdate() {
	m.clearCaches()
}
