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
	"fmt"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

		cms, _ := c.kubeClient.DeckhouseV1alpha1().ModuleSources().Get(context.TODO(), "test-source", metav1.GetOptions{})
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

		ms, err = c.kubeClient.DeckhouseV1alpha1().ModuleSources().Get(context.TODO(), "test-source", metav1.GetOptions{})
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

		ms, err = c.kubeClient.DeckhouseV1alpha1().ModuleSources().Get(context.TODO(), "test-source-2", metav1.GetOptions{})
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

		ms, err = c.kubeClient.DeckhouseV1alpha1().ModuleSources().Get(context.TODO(), "test-source-3", metav1.GetOptions{})
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
	modulePullOverrideInformer := informerFactory.Deckhouse().V1alpha1().ModulePullOverrides()
	defer informerFactory.Start(ctx.Done())

	return NewController(cs, moduleSourceInformer, moduleReleaseInformer, moduleUpdatePolicyInformer, modulePullOverrideInformer, nil)
}

func createFakeModuleSource(cs versioned.Interface, yamlObj string) (*v1alpha1.ModuleSource, error) {
	var ms *v1alpha1.ModuleSource
	err := yaml.Unmarshal([]byte(yamlObj), &ms)
	if err != nil {
		return nil, err
	}

	ms, err = cs.DeckhouseV1alpha1().ModuleSources().Create(context.TODO(), ms, metav1.CreateOptions{})

	return ms, err
}

// nolint: unparam
func createFakeModuleRelease(cs versioned.Interface, yamlObj string) (*v1alpha1.ModuleRelease, error) {
	var mr *v1alpha1.ModuleRelease
	err := yaml.Unmarshal([]byte(yamlObj), &mr)
	if err != nil {
		return nil, err
	}

	mr, err = cs.DeckhouseV1alpha1().ModuleReleases().Create(context.TODO(), mr, metav1.CreateOptions{})

	return mr, err
}

func TestGetReleasePolicy(t *testing.T) {
	embeddedDeckhousePolicy := &v1alpha1.ModuleUpdatePolicySpec{
		Update: v1alpha1.ModuleUpdatePolicySpecUpdate{
			Mode: "Auto",
		},
		ReleaseChannel: "Stable",
	}
	c := &Controller{logger: log.New(), deckhouseEmbeddedPolicy: embeddedDeckhousePolicy}

	t.Run("Exact match policy", func(t *testing.T) {
		policy := &v1alpha1.ModuleUpdatePolicy{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ModuleUpdatePolicy",
				APIVersion: "deckhouse.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-1",
			},
			Spec: v1alpha1.ModuleUpdatePolicySpec{
				ModuleReleaseSelector: v1alpha1.ModuleUpdatePolicySpecReleaseSelector{
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"source": "test-1", "module": "test-module-1"}},
				},
			},
		}
		mup, err := c.getReleasePolicy("test-1", "test-module-1", []*v1alpha1.ModuleUpdatePolicy{policy})
		require.NoError(t, err)
		assert.Equal(t, "test-1", mup.Name)
	})

	t.Run("Only module match policy", func(t *testing.T) {
		policy := &v1alpha1.ModuleUpdatePolicy{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ModuleUpdatePolicy",
				APIVersion: "deckhouse.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-2",
			},
			Spec: v1alpha1.ModuleUpdatePolicySpec{
				ModuleReleaseSelector: v1alpha1.ModuleUpdatePolicySpecReleaseSelector{
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"module": "test-module-2"}},
				},
			},
		}
		mup, err := c.getReleasePolicy("test-2", "test-module-2", []*v1alpha1.ModuleUpdatePolicy{policy})
		require.NoError(t, err)
		assert.Equal(t, "test-2", mup.Name)
	})

	t.Run("Only source match policy", func(t *testing.T) {
		policy := &v1alpha1.ModuleUpdatePolicy{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ModuleUpdatePolicy",
				APIVersion: "deckhouse.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-3",
			},
			Spec: v1alpha1.ModuleUpdatePolicySpec{
				ModuleReleaseSelector: v1alpha1.ModuleUpdatePolicySpecReleaseSelector{
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"source": "test-3"}},
				},
			},
		}
		mup, err := c.getReleasePolicy("test-3", "test-module-3", []*v1alpha1.ModuleUpdatePolicy{policy})
		require.NoError(t, err)
		assert.Equal(t, "test-3", mup.Name)
	})

	t.Run("Except module policy", func(t *testing.T) {
		policy := &v1alpha1.ModuleUpdatePolicy{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ModuleUpdatePolicy",
				APIVersion: "deckhouse.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-4",
			},
			Spec: v1alpha1.ModuleUpdatePolicySpec{
				ModuleReleaseSelector: v1alpha1.ModuleUpdatePolicySpecReleaseSelector{
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"source": "test-4", "module": "foobar"}},
				},
			},
		}
		mup, err := c.getReleasePolicy("test-4", "test-module-4", []*v1alpha1.ModuleUpdatePolicy{policy})
		fmt.Println(mup)
		require.NoError(t, err)
		assert.Equal(t, "", mup.Name)
	})

	t.Run("No policy set", func(t *testing.T) {
		mup, err := c.getReleasePolicy("test-5", "test-module-5", []*v1alpha1.ModuleUpdatePolicy{})
		require.NoError(t, err)
		assert.Equal(t, "", mup.Name)
	})
}
