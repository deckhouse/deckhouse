package cluster

import (
	"fmt"
	"update-observer/common"

	"golang.org/x/mod/semver"
	corev1 "k8s.io/api/core/v1"
)

type NodesState struct {
	DesiredCount   int `json:"desiredCount" yaml:"desiredCount"`
	UpToDateCount  int `json:"upToDateCount" yaml:"upToDateCount"`
	CurrentVersion string
}

func GetNodesState(nodes []corev1.Node, desiredVersion string) (*NodesState, error) {
	res := &NodesState{}

	var err error
	for _, node := range nodes {
		res.DesiredCount++
		v := node.Status.NodeInfo.KubeletVersion
		v, err = common.NormalizeVersion(v)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize version of node '%s': %w", node.Name, err)
		}

		if v == desiredVersion {
			res.UpToDateCount++
		}

		if res.CurrentVersion == "" || semver.Compare(v, res.CurrentVersion) == 1 {
			res.CurrentVersion = v
		}
	}

	return res, nil
}
