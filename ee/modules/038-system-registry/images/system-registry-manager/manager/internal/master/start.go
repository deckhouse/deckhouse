/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package master

import (
	"context"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/leaderelection"
	common "system-registry-manager/internal/common"
	kube_actions "system-registry-manager/pkg/kubernetes/actions"
	pkg_logs "system-registry-manager/pkg/logs"
)

const (
	processName = "leader"
)

type Master struct {
	commonCfg *common.RuntimeConfig
	rootCtx   context.Context
	log       *log.Entry
}

func New(rootCtx context.Context, rCfg *common.RuntimeConfig) *Master {
	rootCtx = pkg_logs.SetLoggerToContext(rootCtx, processName)
	log := pkg_logs.GetLoggerFromContext(rootCtx)

	return &Master{
		commonCfg: rCfg,
		rootCtx:   rootCtx,
		log:       log,
	}
}

func (m *Master) Start() {
	recorder := kube_actions.NewLeaderElectionRecorder(m.log)

	leaderCallbacks := leaderelection.LeaderCallbacks{
		OnStartedLeading: func(ctx context.Context) {
			func(ctx context.Context) {
				m.log.Info("Master controller loop...")
				select {}
			}(m.rootCtx)
		},
		OnStoppedLeading: func() {
			m.commonCfg.StopManager()
		},
		OnNewLeader: func(identity string) {

		},
	}

	err := kube_actions.StartLeaderElection(m.rootCtx, recorder, leaderCallbacks)
	if err != nil {
		defer m.commonCfg.StopManager()
		log.Errorf("Failed to start master election: %v", err)
	}
	m.log.Info("Master shutdown")
}
