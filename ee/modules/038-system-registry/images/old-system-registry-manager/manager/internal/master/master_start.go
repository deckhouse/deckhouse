package master

import (
	"context"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/leaderelection"
	kubeactions "system-registry-manager/pkg/kubernetes/actions"
	pkglogs "system-registry-manager/pkg/logs"
)

const processName = "leader"

type Master struct {
	rootCtx    context.Context
	cancelFunc context.CancelFunc
	log        *logrus.Entry
}

func New(rootCtx context.Context, cancel context.CancelFunc) *Master {
	rootCtx = pkglogs.SetLoggerToContext(rootCtx, processName)
	log := pkglogs.GetLoggerFromContext(rootCtx)

	return &Master{
		rootCtx:    rootCtx,
		cancelFunc: cancel,
		log:        log,
	}
}

func (m *Master) Start() {
	defer m.log.Info("Master shutdown")

	recorder := kubeactions.NewLeaderElectionRecorder(m.log)
	identity, err := kubeactions.NewIdentityForLeaderElection()
	if err != nil {
		m.logAndStopManager("Failed to start master election", err)
		return
	}

	leaderCallbacks := leaderelection.LeaderCallbacks{
		OnStartedLeading: func(ctx context.Context) {
			startMasterWorkflow(ctx, m)
		},
		OnStoppedLeading: func() {
			m.logAndStopManager("Lost leadership, stopping manager", err) // TODO don't need to stop all application
		},
	}

	err = kubeactions.StartLeaderElection(m.rootCtx, recorder, leaderCallbacks, identity)
	if err != nil {
		m.logAndStopManager("Failed to start master election", err)
	}
}

func (m *Master) logAndStopManager(message string, err error) {
	defer m.cancelFunc()
	m.log.Errorf("%s: %v", message, err)
}
