// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package converge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// DestructiveChangeID returns sha256 hash of destructive changes
func DestructiveChangeID(s *Statistics) (string, error) {
	h := sha256.New()

	b, err := destructiveChangeID(s)
	if err != nil {
		return "", err
	}

	h.Write(b)
	return hex.EncodeToString(h.Sum(nil)), nil
}

func destructiveChangeID(s *Statistics) ([]byte, error) {
	if s == nil {
		return []byte{}, nil
	}

	m := make(map[string]any)

	addNodeChanges(s.Node, m)

	addClusterChanges(s.Cluster, m)

	if len(m) == 0 {
		return []byte{}, nil
	}

	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal destructive change id %w", err)
	}

	return b, nil
}

func addNodeChanges(nodes []NodeCheckResult, m map[string]any) {
	// nodes[].destructive_changes.resources_deleted (current_value["type"], current_value["name"])
	// nodes[].destructive_changes.resources_recreated (next_value)
	for _, node := range nodes {
		if node.DestructiveChanges == nil {
			continue
		}

		for i, resourceDeleted := range node.DestructiveChanges.ResourcesDeleted {
			if resourceDeleted.CurrentValue == nil {
				continue
			}

			switch currentValue := resourceDeleted.CurrentValue.(type) {
			case map[string]any:
				m[fmt.Sprintf("node:%s:resource_deleted:%d:current", node.Name, i)] = map[string]any{
					"type": currentValue["type"],
					"name": currentValue["name"],
				}
			default:
				m[fmt.Sprintf("node:%s:resource_deleted:%d:current", node.Name, i)] = currentValue
			}
		}

		for i, resourcedRecreated := range node.DestructiveChanges.ResourcesRecreated {
			if resourcedRecreated.NextValue == nil {
				continue
			}
			m[fmt.Sprintf("node:%s:resource_recreated:%d:next", node.Name, i)] = resourcedRecreated.NextValue
		}
	}

	return
}

func addClusterChanges(cluster ClusterCheckResult, m map[string]any) {
	// cluster.destructive_changes.output_broken_reason
	// cluster.destructive_changes.output_zones_changed (next_value)
	// cluster.destructive_changes.resources_deleted (current_value["type"], current_value["name"])
	// cluster.destructive_changes.resources_recreated (next_value)
	if cluster.DestructiveChanges == nil {
		return
	}

	if cluster.DestructiveChanges.OutputBrokenReason != "" {
		m["cluster:output_broken_reason"] = cluster.DestructiveChanges.OutputBrokenReason
	}

	if cluster.DestructiveChanges.OutputZonesChanged.NextValue != nil {
		m["cluster:output_zones_changed:next"] = cluster.DestructiveChanges.OutputZonesChanged.NextValue
	}

	for i, resourceDeleted := range cluster.DestructiveChanges.ResourcesDeleted {
		if resourceDeleted.CurrentValue == nil {
			continue
		}

		switch currentValue := resourceDeleted.CurrentValue.(type) {
		case map[string]any:
			m[fmt.Sprintf("cluster:resource_deleted:%d:current", i)] = map[string]any{
				"type": currentValue["type"],
				"name": currentValue["name"],
			}
		default:
			m[fmt.Sprintf("node:resource_deleted:%d:current", i)] = currentValue
		}
	}

	for i, resourcedRecreated := range cluster.DestructiveChanges.ResourcesRecreated {
		if resourcedRecreated.NextValue == nil {
			continue
		}
		m[fmt.Sprintf("cluster:resource_recreated:%d:next", i)] = resourcedRecreated.NextValue
	}
}
