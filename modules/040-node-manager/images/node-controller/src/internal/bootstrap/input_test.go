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

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Контекст рендера воспроизводит _bootstrap.tpl:1-40 дословно: шаблоны читают
// именно эти ключи, а лишний или переименованный ключ меняет рендер молча.
func TestInputTemplateContext(t *testing.T) {
	in := Input{
		NodeGroup:              map[string]any{"name": "worker", "nodeType": "Static"},
		APIServerEndpoints:     []string{"10.0.0.1:6443"},
		ClusterMasterEndpoints: []map[string]any{{"address": "10.0.0.1", "kubeApiPort": int64(6443), "rppServerPort": int64(4219), "rppBootstrapServerPort": int64(4220)}},
		ClusterUUID:            "uuid-1",
		Images:                 map[string]any{"registrypackages": map[string]any{"jq171": "sha256:jq"}},
		PackagesProxy:          map[string]any{"token": "tok"},
		MingetB64:              "bWluZ2V0",
		Provider:               "yandex",
		KubernetesCA:           "CA",
		BootstrapToken:         "abc.def",
	}

	ctx := in.templateContext()

	assert.Equal(t, []string{"10.0.0.1:6443"}, ctx["apiserverEndpoints"])
	assert.Equal(t, []string{"10.0.0.1:6443"}, ctx["clusterMasterKubeAPIEndpoints"])
	assert.Equal(t, []string{"10.0.0.1:4219"}, ctx["clusterMasterRPPAddresses"])
	assert.Equal(t, []string{"10.0.0.1:4220"}, ctx["clusterMasterRPPBootstrapAddresses"])
	assert.Equal(t, "uuid-1", ctx["clusterUUID"])
	assert.Equal(t, "yandex", ctx["provider"])

	// lib.sh.tpl:496 читает адреса через Values, а не через apiserverEndpoints.
	values := ctx["Values"].(map[string]any)
	internal := values["nodeManager"].(map[string]any)["internal"].(map[string]any)
	assert.Equal(t, []string{"10.0.0.1:6443"}, internal["clusterMasterAddresses"])

	// runType отсутствует: helm его не кладёт, и от этого зависят две ветки.
	_, hasRunType := ctx["runType"]
	assert.False(t, hasRunType, "runType must stay absent to match the helm context")
}

// Порт отсутствует в записи эндпоинта — запись просто не попадает в список,
// как и в helm (`if hasKey $endpoint "kubeApiPort"`).
func TestInputTemplateContextSkipsMissingPorts(t *testing.T) {
	in := Input{ClusterMasterEndpoints: []map[string]any{{"address": "10.0.0.9", "kubeApiPort": int64(6443)}}}

	ctx := in.templateContext()

	assert.Equal(t, []string{"10.0.0.9:6443"}, ctx["clusterMasterKubeAPIEndpoints"])
	assert.Empty(t, ctx["clusterMasterRPPAddresses"])
	assert.Empty(t, ctx["clusterMasterRPPBootstrapAddresses"])
}
