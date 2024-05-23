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
	sync.Mutex
	rootContextCancel *context.CancelFunc
	isMaster          bool
	masterName        string
	currentMasterName string
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
	rCfg.Lock()
	defer rCfg.Unlock()
	return rCfg.isMaster
}

func (rCfg *RuntimeConfig) IsMasterUpdate(isMaster bool) {
	rCfg.Lock()
	defer rCfg.Unlock()
	rCfg.isMaster = isMaster
}

func (rCfg *RuntimeConfig) MasterName() string {
	rCfg.Lock()
	defer rCfg.Unlock()
	return rCfg.masterName
}

func (rCfg *RuntimeConfig) CurrentMasterName() string {
	rCfg.Lock()
	defer rCfg.Unlock()
	return rCfg.currentMasterName
}

func (rCfg *RuntimeConfig) CurrentMasterNameUpdate(currentMasterName string) {
	rCfg.Lock()
	defer rCfg.Unlock()
	rCfg.currentMasterName = currentMasterName
}
