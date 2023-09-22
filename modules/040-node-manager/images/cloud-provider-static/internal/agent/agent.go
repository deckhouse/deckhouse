package agent

import (
	"cloud-provider-static/internal/providerid"
	"sync"
)

type Agent struct {
	locksMutex sync.Mutex
	locks      map[providerid.ProviderID]struct{}
}

func NewAgent() *Agent {
	return &Agent{
		locks: make(map[providerid.ProviderID]struct{}),
	}
}

func (a *Agent) lock(providerID providerid.ProviderID, fn func()) {
	a.locksMutex.Lock()
	defer a.locksMutex.Unlock()

	_, ok := a.locks[providerID]
	if ok {
		return
	}

	a.locks[providerID] = struct{}{}

	go func() {
		defer func() {
			a.locksMutex.Lock()
			defer a.locksMutex.Unlock()

			delete(a.locks, providerID)
		}()

		fn()
	}()
}
