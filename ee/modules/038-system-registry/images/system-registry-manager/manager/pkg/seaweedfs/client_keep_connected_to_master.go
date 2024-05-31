package seaweedfs

import (
	"context"
	"sync"
)

type KeepConnectedToMaster struct {
	commandEnv *commandEnv
	ctx        context.Context
	ctxCancel  context.CancelFunc
	once       *sync.Once
}

func NewKeepConnectedToMaster(commandEnv *commandEnv) *KeepConnectedToMaster {
	ctx, cancel := context.WithCancel(context.Background())
	return &KeepConnectedToMaster{
		commandEnv: commandEnv,
		ctx:        ctx,
		ctxCancel:  cancel,
		once:       &sync.Once{},
	}
}

func (k *KeepConnectedToMaster) Start() {
	k.once.Do(func() {
		go k.commandEnv.MasterClient.KeepConnectedToMaster(k.ctx)
	})
}

func (k *KeepConnectedToMaster) Stop() {
	k.ctxCancel()
}
