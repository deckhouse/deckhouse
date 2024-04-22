/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package workflow

import (
	"context"
	"github.com/sirupsen/logrus"
	pkg_logs "system-registry-manager/pkg/logs"
)

type SeaweedfsCertsWorkflow struct {
	log               *logrus.Entry
	ctx               context.Context
	ExpectedNodeCount int
	NodeManagers      []RegistryNodeManager
}

func NewSeaweedfsCertsWorkflow(ctx context.Context, nodeManagers []RegistryNodeManager, expectedNodeCount int) *SeaweedfsCertsWorkflow {
	log := pkg_logs.GetLoggerFromContext(ctx)
	return &SeaweedfsCertsWorkflow{
		log:               log,
		ctx:               ctx,
		ExpectedNodeCount: expectedNodeCount,
		NodeManagers:      nodeManagers,
	}
}

func (w *SeaweedfsCertsWorkflow) Start() error {
	w.log.Info("▶️ CertsWorkflow :: Start")

	w.log.Info("Start :: Selecting nodes that exist and need certificate updates")
	existAndNeedUpdateCert, _, err := SelectBy(w.NodeManagers, CmpIsExist, CmpIsNeedUpdateCerts)
	if err != nil {
		return err
	}

	if len(existAndNeedUpdateCert) <= 0 {
		w.log.Info("Start :: Nothing to do")
		return nil
	}

	w.log.Infof("Start :: Found %s nodes that need certificate updates", GetNodeNames(existAndNeedUpdateCert))
	updateRequest := SeaweedfsUpdateNodeRequest{
		Certs: struct {
			UpdateOrCreate bool "json:\"updateOrCreate\""
		}{true},
		Manifests: struct {
			UpdateOrCreate bool "json:\"updateOrCreate\""
		}{false},
		StaticPods: struct {
			MasterPeers     []string "json:\"masterPeers\""
			IsRaftBootstrap bool     "json:\"isRaftBootstrap\""
			UpdateOrCreate  bool     "json:\"updateOrCreate\""
		}{
			MasterPeers:     []string{},
			IsRaftBootstrap: false,
			UpdateOrCreate:  false,
		},
	}

	for _, node := range existAndNeedUpdateCert {
		w.log.Infof("Start :: Updating CA certificates for node: %s", node.GetNodeName())
		if err := node.UpdateNodeManifests(&updateRequest); err != nil {
			return err
		}
	}
	return nil
}
