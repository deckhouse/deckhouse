package ensure_crds

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	apimachineryv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/flant/kube-client/fake"

	"github.com/stretchr/testify/require"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/shell-operator/pkg/kube/object_patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

func TestEnsureCRDs(t *testing.T) {
	cluster := fake.NewFakeCluster(fake.ClusterVersionV125)
	dependency.TestDC.K8sClient = cluster.Client

	patcher := object_patch.NewObjectPatcher(cluster.Client)

	pc := object_patch.NewPatchCollector()
	merr := EnsureCRDs("./test_data/**", &go_hook.HookInput{PatchCollector: pc}, dependency.TestDC)
	require.NoError(t, merr.ErrorOrNil())

	err := patcher.ExecuteOperations(pc.Operations())
	require.NoError(t, err)

	//
	list, err := cluster.Client.Dynamic().Resource(schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}).List(context.TODO(), apimachineryv1.ListOptions{})
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

func BenchmarkEnsureCRDs(b *testing.B) {
	path := "./test_data/**"
	dc := dependency.TestDC
	in := &go_hook.HookInput{PatchCollector: object_patch.NewPatchCollector()}
	//b.Run("old", func(b *testing.B) {
	//	_ = EnsureCRDs(path, in, dc)
	//})

	b.Run("new", func(b *testing.B) {
		_ = EnsureCRDs(path, in, dc)
	})
}
