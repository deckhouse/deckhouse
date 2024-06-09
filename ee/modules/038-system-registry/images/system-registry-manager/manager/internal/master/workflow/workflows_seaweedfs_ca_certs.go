/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package workflow

import (
	"context"
	"fmt"
	pkg_logs "system-registry-manager/pkg/logs"

	"github.com/cloudflare/cfssl/log"
	"github.com/sirupsen/logrus"
)

type SeaweedfsCertsWorkflow struct {
	log               *logrus.Entry
	ctx               context.Context
	ExpectedNodeCount int
	NodeManagers      []NodeManager
}

func NewSeaweedfsCertsWorkflow(ctx context.Context, nodeManagers []NodeManager, expectedNodeCount int) *SeaweedfsCertsWorkflow {
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

	w.log.Info("Selecting nodes that exist and need certificate updates")
	existAndNeedUpdateCert, _, err := SelectBy(w.NodeManagers, CmpIsExist, CmpIsNeedUpdateCerts)
	if err != nil {
		return err
	}

	if len(existAndNeedUpdateCert) <= 0 {
		log.Info("Nothing to do")
	}

	w.log.Infof("Found %s nodes that need certificate updates", GetNodeNames(existAndNeedUpdateCert))
	updateRequest := SeaweedfsUpdateNodeRequest{
		Certs: struct {
			UpdateOrCreate bool "json:\"updateOrCreate\""
		}{true},
		Manifests: struct {
			UpdateOrCreate bool "json:\"updateOrCreate\""
		}{false},
		StaticPods: struct {
			MasterPeers    []string "json:\"masterPeers\""
			UpdateOrCreate bool     "json:\"updateOrCreate\""
		}{
			MasterPeers:    []string{},
			UpdateOrCreate: false,
		},
	}

	for _, node := range existAndNeedUpdateCert {
		w.log.Infof("Updating CA certificates for node: %s", node.GetNodeName())
		if err := node.UpdateNodeManifests(&updateRequest); err != nil {
			return err
		}
	}

	{
		w.log.Infof("Waiting nodes and leader election for: %s", GetNodeNames(existAndNeedUpdateCert))

		haveLeader := false
		var cpmFunc CpmFuncNodeClusterStatus = func(status *SeaweedfsNodeClusterStatus) bool {
			if status.IsLeader {
				haveLeader = true
			}
			return haveLeader
		}

		wait, err := WaitBy(w.log, existAndNeedUpdateCert, CmpIsRunning, cpmFunc)
		if err != nil {
			return err
		}
		if !wait {
			return fmt.Errorf("error waitig nodes: %s", GetNodeNames(existAndNeedUpdateCert))
		}
	}
	w.log.Info("SeaweedfsCertsWorkflow completed successfully")
	return nil
}
