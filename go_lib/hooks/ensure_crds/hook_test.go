/*
Copyright 2023 Flant JSC

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

package ensure_crds

import (
	"context"
	"sort"
	"testing"

	"github.com/flant/kube-client/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimachineryv1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

func TestEnsureCRDs(t *testing.T) {
	cluster := fake.NewFakeCluster(fake.ClusterVersionV125)
	dependency.TestDC.K8sClient = cluster.Client

	merr := EnsureCRDs("./test_data/**", dependency.TestDC)
	require.Equal(t, 1, merr.Len())
	assert.Errorf(t, merr.Errors[0], "invalid CRD document apiversion/kind: 'v1/Pod'")

	//
	list, err := cluster.Client.Dynamic().Resource(crdGVR).List(context.TODO(), apimachineryv1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, list.Items, 4)

	expected := []string{
		"modulereleases.deckhouse.io",
		"modules.deckhouse.io",
		"modulesources.deckhouse.io",
		"prometheuses.monitoring.coreos.com",
	}

	result := make([]string, 0, len(expected))
	for _, item := range list.Items {
		result = append(result, item.GetName())
	}
	sort.Strings(result)
	assert.Equal(t, expected, result)
}
