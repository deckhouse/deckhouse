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

package cloudprovider

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// nodeGroupSeries counts the series a NodeGroup has in a gauge and drops them, which is both the
// assertion and the cleanup these tests need.
func nodeGroupSeries(gauge *prometheus.GaugeVec, name string) int {
	return gauge.DeletePartialMatch(prometheus.Labels{"name": name})
}

func TestTrackProviderType(t *testing.T) {
	yandex := Provider{Type: "yandex"}

	for _, tc := range []struct {
		name        string
		nodeType    v1.NodeType
		declared    string
		provider    Provider
		resolveErr  error
		wantUnset   int
		wantInvalid int
	}{
		{
			name:      "a cloud NodeGroup declaring nothing is unset",
			nodeType:  v1.NodeTypeCloudEphemeral,
			provider:  yandex,
			wantUnset: 1,
		},
		{
			name:     "a cloud NodeGroup declaring its provider is clean",
			nodeType: v1.NodeTypeCloudEphemeral,
			declared: "yandex",
			provider: yandex,
		},
		{
			name:     "a Static NodeGroup has no provider to declare",
			nodeType: v1.NodeTypeStatic,
		},
		{
			name:        "a declaration that does not resolve is invalid",
			nodeType:    v1.NodeTypeStatic,
			declared:    "yandex",
			resolveErr:  errors.New("the nodes of this group run in no cloud"),
			wantInvalid: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ng := &v1.NodeGroup{Spec: v1.NodeGroupSpec{NodeType: tc.nodeType, ProviderType: tc.declared}}
			ng.Name = "worker"

			TrackProviderMetrics(ng, tc.provider, tc.resolveErr)

			assert.Equal(t, tc.wantUnset, nodeGroupSeries(providerTypeUnset, ng.Name))
			assert.Equal(t, tc.wantInvalid, nodeGroupSeries(providerTypeInvalid, ng.Name))
		})
	}
}

// A verdict must not outlive the NodeGroup that earned it, and must not take its neighbours with it.
func TestClearProviderTypeMetrics(t *testing.T) {
	deleted := &v1.NodeGroup{Spec: v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic, ProviderType: "yandex"}}
	deleted.Name = "worker"
	kept := &v1.NodeGroup{Spec: v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral}}
	kept.Name = "other"
	t.Cleanup(func() { ClearProviderMetrics(kept.Name) })

	TrackProviderMetrics(deleted, Provider{}, errors.New("the nodes of this group run in no cloud"))
	TrackProviderMetrics(kept, Provider{Type: "yandex"}, nil)

	ClearProviderMetrics(deleted.Name)

	assert.Equal(t, 0, nodeGroupSeries(providerTypeInvalid, deleted.Name))
	assert.Equal(t, 1, nodeGroupSeries(providerTypeUnset, kept.Name))
}
