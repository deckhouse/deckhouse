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

func TestTrackValidateNodeGroupMetrics(t *testing.T) {
	yandex := Provider{Type: "yandex"}

	for _, tc := range []struct {
		name        string
		nodeType    v1.NodeType
		declared    string
		provider    Provider
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
			wantInvalid: 1,
		},
		{
			// 'None' is a declaration, not an absent one: over a cloud it is invalid, and reporting
			// it as unset on top would tell the operator to fill a field that is filled.
			name:        "declaring None over a cloud is invalid and not unset",
			nodeType:    v1.NodeTypeCloudStatic,
			declared:    "None",
			provider:    yandex,
			wantInvalid: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ng := &v1.NodeGroup{Spec: v1.NodeGroupSpec{NodeType: tc.nodeType, ProviderType: tc.declared}}
			ng.Name = "worker"

			TrackValidateNodeGroupMetrics(ng, tc.provider)

			assert.Equal(t, tc.wantUnset, nodeGroupSeries(providerTypeUnset, ng.Name))
			assert.Equal(t, tc.wantInvalid, nodeGroupSeries(providerTypeInvalid, ng.Name))
		})
	}
}

// The cluster runs one cloud, so the node type alone decides which provider a group runs in:
// everything but Static belongs to the one the cluster configuration names, whatever InstanceClass
// kind the group references. spec.providerType picks nothing — it declares that answer, and a
// declaration that disagrees is the error this returns.
func TestValidateNodeGroupProvider(t *testing.T) {
	yandex := Provider{Type: "yandex", InstanceClassKind: "YandexInstanceClass"}
	aws := Provider{Type: "aws", InstanceClassKind: "AWSInstanceClass"}
	// Kept on load for the InstanceClass kind it carries; it is nobody's default.
	nameless := Provider{InstanceClassKind: "VsphereInstanceClass"}

	inYandexCloud := NewCatalog([]Provider{aws, yandex, nameless}, yandex)
	staticCluster := NewCatalog([]Provider{nameless}, Provider{})

	tests := []struct {
		name     string
		pCatalog Catalog
		ng       *v1.NodeGroup
		declared string
		want     string
		wantErr  bool
	}{
		{
			// The kind a group references does not pick its provider: a kind mismatch is a verdict
			// about the NodeGroup, and derived_status.RunCloudChecks is what reports it.
			name:     "CloudEphemeral takes the cluster provider, not the one its kind belongs to",
			pCatalog: inYandexCloud, ng: cloudEphemeral("worker-aws", "AWSInstanceClass"), want: "yandex",
		},
		{
			name:     "CloudEphemeral without a classReference takes the cluster provider",
			pCatalog: inYandexCloud, ng: nodeGroupOfType("worker", v1.NodeTypeCloudEphemeral), want: "yandex",
		},
		{
			// CloudPermanent nodes are created by the installer and reference no InstanceClass, so
			// the cluster configuration is the only thing left to name their provider.
			name:     "CloudPermanent takes the cluster provider",
			pCatalog: inYandexCloud, ng: nodeGroupOfType("master", v1.NodeTypeCloudPermanent), want: "yandex",
		},
		{
			// CloudStatic nodes do run in the cluster's cloud, Deckhouse just does not order them:
			// they still need the provider steps and the cloud variables.
			name:     "CloudStatic takes the cluster provider",
			pCatalog: inYandexCloud, ng: nodeGroupOfType("cloudstatic", v1.NodeTypeCloudStatic), want: "yandex",
		},
		{
			// The whole point of the per-NodeGroup provider: a Static node lives outside every
			// cloud, so the provider steps must not reach it even in a cloud cluster.
			name:     "Static resolves to no provider in a cloud cluster",
			pCatalog: inYandexCloud, ng: nodeGroupOfType("static", v1.NodeTypeStatic),
		},
		{
			name:     "a cluster that names no provider resolves to nothing",
			pCatalog: staticCluster, ng: nodeGroupOfType("cloudstatic", v1.NodeTypeCloudStatic),
		},

		{
			name:     "declaring the resolved provider, case-insensitively",
			pCatalog: inYandexCloud, ng: nodeGroupOfType("cloudstatic", v1.NodeTypeCloudStatic),
			declared: "Yandex", want: "yandex",
		},
		{
			name:     "declaring another provider",
			pCatalog: inYandexCloud, ng: nodeGroupOfType("cloudstatic", v1.NodeTypeCloudStatic),
			declared: "aws", want: "yandex", wantErr: true,
		},
		{
			// None is how a group outside every cloud spells it.
			name:     "declaring None on Static",
			pCatalog: inYandexCloud, ng: nodeGroupOfType("static", v1.NodeTypeStatic),
			declared: "None",
		},
		{
			name:     "declaring none in a cluster that has none",
			pCatalog: staticCluster, ng: nodeGroupOfType("cloudstatic", v1.NodeTypeCloudStatic),
			declared: "none",
		},
		{
			name:     "declaring None in a cloud",
			pCatalog: inYandexCloud, ng: nodeGroupOfType("cloudstatic", v1.NodeTypeCloudStatic),
			declared: "None", want: "yandex", wantErr: true,
		},
		{
			// A Static group runs in no cloud, so naming one is wrong even where the cluster has it.
			name:     "declaring a provider on Static",
			pCatalog: inYandexCloud, ng: nodeGroupOfType("static", v1.NodeTypeStatic),
			declared: "yandex", wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.ng.Spec.ProviderType = tc.declared

			got := tc.pCatalog.ByNodeGroup(tc.ng)
			err := ValidateNodeGroup(tc.ng, got)

			assert.Equal(t, tc.want, got.Type, "the provider is the answer even when the declaration is not")
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
