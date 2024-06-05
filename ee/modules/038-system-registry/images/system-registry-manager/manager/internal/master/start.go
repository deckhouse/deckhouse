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
	"system-registry-manager/internal/master/handler"
	kube_actions "system-registry-manager/pkg/kubernetes/actions"
	pkg_logs "system-registry-manager/pkg/logs"
)

const (
	processName = "leader"
)

type Master struct {
	commonHandler *handler.CommonHandler
	commonCfg     *common.RuntimeConfig
	rootCtx       context.Context
	log           *log.Entry
}

func New(rootCtx context.Context, rCfg *common.RuntimeConfig) (*Master, error) {
	rootCtx = pkg_logs.SetLoggerToContext(rootCtx, processName)
	log := pkg_logs.GetLoggerFromContext(rootCtx)

	master := &Master{
		commonCfg: rCfg,
		rootCtx:   rootCtx,
		log:       log,
	}

	var err error
	master.commonHandler, err = handler.NewCommonHandler(rootCtx)
	return master, err
}

func (m *Master) Start() {
	defer m.log.Info("Master shutdown")

	recorder := kube_actions.NewLeaderElectionRecorder(m.log)
	identity, err := kube_actions.NewIdentityForLeaderElection()
	if err != nil {
		defer m.commonCfg.StopManager()
		m.log.Errorf("Failed to start master election: %v", err)
		return
	}

	leaderCallbacks := leaderelection.LeaderCallbacks{
		OnStartedLeading: func(ctx context.Context) {
			m.commonCfg.IsMasterUpdate(true)
			m.commonCfg.CurrentMasterNameUpdate(identity)
			defer m.commonCfg.IsMasterUpdate(false)
			startMasterWorkflow(ctx, m)
		},
		OnStoppedLeading: func() {
			m.commonCfg.IsMasterUpdate(false)
			m.commonCfg.StopManager()
		},
		OnNewLeader: func(identity string) {
			m.commonCfg.CurrentMasterNameUpdate(identity)
		},
	}

	err = kube_actions.StartLeaderElection(m.rootCtx, recorder, leaderCallbacks, identity)
	if err != nil {
		defer m.commonCfg.StopManager()
		m.log.Errorf("Failed to start master election: %v", err)
	}
}
