/*
Copyright 2022 Flant JSC

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

package kinds

import (
	"testing"

	"github.com/flant/kube-client/fake"

	"github.com/stretchr/testify/require"

	"k8s.io/client-go/restmapper"

	"github.com/flant/constraint_exporter/pkg/gatekeeper"

	"github.com/stretchr/testify/assert"
)

func TestDeduplicate(t *testing.T) {
	con := gatekeeper.Constraint{Spec: gatekeeper.ConstraintSpec{Match: gatekeeper.Match{
		Kinds: []gatekeeper.MatchKind{
			{
				APIGroups: []string{""},
				Kinds:     []string{"Pod"},
			},
			{
				APIGroups: []string{""},
				Kinds:     []string{"Pod"},
			},
			{
				APIGroups: []string{"networking.k8s.io", "extensions"},
				Kinds:     []string{"Ingress"},
			},
		},
	}}}

	con2 := gatekeeper.Constraint{Spec: gatekeeper.ConstraintSpec{Match: gatekeeper.Match{
		Kinds: []gatekeeper.MatchKind{
			{
				APIGroups: []string{""},
				Kinds:     []string{"Pod"},
			},
			{
				APIGroups: []string{"extensions", "networking.k8s.io"},
				Kinds:     []string{"Ingress"},
			},
		},
	}}}

	res, _ := deduplicateKinds([]gatekeeper.Constraint{con, con2})

	assert.Len(t, res, 2)
	assert.Equal(t, res[":Pod"], gatekeeper.MatchKind{APIGroups: []string{""}, Kinds: []string{"Pod"}})
	assert.Equal(t, res["extensions,networking.k8s.io:Ingress"], gatekeeper.MatchKind{APIGroups: []string{"extensions", "networking.k8s.io"}, Kinds: []string{"Ingress"}})
}

func TestResourceMapper(t *testing.T) {
	cl := fake.NewFakeCluster(fake.ClusterVersionV121)

	apiRes, err := restmapper.GetAPIGroupResources(cl.Discovery)
	require.NoError(t, err)

	rmatch := resourceMatcher{
		apiGroupResources: apiRes,
		mapper:            restmapper.NewDiscoveryRESTMapper(apiRes),
	}

	t.Run("deduplicate objects with wildcard and empty apiGroup", func(t *testing.T) {
		kinds := []gatekeeper.MatchKind{
			{
				APIGroups: []string{"*"},
				Kinds:     []string{"Pod"},
			},
			{
				APIGroups: []string{""},
				Kinds:     []string{"Pod"},
			},
		}

		data, err := rmatch.convertKindsToResource(kinds)
		require.NoError(t, err)
		assert.YAMLEq(t, `
- apiGroups:
  - ""
  resources:
  - pods
`, string(data))
	})

	t.Run("dedup equal resources", func(t *testing.T) {
		kinds := []gatekeeper.MatchKind{
			{
				APIGroups: []string{"extensions", "networking.k8s.io"},
				Kinds:     []string{"Ingress"},
			},
			{
				APIGroups: []string{"networking.k8s.io"},
				Kinds:     []string{"Ingress"},
			},
		}

		data, err := rmatch.convertKindsToResource(kinds)
		require.NoError(t, err)
		assert.YAMLEq(t, `
- apiGroups:
  - extensions
  - networking.k8s.io
  resources:
  - ingresses
`, string(data))
	})

	t.Run("keep different resources", func(t *testing.T) {
		kinds := []gatekeeper.MatchKind{
			{
				APIGroups: []string{""},
				Kinds:     []string{"Pod"},
			},
			{
				APIGroups: []string{"networking.k8s.io"},
				Kinds:     []string{"Ingress"},
			},
			{
				APIGroups: []string{"apps"},
				Kinds:     []string{"Deployment"},
			},
			{
				APIGroups: []string{""},
				Kinds:     []string{"PodExecOptions"},
			},
		}

		data, err := rmatch.convertKindsToResource(kinds)
		require.NoError(t, err)
		assert.YAMLEq(t, `
- apiGroups:
  - ""
  resources:
  - pods
- apiGroups:
  - networking.k8s.io
  resources:
  - ingresses
- apiGroups:
  - apps
  resources:
  - deployments
- apiGroups:
  - ""
  resources:
  - pods/attach
  - pods/exec
  operations:
  - CONNECT
`, string(data))
	})

	t.Run("wrong kind", func(t *testing.T) {
		kinds := []gatekeeper.MatchKind{
			{
				APIGroups: []string{""},
				Kinds:     []string{"Pod"},
			},
			{
				APIGroups: []string{"apps/v1"},
				Kinds:     []string{"Deployment"},
			},
		}

		data, err := rmatch.convertKindsToResource(kinds)
		require.NoError(t, err)
		assert.YAMLEq(t, `
- apiGroups:
  - ""
  resources:
  - pods
`, string(data))
	})
}
