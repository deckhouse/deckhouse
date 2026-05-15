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

package controlplaneoperation

import (
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

const etcdContainerName = "etcd"

type etcdPerformanceParams struct {
	HeartbeatIntervalMs int
	ElectionTimeoutMs   int
}

// etcdPerformanceParamsForNode resolves etcd performance parameters for a node.
// Return (params, true) when a non-default profile applies
// Return (zero, false) when the default profile should be used
func etcdPerformanceParamsForNode(node NodeIdentity) (etcdPerformanceParams, bool) {
	if node.EtcdArbiter {
		return etcdPerformanceParams{
			HeartbeatIntervalMs: 500,
			ElectionTimeoutMs:   5000,
		}, true
	}
	return etcdPerformanceParams{}, false
}

// applyEtcdPerformanceTuning rewrites ETCD_HEARTBEAT_INTERVAL and ETCD_ELECTION_TIMEOUT
// Return an error if a missing entry is detected.
func applyEtcdPerformanceTuning(manifestBytes []byte, params etcdPerformanceParams) ([]byte, error) {
	pod := &corev1.Pod{}
	if err := yaml.Unmarshal(manifestBytes, pod); err != nil {
		return nil, fmt.Errorf("unmarshal etcd pod manifest: %w", err)
	}

	var patched bool
	for i := range pod.Spec.Containers {
		c := &pod.Spec.Containers[i]
		if c.Name != etcdContainerName {
			continue
		}
		if err := setContainerEnvValue(c, "ETCD_HEARTBEAT_INTERVAL", strconv.Itoa(params.HeartbeatIntervalMs)); err != nil {
			return nil, err
		}
		if err := setContainerEnvValue(c, "ETCD_ELECTION_TIMEOUT", strconv.Itoa(params.ElectionTimeoutMs)); err != nil {
			return nil, err
		}
		patched = true
		break
	}
	if !patched {
		return nil, fmt.Errorf("etcd container %q not found in static pod manifest", etcdContainerName)
	}

	out, err := yaml.Marshal(pod)
	if err != nil {
		return nil, fmt.Errorf("marshal etcd pod manifest: %w", err)
	}
	return out, nil
}

// setContainerEnvValue updates the value of an existing env var on a container
// Return an error if the env var is not declared on the container.
func setContainerEnvValue(c *corev1.Container, name, value string) error {
	for i := range c.Env {
		if c.Env[i].Name != name {
			continue
		}
		if c.Env[i].ValueFrom != nil {
			return fmt.Errorf("env %q on container %q has valueFrom; refusing to override", name, c.Name)
		}
		c.Env[i].Value = value
		return nil
	}
	return fmt.Errorf("env %q not declared on container %q in template", name, c.Name)
}
