/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package common

import (
	"context"
	"sync"
)

type RuntimeConfig struct {
	sync.RWMutex
	rootContextCancel *context.CancelFunc
	isMaster          bool
}

func NewRuntimeConfig(rootContextCancel *context.CancelFunc) *RuntimeConfig {
	return &RuntimeConfig{
		rootContextCancel: rootContextCancel,
		isMaster:          false,
	}
}

func (rCfg *RuntimeConfig) StopManager() {
	rCfg.Lock()
	defer rCfg.Unlock()
	(*rCfg.rootContextCancel)()
}

func (rCfg *RuntimeConfig) IsMaster() bool {
	rCfg.RLock()
	defer rCfg.RUnlock()
	return rCfg.isMaster
}

func (rCfg *RuntimeConfig) IsMasterUpdate(isMaster bool) {
	rCfg.Lock()
	defer rCfg.Unlock()
	rCfg.isMaster = isMaster
}
