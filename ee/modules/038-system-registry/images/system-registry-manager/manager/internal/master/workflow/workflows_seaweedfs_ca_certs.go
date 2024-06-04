/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package workflow

type SeaweedfsCertsWorkflow struct {
	ExpectedNodeCount int
	NodeManagers      []*SeaweedfsNodeManager
}

func NewSeaweedfsCaCertsWorkflow(nodeManagers []*SeaweedfsNodeManager, expectedNodeCount int) *SeaweedfsScaleWorkflow {
	return &SeaweedfsScaleWorkflow{
		ExpectedNodeCount: expectedNodeCount,
		NodeManagers:      nodeManagers,
	}
}

func (w *SeaweedfsCertsWorkflow) Start() error {
	existAndNeedUpdateCA, _, err := SelectByRunningStatus(w.NodeManagers, CmpSelectIsExist, CmpSelectIsNeedUpdateCaCerts)
	if err != nil {
		return err
	}

	updateRequest := SeaweedfsUpdateNodeRequest{
		UpdateCert:      true,
		UpdateCaCerts:   true,
		UpdateManifests: false,
	}

	for _, node := range existAndNeedUpdateCA {
		if err := (*node).UpdateNodeManifests(&updateRequest); err != nil {
			return err
		}
	}
	return nil
}
