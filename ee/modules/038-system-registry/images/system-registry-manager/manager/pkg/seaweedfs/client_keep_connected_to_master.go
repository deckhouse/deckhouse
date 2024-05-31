package seaweedfs

import (
	"context"
	"sync"
)

type KeepConnectedToMaster struct {
	cm        *commandEnv
	ctx       context.Context
	ctxCancel context.CancelFunc
	once      *sync.Once
}

func NewKeepConnectedToMaster(cm *commandEnv) *KeepConnectedToMaster {
	ctx, cancel := context.WithCancel(context.Background())
	return &KeepConnectedToMaster{
		cm:        cm,
		ctx:       ctx,
		ctxCancel: cancel,
		once:      &sync.Once{},
	}
}

func (k *KeepConnectedToMaster) Start() {
	k.once.Do(func() {
		go k.cm.MasterClient.KeepConnectedToMaster(k.ctx)
	})
}

func (k *KeepConnectedToMaster) Stop() {
	k.ctxCancel()
}
