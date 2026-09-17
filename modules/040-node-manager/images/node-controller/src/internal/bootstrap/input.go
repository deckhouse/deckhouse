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

package bootstrap

import "fmt"

// Input is everything the bashible bootstrap templates read. It mirrors the
// context helm builds in templates/node-group/_bootstrap.tpl:1-40 — templates
// resolve these keys by name, so a renamed key changes the render silently.
type Input struct {
	// NodeGroup is the resolved NodeGroup as a map: text/template resolves
	// lowercase field names on maps only.
	NodeGroup              map[string]any
	APIServerEndpoints     []string
	ClusterMasterEndpoints []map[string]any
	ClusterUUID            string
	Images                 map[string]any
	PackagesProxy          map[string]any
	MingetB64              string
	Provider               string
	KubernetesCA           string
	BootstrapToken         string
	SSHPublicKey           string
	Files                  *Files
}

func (in Input) templateContext() map[string]any {
	return map[string]any{
		"nodeGroup":                          in.NodeGroup,
		"apiserverEndpoints":                 in.APIServerEndpoints,
		"clusterMasterEndpoints":             in.ClusterMasterEndpoints,
		"clusterMasterKubeAPIEndpoints":      endpointList(in.ClusterMasterEndpoints, "kubeApiPort"),
		"clusterMasterRPPAddresses":          endpointList(in.ClusterMasterEndpoints, "rppServerPort"),
		"clusterMasterRPPBootstrapAddresses": endpointList(in.ClusterMasterEndpoints, "rppBootstrapServerPort"),
		"clusterUUID":                        in.ClusterUUID,
		"images":                             in.Images,
		"packagesProxy":                      in.PackagesProxy,
		"mingetB64":                          in.MingetB64,
		"provider":                           in.Provider,
		"Files":                              in.Files,
		"Values": map[string]any{
			"nodeManager": map[string]any{
				"internal": map[string]any{
					"clusterMasterAddresses": in.APIServerEndpoints,
				},
			},
		},
	}
}

// endpointList joins address and the named port of every endpoint that has it,
// skipping the rest — the same shape helm builds with `if hasKey`.
func endpointList(endpoints []map[string]any, portKey string) []string {
	out := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		port, ok := endpoint[portKey]
		if !ok {
			continue
		}
		out = append(out, fmt.Sprintf("%v:%v", endpoint["address"], port))
	}
	return out
}
