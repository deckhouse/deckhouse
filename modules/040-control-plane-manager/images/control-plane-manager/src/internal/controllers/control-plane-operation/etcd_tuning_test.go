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
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

const etcdManifestWithDefaults = `apiVersion: v1
kind: Pod
metadata:
  name: etcd
  namespace: kube-system
spec:
  containers:
  - name: etcd
    image: etcd:3.6
    env:
    - name: ETCD_HEARTBEAT_INTERVAL
      value: "100"
    - name: ETCD_ELECTION_TIMEOUT
      value: "1000"
`

var arbiterParams = etcdPerformanceParams{HeartbeatIntervalMs: 500, ElectionTimeoutMs: 5000}

func TestApplyEtcdPerformanceTuning_OverridesArbiterValues(t *testing.T) {
	out, err := applyEtcdPerformanceTuning([]byte(etcdManifestWithDefaults), arbiterParams)
	require.NoError(t, err)

	pod := mustUnmarshalPod(t, out)
	require.Len(t, pod.Spec.Containers, 1)
	require.Equal(t, "etcd", pod.Spec.Containers[0].Name)

	envByName := envMap(pod.Spec.Containers[0].Env)
	require.Equal(t, "500", envByName["ETCD_HEARTBEAT_INTERVAL"])
	require.Equal(t, "5000", envByName["ETCD_ELECTION_TIMEOUT"])
}

func TestApplyEtcdPerformanceTuning_PreservesUnrelatedFields(t *testing.T) {
	out, err := applyEtcdPerformanceTuning([]byte(etcdManifestWithDefaults), arbiterParams)
	require.NoError(t, err)

	pod := mustUnmarshalPod(t, out)
	require.Equal(t, "etcd", pod.Name)
	require.Equal(t, "kube-system", pod.Namespace)
	require.Equal(t, "etcd:3.6", pod.Spec.Containers[0].Image)
}

func TestApplyEtcdPerformanceTuning_ErrorWhenEtcdContainerMissing(t *testing.T) {
	manifest := `apiVersion: v1
kind: Pod
metadata:
  name: etcd
spec:
  containers:
  - name: not-etcd
    env:
    - name: ETCD_HEARTBEAT_INTERVAL
      value: "100"
    - name: ETCD_ELECTION_TIMEOUT
      value: "1000"
`
	_, err := applyEtcdPerformanceTuning([]byte(manifest), arbiterParams)
	require.Error(t, err)
	require.Contains(t, err.Error(), `etcd container "etcd" not found`)
}

func TestApplyEtcdPerformanceTuning_ErrorWhenEnvVarMissingInTemplate(t *testing.T) {
	manifest := `apiVersion: v1
kind: Pod
metadata:
  name: etcd
spec:
  containers:
  - name: etcd
    env:
    - name: ETCD_HEARTBEAT_INTERVAL
      value: "100"
`
	_, err := applyEtcdPerformanceTuning([]byte(manifest), arbiterParams)
	require.Error(t, err)
	require.Contains(t, err.Error(), `env "ETCD_ELECTION_TIMEOUT" not declared`)
}

func TestApplyEtcdPerformanceTuning_ErrorOnValueFrom(t *testing.T) {
	manifest := `apiVersion: v1
kind: Pod
metadata:
  name: etcd
spec:
  containers:
  - name: etcd
    env:
    - name: ETCD_HEARTBEAT_INTERVAL
      valueFrom:
        configMapKeyRef:
          name: etcd-tuning
          key: heartbeat
    - name: ETCD_ELECTION_TIMEOUT
      value: "1000"
`
	_, err := applyEtcdPerformanceTuning([]byte(manifest), arbiterParams)
	require.Error(t, err)
	require.Contains(t, err.Error(), "has valueFrom")
	require.Contains(t, err.Error(), "ETCD_HEARTBEAT_INTERVAL")
}

func TestApplyEtcdPerformanceTuning_ErrorOnMalformedYAML(t *testing.T) {
	_, err := applyEtcdPerformanceTuning([]byte("not: a: valid: pod\n"), arbiterParams)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unmarshal etcd pod manifest")
}

func TestEtcdPerformanceParamsForNode(t *testing.T) {
	t.Run("non-arbiter node returns nil", func(t *testing.T) {
		require.Nil(t, etcdPerformanceParamsForNode(NodeIdentity{EtcdArbiter: false}))
	})

	t.Run("arbiter node returns relaxed params", func(t *testing.T) {
		params := etcdPerformanceParamsForNode(NodeIdentity{EtcdArbiter: true})
		require.NotNil(t, params)
		require.Equal(t, 500, params.HeartbeatIntervalMs)
		require.Equal(t, 5000, params.ElectionTimeoutMs)
	})
}

// helpers

func mustUnmarshalPod(t *testing.T, b []byte) *corev1.Pod {
	t.Helper()
	pod := &corev1.Pod{}
	require.NoError(t, yaml.Unmarshal(b, pod))
	return pod
}

func envMap(env []corev1.EnvVar) map[string]string {
	m := make(map[string]string, len(env))
	for _, e := range env {
		m[e.Name] = e.Value
	}
	return m
}
