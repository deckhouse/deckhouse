/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cluster

import (
	"fmt"
	"update-observer/common"

	"golang.org/x/mod/semver"
	corev1 "k8s.io/api/core/v1"
)

type NodesState struct {
	DesiredCount   int
	UpToDateCount  int
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
