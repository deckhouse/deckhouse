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
	NodeManagers      []NodeManager
}

func NewSeaweedfsCaCertsWorkflow(ctx context.Context, nodeManagers []NodeManager, expectedNodeCount int) *SeaweedfsCertsWorkflow {
	log := pkg_logs.GetLoggerFromContext(ctx)
	return &SeaweedfsCertsWorkflow{
		log:               log,
		ctx:               ctx,
		ExpectedNodeCount: expectedNodeCount,
		NodeManagers:      nodeManagers,
	}
}

func (w *SeaweedfsCertsWorkflow) Start() error {
	w.log.Info("Starting SeaweedfsCertsWorkflow")

	w.log.Info("Selecting nodes that exist and need CA certificates update")
	existAndNeedUpdateCA, _, err := SelectByRunningStatus(w.NodeManagers, CmpSelectIsExist, CmpSelectIsNeedUpdateCaCerts)
	if err != nil {
		return err
	}

	w.log.Infof("Found %s nodes that need CA certificates update", GetNodeNames(existAndNeedUpdateCA))

	updateRequest := SeaweedfsUpdateNodeRequest{
		UpdateCert:      true,
		UpdateCaCerts:   true,
		UpdateManifests: false,
	}

	for _, node := range existAndNeedUpdateCA {
		w.log.Infof("Updating CA certificates for node: %s", node.GetNodeName())
		if err := node.UpdateNodeManifests(&updateRequest); err != nil {
			return err
		}
	}

	w.log.Info("SeaweedfsCertsWorkflow completed successfully")
	return nil
}
