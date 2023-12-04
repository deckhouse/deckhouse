// Copyright 2023 Flant JSC
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

package source

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/clientset/versioned"
	decFake "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/clientset/versioned/fake"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/informers/externalversions"
)

func TestController_CreateReconcile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.TODO(), 15*time.Second)
	defer cancel()

	t.Run("ModuleSource with invalid auth", func(t *testing.T) {
		var ms *v1alpha1.ModuleSource

		c := createFakeController(ctx)

		ms, err := createFakeModuleSource(c.kubeClient, `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: test-source
spec:
  registry:
    dockerCfg: YXNiCg==
    repo: dev-registry.deckhouse.io/deckhouse/modules
    scheme: HTTPS
  releaseChannel: alpha
`)
		require.NoError(t, err)

		result, err := c.createOrUpdateReconcile(ctx, ms)
		require.Error(t, err)
		assert.False(t, result.Requeue, "error have to be permanent, we don't want to reconcile until the ModuleSource will change")
		assert.ErrorContains(t, err, "credentials not found in the dockerCfg")

		cms, _ := c.kubeClient.DeckhouseV1alpha1().ModuleSources().Get(context.TODO(), "test-source", v1.GetOptions{})
		assert.Contains(t, cms.Status.Msg, "credentials not found in the dockerCfg")
		assert.Len(t, cms.Status.AvailableModules, 0)
	})
}

func TestController_DeleteReconcile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.TODO(), 15*time.Second)
	defer cancel()

	c := createFakeController(ctx)

	if ok := cache.WaitForCacheSync(ctx.Done(), c.moduleSourcesSynced, c.moduleReleasesSynced); !ok {
		c.logger.Fatal("failed to wait for caches to sync")
	}

	t.Run("ModuleSource with finalizer and empty releases", func(t *testing.T) {
		var ms *v1alpha1.ModuleSource

		ms, err := createFakeModuleSource(c.kubeClient, `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: test-source
  finalizers:
  - modules.deckhouse.io/release-exists
spec:
  registry:
    dockerCfg: YXNiCg==
    repo: dev-registry.deckhouse.io/deckhouse/modules
    scheme: HTTPS
  releaseChannel: alpha
`)
		require.NoError(t, err)

		time.Sleep(120 * time.Millisecond)

		result, err := c.deleteReconcile(ctx, ms)
		require.NoError(t, err)
		assert.False(t, result.Requeue)
		assert.Empty(t, result.RequeueAfter)

		ms, err = c.kubeClient.DeckhouseV1alpha1().ModuleSources().Get(context.TODO(), "test-source", v1.GetOptions{})
		require.NoError(t, err)
		assert.Len(t, ms.Finalizers, 0)
	})

	t.Run("ModuleSource with finalizer and release", func(t *testing.T) {
		var ms *v1alpha1.ModuleSource

		ms, err := createFakeModuleSource(c.kubeClient, `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: test-source-2
  finalizers:
  - modules.deckhouse.io/release-exists
spec:
  registry:
    dockerCfg: YXNiCg==
    repo: dev-registry.deckhouse.io/deckhouse/modules
    scheme: HTTPS
  releaseChannel: alpha
`)
		require.NoError(t, err)

		_, err = createFakeModuleRelease(c.kubeClient, `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleRelease
metadata:
  labels:
    module: some-module
    release-checksum: ed8ed428a470a76e30ed4f50dd7cf570
    source: test-source-2
    status: deployed
  name: some-module-v0.0.1
  ownerReferences:
  - apiVersion: deckhouse.io/v1alpha1
    controller: true
    kind: ModuleSource
    name: test-source-2
    uid: ec6c2028-39bd-4068-bbda-84587e63e4c4
spec:
  moduleName: some-module
  version: 0.0.1
  weight: 900
status:
  approved: false
  message: ""
  phase: Deployed
`)
		require.NoError(t, err)

		time.Sleep(120 * time.Millisecond)

		result, err := c.deleteReconcile(ctx, ms)
		require.NoError(t, err)
		assert.False(t, result.Requeue)
		assert.Equal(t, 5*time.Second, result.RequeueAfter)

		ms, err = c.kubeClient.DeckhouseV1alpha1().ModuleSources().Get(context.TODO(), "test-source-2", v1.GetOptions{})
		require.NoError(t, err)
		assert.Len(t, ms.Finalizers, 1)
		assert.Equal(t, ms.Status.Msg, "ModuleSource contains at least 1 Deployed release and cannot be deleted. Please delete ModuleRelease manually to continue")
	})

	t.Run("ModuleSource with finalizer,annotation and release", func(t *testing.T) {
		var ms *v1alpha1.ModuleSource

		ms, err := createFakeModuleSource(c.kubeClient, `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: test-source-3
  annotations:
    modules.deckhouse.io/force-delete: "true"
  finalizers:
  - modules.deckhouse.io/release-exists
spec:
  registry:
    dockerCfg: YXNiCg==
    repo: dev-registry.deckhouse.io/deckhouse/modules
    scheme: HTTPS
  releaseChannel: alpha
`)
		require.NoError(t, err)

		_, err = createFakeModuleRelease(c.kubeClient, `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleRelease
metadata:
  labels:
    module: some-module-2
    release-checksum: ed8ed428a470a76e30ed4f50dd7cf570
    source: test-source-3
    status: deployed
  name: some-module-2-v0.0.1
  ownerReferences:
  - apiVersion: deckhouse.io/v1alpha1
    controller: true
    kind: ModuleSource
    name: test-source-3
    uid: ec6c2028-39bd-4068-bbda-84587e63e4c4
spec:
  moduleName: some-module-2
  version: 0.0.1
  weight: 900
status:
  approved: false
  message: ""
  phase: Deployed
`)
		require.NoError(t, err)

		time.Sleep(120 * time.Millisecond)

		result, err := c.deleteReconcile(ctx, ms)
		require.NoError(t, err)
		assert.False(t, result.Requeue)
		assert.Empty(t, result.RequeueAfter)

		ms, err = c.kubeClient.DeckhouseV1alpha1().ModuleSources().Get(context.TODO(), "test-source-3", v1.GetOptions{})
		require.NoError(t, err)
		assert.Len(t, ms.Finalizers, 0)
	})
}

func createFakeController(ctx context.Context) *Controller {
	scheme := runtime.NewScheme()
	utilruntime.Must(v1alpha1.AddToScheme(scheme))

	cs := decFake.NewSimpleClientset()

	informerFactory := externalversions.NewSharedInformerFactory(cs, 15*time.Minute)
	moduleSourceInformer := informerFactory.Deckhouse().V1alpha1().ModuleSources()
	moduleReleaseInformer := informerFactory.Deckhouse().V1alpha1().ModuleReleases()
	moduleUpdatePolicyInformer := informerFactory.Deckhouse().V1alpha1().ModuleUpdatePolicies()
	defer informerFactory.Start(ctx.Done())

	return NewController(cs, moduleSourceInformer, moduleReleaseInformer, moduleUpdatePolicyInformer, nil)
}

func createFakeModuleSource(cs versioned.Interface, yamlObj string) (*v1alpha1.ModuleSource, error) {
	var ms *v1alpha1.ModuleSource
	err := yaml.Unmarshal([]byte(yamlObj), &ms)
	if err != nil {
		return nil, err
	}

	ms, err = cs.DeckhouseV1alpha1().ModuleSources().Create(context.TODO(), ms, v1.CreateOptions{})

	return ms, err
}

// nolint: unparam
func createFakeModuleRelease(cs versioned.Interface, yamlObj string) (*v1alpha1.ModuleRelease, error) {
	var mr *v1alpha1.ModuleRelease
	err := yaml.Unmarshal([]byte(yamlObj), &mr)
	if err != nil {
		return nil, err
	}

	mr, err = cs.DeckhouseV1alpha1().ModuleReleases().Create(context.TODO(), mr, v1.CreateOptions{})

	return mr, err
}
