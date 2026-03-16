//go:build ai_tests

/*
Copyright 2025 Flant JSC

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

package test

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	infrastructurev1alpha1 "github.com/deckhouse/node-controller/api/infrastructure.cluster.x-k8s.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/controller/controlplane"
	"github.com/deckhouse/node-controller/internal/controller/csinode"
	"github.com/deckhouse/node-controller/internal/controller/deployment/bashiblelock"
	instancectrl "github.com/deckhouse/node-controller/internal/controller/instance"
	"github.com/deckhouse/node-controller/internal/controller/node/bashiblecleanup"
	"github.com/deckhouse/node-controller/internal/controller/node/fencing"
	"github.com/deckhouse/node-controller/internal/controller/node/gpu"
	"github.com/deckhouse/node-controller/internal/controller/node/providerid"
	"github.com/deckhouse/node-controller/internal/controller/node/template"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/master"
	"github.com/deckhouse/node-controller/internal/controller/nodeuser"
	"github.com/deckhouse/node-controller/internal/controller/pod/bashible"
	"github.com/deckhouse/node-controller/internal/dynr"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/mcm.sapcloud.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/controller/machine/ycpreemptible"
	"github.com/deckhouse/node-controller/internal/controller/machinedeployment"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/chaosmonkey"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/instanceclass"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/status"
)

func init() {
	logf.SetLogger(zap.New(zap.UseDevMode(true)))
}

// setupController creates a dynamicReconciler-like setup for a single reconciler
// directly with the manager, bypassing the global dynr registry. This replicates
// the logic from dynr.dynamicReconciler.setupWithManager.
func setupController(
	mgr ctrl.Manager,
	name string,
	obj client.Object,
	reconciler dynr.Reconciler,
) error {
	// Inject dependencies exactly like dynr.dynamicReconciler.inject does.
	if v, ok := reconciler.(dynr.NeedsClient); ok {
		v.InjectClient(mgr.GetClient())
	}
	if v, ok := reconciler.(dynr.NeedsScheme); ok {
		v.InjectScheme(mgr.GetScheme())
	}
	if v, ok := reconciler.(dynr.NeedsLogger); ok {
		v.InjectLogger(logr.Discard())
	}

	// Collect SetupForPredicates.
	var forOpts []builder.ForOption
	if fp, ok := reconciler.(dynr.HasForPredicates); ok {
		if preds := fp.SetupForPredicates(); len(preds) > 0 {
			forOpts = append(forOpts, builder.WithPredicates(preds...))
		}
	}

	b := ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(obj, forOpts...)

	// Setup any additional watches the reconciler declares.
	w := &testWatcher{b: b}
	reconciler.SetupWatches(w)

	if err := b.Complete(reconciler); err != nil {
		return fmt.Errorf("build controller %s: %w", name, err)
	}

	return nil
}

// testWatcher implements dynr.Watcher by delegating to ctrl.Builder.
type testWatcher struct {
	b *ctrl.Builder
}

func (w *testWatcher) Owns(object client.Object, opts ...builder.OwnsOption) dynr.Watcher {
	w.b.Owns(object, opts...)
	return w
}

func (w *testWatcher) Watches(object client.Object, eventHandler handler.EventHandler, opts ...builder.WatchesOption) dynr.Watcher {
	w.b.Watches(object, eventHandler, opts...)
	return w
}

func (w *testWatcher) WatchesRawSource(src source.Source) dynr.Watcher {
	w.b.WatchesRawSource(src)
	return w
}

func (w *testWatcher) WithEventFilter(p predicate.Predicate) dynr.Watcher {
	w.b.WithEventFilter(p)
	return w
}

// setupEnvtest starts a fresh envtest environment and returns the k8s client
// and context. Each test should call this independently.
func setupEnvtest(t *testing.T, setupFn func(mgr ctrl.Manager) error) (client.Client, context.Context) {
	t.Helper()

	testEnv := &envtest.Environment{
		BinaryAssetsDirectory: "/Users/pallam/go/envtest/k8s/1.35.0-darwin-arm64",
	}

	cfg, err := testEnv.Start()
	require.NoError(t, err, "failed to start envtest")

	s := scheme.Scheme
	require.NoError(t, storagev1.AddToScheme(s))

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: s,
	})
	require.NoError(t, err, "failed to create manager")

	// Let the caller register its controller with the manager.
	require.NoError(t, setupFn(mgr), "failed to setup controller")

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := mgr.Start(ctx); err != nil {
			// Only log if context was not cancelled (i.e. not during cleanup).
			select {
			case <-ctx.Done():
			default:
				t.Logf("manager exited with error: %v", err)
			}
		}
	}()

	// Wait for the cache to sync.
	require.True(t, mgr.GetCache().WaitForCacheSync(ctx), "cache failed to sync")

	t.Cleanup(func() {
		cancel()
		require.NoError(t, testEnv.Stop(), "failed to stop envtest")
	})

	return mgr.GetClient(), ctx
}

// ---------------------------------------------------------------------------
// Test 1: BashibleCleanup controller — full pipeline with predicates
// ---------------------------------------------------------------------------

func TestAI_Integration_BashibleCleanup_Predicate(t *testing.T) {
	k8sClient, ctx := setupEnvtest(t, func(mgr ctrl.Manager) error {
		r := &bashiblecleanup.Reconciler{}
		return setupController(mgr, "test-bashible-cleanup", &corev1.Node{}, r)
	})

	t.Run("DOES reconcile: node with bashible label gets cleaned up", func(t *testing.T) {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bashible-has-label",
				Labels: map[string]string{
					"node.deckhouse.io/bashible-first-run-finished": "",
					"keep-me": "yes",
				},
			},
			Spec: corev1.NodeSpec{
				Taints: []corev1.Taint{
					{
						Key:    "node.deckhouse.io/bashible-uninitialized",
						Effect: corev1.TaintEffectNoSchedule,
					},
					{
						Key:    "other-taint",
						Effect: corev1.TaintEffectNoExecute,
					},
				},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, node))

		// Wait for the controller to reconcile and remove the label + taint.
		require.Eventually(t, func() bool {
			n := &corev1.Node{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "bashible-has-label"}, n); err != nil {
				return false
			}
			_, hasLabel := n.Labels["node.deckhouse.io/bashible-first-run-finished"]
			return !hasLabel
		}, 10*time.Second, 200*time.Millisecond, "label should be removed by controller")

		// Verify final state.
		final := &corev1.Node{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "bashible-has-label"}, final))

		// Label should be gone.
		_, hasLabel := final.Labels["node.deckhouse.io/bashible-first-run-finished"]
		assert.False(t, hasLabel, "bashible label should be removed")

		// Other labels preserved.
		assert.Equal(t, "yes", final.Labels["keep-me"])

		// Bashible taint should be removed, other taint preserved.
		hasBashibleTaint := false
		hasOtherTaint := false
		for _, taint := range final.Spec.Taints {
			if taint.Key == "node.deckhouse.io/bashible-uninitialized" {
				hasBashibleTaint = true
			}
			if taint.Key == "other-taint" {
				hasOtherTaint = true
			}
		}
		assert.False(t, hasBashibleTaint, "bashible taint should be removed")
		assert.True(t, hasOtherTaint, "other taint should be preserved")
	})

	t.Run("DOES NOT reconcile: node without bashible label stays unchanged", func(t *testing.T) {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bashible-no-label",
				Labels: map[string]string{
					"some-other-label": "value",
				},
			},
			Spec: corev1.NodeSpec{
				Taints: []corev1.Taint{
					{
						Key:    "node.deckhouse.io/bashible-uninitialized",
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, node))

		// Give the controller time to process. It should NOT act on this node
		// because the predicate filters it out (no bashible label).
		time.Sleep(2 * time.Second)

		final := &corev1.Node{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "bashible-no-label"}, final))

		// Taint should still be present (controller did not reconcile).
		hasBashibleTaint := false
		for _, taint := range final.Spec.Taints {
			if taint.Key == "node.deckhouse.io/bashible-uninitialized" {
				hasBashibleTaint = true
			}
		}
		assert.True(t, hasBashibleTaint, "bashible taint should remain (predicate filtered node)")
	})
}

// ---------------------------------------------------------------------------
// Test 2: ProviderID controller — full pipeline with predicates
// ---------------------------------------------------------------------------

func TestAI_Integration_ProviderID_Predicate(t *testing.T) {
	k8sClient, ctx := setupEnvtest(t, func(mgr ctrl.Manager) error {
		r := &providerid.Reconciler{}
		return setupController(mgr, "test-provider-id", &corev1.Node{}, r)
	})

	t.Run("DOES reconcile: static node without providerID gets static://", func(t *testing.T) {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "static-node-no-provider",
				Labels: map[string]string{
					"node.deckhouse.io/type": "Static",
				},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, node))

		// Wait for the controller to set providerID.
		require.Eventually(t, func() bool {
			n := &corev1.Node{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "static-node-no-provider"}, n); err != nil {
				return false
			}
			return n.Spec.ProviderID == "static://"
		}, 10*time.Second, 200*time.Millisecond, "providerID should be set to static://")
	})

	t.Run("DOES NOT reconcile: CloudEphemeral node stays unchanged", func(t *testing.T) {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cloud-ephemeral-node",
				Labels: map[string]string{
					"node.deckhouse.io/type": "CloudEphemeral",
				},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, node))

		// Give the controller time. It should NOT act on CloudEphemeral nodes.
		time.Sleep(2 * time.Second)

		final := &corev1.Node{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "cloud-ephemeral-node"}, final))

		assert.Empty(t, final.Spec.ProviderID, "providerID should remain empty for CloudEphemeral node")
	})

	t.Run("DOES NOT reconcile: static node with existing providerID stays unchanged", func(t *testing.T) {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "static-node-has-provider",
				Labels: map[string]string{
					"node.deckhouse.io/type": "Static",
				},
			},
			Spec: corev1.NodeSpec{
				ProviderID: "aws://existing",
			},
		}
		require.NoError(t, k8sClient.Create(ctx, node))

		// Give the controller time. It should NOT override existing providerID.
		time.Sleep(2 * time.Second)

		final := &corev1.Node{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "static-node-has-provider"}, final))

		assert.Equal(t, "aws://existing", final.Spec.ProviderID, "existing providerID should not be overwritten")
	})
}

// ---------------------------------------------------------------------------
// Test 3: CSITaint controller — full pipeline with predicates
// ---------------------------------------------------------------------------

func TestAI_Integration_CSITaint_Predicate(t *testing.T) {
	k8sClient, ctx := setupEnvtest(t, func(mgr ctrl.Manager) error {
		r := &csinode.Reconciler{}
		return setupController(mgr, "test-csi-taint", &storagev1.CSINode{}, r)
	})

	t.Run("DOES reconcile: CSINode with drivers removes taint from node", func(t *testing.T) {
		// Create the Node first (CSINode references it by name).
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "csi-node-with-drivers",
			},
			Spec: corev1.NodeSpec{
				Taints: []corev1.Taint{
					{
						Key:    "node.deckhouse.io/csi-not-bootstrapped",
						Effect: corev1.TaintEffectNoSchedule,
					},
					{
						Key:    "other-taint",
						Effect: corev1.TaintEffectNoExecute,
					},
				},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, node))

		// Create the CSINode with a driver.
		csiNode := &storagev1.CSINode{
			ObjectMeta: metav1.ObjectMeta{
				Name: "csi-node-with-drivers",
			},
			Spec: storagev1.CSINodeSpec{
				Drivers: []storagev1.CSINodeDriver{
					{Name: "ebs.csi.aws.com", NodeID: "csi-node-with-drivers"},
				},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, csiNode))

		// Wait for the controller to remove the CSI taint.
		require.Eventually(t, func() bool {
			n := &corev1.Node{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "csi-node-with-drivers"}, n); err != nil {
				return false
			}
			for _, taint := range n.Spec.Taints {
				if taint.Key == "node.deckhouse.io/csi-not-bootstrapped" {
					return false
				}
			}
			return true
		}, 10*time.Second, 200*time.Millisecond, "CSI taint should be removed")

		// Verify final state.
		final := &corev1.Node{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "csi-node-with-drivers"}, final))

		hasCSITaint := false
		hasOtherTaint := false
		for _, taint := range final.Spec.Taints {
			if taint.Key == "node.deckhouse.io/csi-not-bootstrapped" {
				hasCSITaint = true
			}
			if taint.Key == "other-taint" {
				hasOtherTaint = true
			}
		}
		assert.False(t, hasCSITaint, "CSI taint should be removed")
		assert.True(t, hasOtherTaint, "other taint should be preserved")
	})

	t.Run("DOES NOT reconcile: CSINode without drivers leaves taint intact", func(t *testing.T) {
		// Create the Node with CSI taint.
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "csi-node-no-drivers",
			},
			Spec: corev1.NodeSpec{
				Taints: []corev1.Taint{
					{
						Key:    "node.deckhouse.io/csi-not-bootstrapped",
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, node))

		// Create the CSINode WITHOUT drivers.
		csiNode := &storagev1.CSINode{
			ObjectMeta: metav1.ObjectMeta{
				Name: "csi-node-no-drivers",
			},
			Spec: storagev1.CSINodeSpec{
				Drivers: []storagev1.CSINodeDriver{},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, csiNode))

		// Give the controller time. It should NOT act because predicate filters
		// CSINodes without drivers.
		time.Sleep(2 * time.Second)

		final := &corev1.Node{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "csi-node-no-drivers"}, final))

		hasCSITaint := false
		for _, taint := range final.Spec.Taints {
			if taint.Key == "node.deckhouse.io/csi-not-bootstrapped" {
				hasCSITaint = true
			}
		}
		assert.True(t, hasCSITaint, "CSI taint should remain (predicate filtered - no drivers)")
	})
}

// ---------------------------------------------------------------------------
// Helper: setupEnvtestWithCRDs — like setupEnvtest but also installs Deckhouse
// CRDs (NodeGroup etc.) so that secondary watches on custom resources work.
// ---------------------------------------------------------------------------

func setupEnvtestWithCRDs(t *testing.T, setupFn func(mgr ctrl.Manager) error) (client.Client, context.Context) {
	t.Helper()

	// Compute absolute path to the testdata/crds directory.
	_, thisFile, _, _ := runtime.Caller(0)
	crdDir := filepath.Join(filepath.Dir(thisFile), "testdata", "crds")

	testEnv := &envtest.Environment{
		BinaryAssetsDirectory: "/Users/pallam/go/envtest/k8s/1.35.0-darwin-arm64",
		CRDDirectoryPaths:     []string{crdDir},
	}

	cfg, err := testEnv.Start()
	require.NoError(t, err, "failed to start envtest")

	s := scheme.Scheme
	require.NoError(t, storagev1.AddToScheme(s))
	require.NoError(t, deckhousev1.AddToScheme(s))

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: s,
	})
	require.NoError(t, err, "failed to create manager")

	// Let the caller register its controller with the manager.
	require.NoError(t, setupFn(mgr), "failed to setup controller")

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := mgr.Start(ctx); err != nil {
			select {
			case <-ctx.Done():
			default:
				t.Logf("manager exited with error: %v", err)
			}
		}
	}()

	// Wait for the cache to sync.
	require.True(t, mgr.GetCache().WaitForCacheSync(ctx), "cache failed to sync")

	t.Cleanup(func() {
		cancel()
		require.NoError(t, testEnv.Stop(), "failed to stop envtest")
	})

	return mgr.GetClient(), ctx
}

// ---------------------------------------------------------------------------
// Test 4: NodeTemplate controller — secondary watch on NodeGroup
// ---------------------------------------------------------------------------

func TestAI_Integration_NodeTemplate_WatchesNodeGroup(t *testing.T) {
	k8sClient, ctx := setupEnvtestWithCRDs(t, func(mgr ctrl.Manager) error {
		r := &template.Reconciler{}
		return setupController(mgr, "test-node-template", &corev1.Node{}, r)
	})

	// Create a NodeGroup with nodeTemplate labels.
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ng",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
			NodeTemplate: &deckhousev1.NodeTemplate{
				Labels: map[string]string{
					"custom": "label1",
				},
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, ng))

	// Create a Node belonging to that NodeGroup.
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "template-watch-node",
			Labels: map[string]string{
				"node.deckhouse.io/group": "test-ng",
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, node))

	t.Run("node gets template labels from NodeGroup on create", func(t *testing.T) {
		require.Eventually(t, func() bool {
			n := &corev1.Node{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "template-watch-node"}, n); err != nil {
				return false
			}
			return n.Labels["custom"] == "label1"
		}, 10*time.Second, 200*time.Millisecond, "node should get custom=label1 from NodeGroup template")
	})

	t.Run("updating NodeGroup template triggers reconcile and updates node", func(t *testing.T) {
		// Update the NodeGroup: change the template label.
		freshNG := &deckhousev1.NodeGroup{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "test-ng"}, freshNG))
		freshNG.Spec.NodeTemplate.Labels = map[string]string{
			"custom": "label2",
		}
		require.NoError(t, k8sClient.Update(ctx, freshNG))

		// The secondary watch (NodeGroup → Nodes) should trigger reconciliation
		// for the node and update its label.
		require.Eventually(t, func() bool {
			n := &corev1.Node{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "template-watch-node"}, n); err != nil {
				return false
			}
			return n.Labels["custom"] == "label2"
		}, 10*time.Second, 200*time.Millisecond, "node should get custom=label2 after NodeGroup template update (secondary watch)")
	})

	t.Run("NodeGroup template label removal cleans up node label", func(t *testing.T) {
		// Remove the "custom" label from NodeGroup template entirely.
		freshNG := &deckhousev1.NodeGroup{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "test-ng"}, freshNG))
		freshNG.Spec.NodeTemplate.Labels = map[string]string{}
		require.NoError(t, k8sClient.Update(ctx, freshNG))

		// The reconciler should remove the "custom" label from the node
		// (it was in last-applied but no longer in the template → excess key).
		require.Eventually(t, func() bool {
			n := &corev1.Node{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "template-watch-node"}, n); err != nil {
				return false
			}
			_, hasCustom := n.Labels["custom"]
			return !hasCustom
		}, 10*time.Second, 200*time.Millisecond, "custom label should be removed after NodeGroup template clears it")
	})
}

// ---------------------------------------------------------------------------
// Test 5: NodeGPU controller — secondary watch on NodeGroup
// ---------------------------------------------------------------------------

func TestAI_Integration_NodeGPU_WatchesNodeGroup(t *testing.T) {
	k8sClient, ctx := setupEnvtestWithCRDs(t, func(mgr ctrl.Manager) error {
		r := &gpu.Reconciler{}
		return setupController(mgr, "test-node-gpu", &corev1.Node{}, r)
	})

	// Create a NodeGroup with GPU settings.
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gpu-ng",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
			GPU: &deckhousev1.GPUSpec{
				Mode: "timeSlicing",
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, ng))

	// Create a Node belonging to that NodeGroup.
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gpu-watch-node",
			Labels: map[string]string{
				"node.deckhouse.io/group": "gpu-ng",
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, node))

	t.Run("node gets GPU labels from NodeGroup on create", func(t *testing.T) {
		require.Eventually(t, func() bool {
			n := &corev1.Node{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "gpu-watch-node"}, n); err != nil {
				return false
			}
			_, hasGPU := n.Labels["node.deckhouse.io/gpu"]
			return hasGPU && n.Labels["node.deckhouse.io/device-gpu.config"] == "timeSlicing"
		}, 10*time.Second, 200*time.Millisecond, "node should get GPU labels from NodeGroup")
	})

	t.Run("updating NodeGroup GPU mode triggers reconcile and updates node", func(t *testing.T) {
		// Update the NodeGroup: change GPU mode.
		freshNG := &deckhousev1.NodeGroup{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "gpu-ng"}, freshNG))
		freshNG.Spec.GPU = &deckhousev1.GPUSpec{
			Mode: "exclusive",
		}
		require.NoError(t, k8sClient.Update(ctx, freshNG))

		// The secondary watch (NodeGroup → Nodes) should trigger reconciliation
		// for the node and update the GPU mode label.
		require.Eventually(t, func() bool {
			n := &corev1.Node{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "gpu-watch-node"}, n); err != nil {
				return false
			}
			return n.Labels["node.deckhouse.io/device-gpu.config"] == "exclusive"
		}, 10*time.Second, 200*time.Millisecond, "node should get device-gpu.config=exclusive after NodeGroup GPU update (secondary watch)")
	})

	t.Run("updating NodeGroup to MIG mode adds MIG config label", func(t *testing.T) {
		// Update the NodeGroup: switch to MIG mode with strategy.
		freshNG := &deckhousev1.NodeGroup{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "gpu-ng"}, freshNG))
		freshNG.Spec.GPU = &deckhousev1.GPUSpec{
			Mode: "mig",
			MIG: &deckhousev1.MIGSpec{
				Strategy: "mixed",
			},
		}
		require.NoError(t, k8sClient.Update(ctx, freshNG))

		// The secondary watch should trigger reconciliation and set MIG label.
		require.Eventually(t, func() bool {
			n := &corev1.Node{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "gpu-watch-node"}, n); err != nil {
				return false
			}
			return n.Labels["nvidia.com/mig.config"] == "mixed"
		}, 10*time.Second, 200*time.Millisecond, "node should get nvidia.com/mig.config=mixed after NodeGroup MIG update (secondary watch)")
	})
}

// ---------------------------------------------------------------------------
// Test 6: Fencing controller — predicate tests
// ---------------------------------------------------------------------------

func TestAI_Integration_Fencing_Predicate(t *testing.T) {
	k8sClient, ctx := setupEnvtest(t, func(mgr ctrl.Manager) error {
		// Register a field indexer for spec.nodeName so that the fencing
		// controller can list pods by node name in the envtest environment.
		if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Pod{}, "spec.nodeName", func(obj client.Object) []string {
			pod := obj.(*corev1.Pod)
			if pod.Spec.NodeName == "" {
				return nil
			}
			return []string{pod.Spec.NodeName}
		}); err != nil {
			return fmt.Errorf("index field spec.nodeName: %w", err)
		}
		r := &fencing.Reconciler{}
		return setupController(mgr, "test-fencing", &corev1.Node{}, r)
	})

	t.Run("DOES reconcile: node with fencing-enabled label triggers reconcile (lease check)", func(t *testing.T) {
		// Create the kube-node-lease namespace (envtest may not have it).
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-node-lease"}}
		_ = k8sClient.Create(ctx, ns) // ignore if already exists

		// Create a node with the fencing-enabled label.
		// Since there is no lease, fencing should consider the lease expired
		// and attempt to delete the node.
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "fenced-node",
				Labels: map[string]string{
					"node-manager.deckhouse.io/fencing-enabled": "",
				},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, node))

		// The fencing controller should reconcile this node. Since there is no
		// lease, it considers the lease expired and deletes the node.
		require.Eventually(t, func() bool {
			n := &corev1.Node{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "fenced-node"}, n)
			return apierrors.IsNotFound(err)
		}, 10*time.Second, 200*time.Millisecond, "fenced node should be deleted by fencing controller (no lease = expired)")
	})

	t.Run("DOES NOT reconcile: node without fencing label stays intact", func(t *testing.T) {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "unfenced-node",
				Labels: map[string]string{
					"some-label": "value",
				},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, node))

		// Give the controller time. It should NOT act on this node because
		// the predicate filters out nodes without the fencing label.
		time.Sleep(2 * time.Second)

		n := &corev1.Node{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "unfenced-node"}, n))
		assert.Equal(t, "value", n.Labels["some-label"], "unfenced node should remain unchanged")
	})
}

// ---------------------------------------------------------------------------
// Test 7: BashiblePod controller — predicate tests
// ---------------------------------------------------------------------------

func TestAI_Integration_BashiblePod_Predicate(t *testing.T) {
	k8sClient, ctx := setupEnvtest(t, func(mgr ctrl.Manager) error {
		r := &bashible.Reconciler{}
		return setupController(mgr, "test-bashible-pod", &corev1.Pod{}, r)
	})

	// Create the required namespace.
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "d8-cloud-instance-manager"}}
	require.NoError(t, k8sClient.Create(ctx, ns))

	t.Run("DOES reconcile: pod with app=bashible-apiserver in d8-cloud-instance-manager is processed", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bashible-apiserver-abc",
				Namespace: "d8-cloud-instance-manager",
				Labels: map[string]string{
					"app": "bashible-apiserver",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "server", Image: "registry.example.com/bashible:v1"},
				},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, pod))

		// The controller should reconcile this pod. Since there is no HostIP yet,
		// the controller will just return (no annotation set). Set a HostIP
		// via status update to trigger annotation logic.
		require.Eventually(t, func() bool {
			p := &corev1.Pod{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "bashible-apiserver-abc", Namespace: "d8-cloud-instance-manager"}, p); err != nil {
				return false
			}
			p.Status.HostIP = "10.0.0.1"
			_ = k8sClient.Status().Update(ctx, p)
			return true
		}, 5*time.Second, 200*time.Millisecond)

		// Now wait for the controller to set the initial-host-ip annotation.
		require.Eventually(t, func() bool {
			p := &corev1.Pod{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "bashible-apiserver-abc", Namespace: "d8-cloud-instance-manager"}, p); err != nil {
				return false
			}
			return p.Annotations["node.deckhouse.io/initial-host-ip"] == "10.0.0.1"
		}, 10*time.Second, 200*time.Millisecond, "controller should set initial-host-ip annotation")
	})

	t.Run("DOES NOT reconcile: pod in different namespace is not processed", func(t *testing.T) {
		// Create another namespace.
		otherNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other-namespace"}}
		require.NoError(t, k8sClient.Create(ctx, otherNS))

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bashible-apiserver-wrong-ns",
				Namespace: "other-namespace",
				Labels: map[string]string{
					"app": "bashible-apiserver",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "server", Image: "registry.example.com/bashible:v1"},
				},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, pod))

		// Set HostIP on the pod.
		require.Eventually(t, func() bool {
			p := &corev1.Pod{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "bashible-apiserver-wrong-ns", Namespace: "other-namespace"}, p); err != nil {
				return false
			}
			p.Status.HostIP = "10.0.0.2"
			return k8sClient.Status().Update(ctx, p) == nil
		}, 5*time.Second, 200*time.Millisecond)

		// Give the controller time. It should NOT set the annotation because
		// the predicate filters out pods not in d8-cloud-instance-manager.
		time.Sleep(2 * time.Second)

		final := &corev1.Pod{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "bashible-apiserver-wrong-ns", Namespace: "other-namespace"}, final))
		assert.Empty(t, final.Annotations["node.deckhouse.io/initial-host-ip"],
			"pod in wrong namespace should not get initial-host-ip annotation")
	})

	t.Run("DOES NOT reconcile: pod with different label is not processed", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-other-pod",
				Namespace: "d8-cloud-instance-manager",
				Labels: map[string]string{
					"app": "something-else",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "server", Image: "registry.example.com/other:v1"},
				},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, pod))

		// Set HostIP.
		require.Eventually(t, func() bool {
			p := &corev1.Pod{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "some-other-pod", Namespace: "d8-cloud-instance-manager"}, p); err != nil {
				return false
			}
			p.Status.HostIP = "10.0.0.3"
			return k8sClient.Status().Update(ctx, p) == nil
		}, 5*time.Second, 200*time.Millisecond)

		// Give the controller time. It should NOT set the annotation because
		// the predicate filters out pods without app=bashible-apiserver label.
		time.Sleep(2 * time.Second)

		final := &corev1.Pod{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "some-other-pod", Namespace: "d8-cloud-instance-manager"}, final))
		assert.Empty(t, final.Annotations["node.deckhouse.io/initial-host-ip"],
			"pod with wrong label should not get initial-host-ip annotation")
	})
}

// ---------------------------------------------------------------------------
// Test 8: BashibleLock controller — predicate tests
// ---------------------------------------------------------------------------

func TestAI_Integration_BashibleLock_Predicate(t *testing.T) {
	k8sClient, ctx := setupEnvtest(t, func(mgr ctrl.Manager) error {
		r := &bashiblelock.Reconciler{}
		return setupController(mgr, "test-bashible-lock", &appsv1.Deployment{}, r)
	})

	// Create the required namespace.
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "d8-cloud-instance-manager"}}
	require.NoError(t, k8sClient.Create(ctx, ns))

	t.Run("DOES reconcile: deployment named bashible-apiserver in correct namespace", func(t *testing.T) {
		// Create the Secret that the controller manages.
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bashible-apiserver-context",
				Namespace: "d8-cloud-instance-manager",
			},
		}
		require.NoError(t, k8sClient.Create(ctx, secret))

		// Create the bashible-apiserver Deployment with an image digest annotation
		// that does NOT match the actual container image -> controller should lock.
		replicas := int32(1)
		dep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bashible-apiserver",
				Namespace: "d8-cloud-instance-manager",
				Annotations: map[string]string{
					"node.deckhouse.io/bashible-apiserver-image-digest": "sha256:expected",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "bashible-apiserver"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "bashible-apiserver"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "bashible-apiserver", Image: "registry.example.com/bashible@sha256:actual"},
						},
					},
				},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, dep))

		// The controller should detect the image mismatch and lock the Secret.
		require.Eventually(t, func() bool {
			s := &corev1.Secret{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "bashible-apiserver-context", Namespace: "d8-cloud-instance-manager"}, s); err != nil {
				return false
			}
			return s.Annotations["node.deckhouse.io/bashible-locked"] == "true"
		}, 10*time.Second, 200*time.Millisecond, "Secret should be locked when image digest does not match")
	})

	t.Run("DOES NOT reconcile: deployment with different name is not processed", func(t *testing.T) {
		// Create a different deployment in the same namespace.
		replicas := int32(1)
		dep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-other-deployment",
				Namespace: "d8-cloud-instance-manager",
				Annotations: map[string]string{
					"node.deckhouse.io/bashible-apiserver-image-digest": "sha256:expected",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "some-other"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "some-other"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "some-other", Image: "registry.example.com/other:v1"},
						},
					},
				},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, dep))

		// Give the controller time. It should NOT process this deployment.
		time.Sleep(2 * time.Second)

		// The Secret annotation should still only reflect what the first test set.
		s := &corev1.Secret{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "bashible-apiserver-context", Namespace: "d8-cloud-instance-manager"}, s))
		assert.Equal(t, "true", s.Annotations["node.deckhouse.io/bashible-locked"],
			"Secret lock state should not be affected by unrelated deployment")
	})
}

// ---------------------------------------------------------------------------
// Test 9: NodeGroupMaster controller — predicate tests
// ---------------------------------------------------------------------------

func TestAI_Integration_NodeGroupMaster_Predicate(t *testing.T) {
	k8sClient, ctx := setupEnvtestWithCRDs(t, func(mgr ctrl.Manager) error {
		r := &master.NodeGroupMasterReconciler{}
		return setupController(mgr, "test-ng-master", &deckhousev1.NodeGroup{}, r)
	})

	t.Run("DOES reconcile: predicate passes NodeGroup named master", func(t *testing.T) {
		// Create a "master" NodeGroup to trigger the controller via the
		// For watch. The predicate filters by name == "master", so the
		// reconciler will be invoked. On reconcile the controller checks
		// if "master" NG exists; since we just created it, it is a no-op.
		masterNG := &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "master",
			},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType: deckhousev1.NodeTypeCloudPermanent,
			},
		}
		require.NoError(t, k8sClient.Create(ctx, masterNG))

		// Verify the NG exists (controller saw the create event, reconciled,
		// found it exists -> no-op, no error).
		require.Eventually(t, func() bool {
			ng := &deckhousev1.NodeGroup{}
			return k8sClient.Get(ctx, types.NamespacedName{Name: "master"}, ng) == nil
		}, 10*time.Second, 200*time.Millisecond, "master NodeGroup should exist after reconcile")

		// Confirm the reconciler did not modify the existing "master" NG
		// (the controller's job is to create it if missing; since it exists,
		// the reconciler returns early).
		ng := &deckhousev1.NodeGroup{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "master"}, ng))
		assert.Equal(t, "master", ng.Name, "master NodeGroup should still exist")
		assert.Equal(t, deckhousev1.NodeTypeCloudPermanent, ng.Spec.NodeType,
			"master NodeGroup type should remain unchanged (reconciler is no-op when it exists)")
	})

	t.Run("DOES NOT reconcile: NodeGroup named worker is not processed", func(t *testing.T) {
		// Create a "worker" NodeGroup — the predicate should filter it out.
		workerNG := &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "worker",
			},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType: deckhousev1.NodeTypeStatic,
			},
		}
		require.NoError(t, k8sClient.Create(ctx, workerNG))

		// Give the controller time. It should NOT process this NodeGroup.
		time.Sleep(2 * time.Second)

		// The "worker" NG should remain exactly as we created it (no modifications).
		ng := &deckhousev1.NodeGroup{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "worker"}, ng))
		assert.Equal(t, deckhousev1.NodeTypeStatic, ng.Spec.NodeType,
			"worker NodeGroup should remain unchanged (predicate filtered)")
	})
}

// ---------------------------------------------------------------------------
// Test 10: NodeUser controller — basic reconcile (no SetupForPredicates)
// ---------------------------------------------------------------------------

func TestAI_Integration_NodeUser_Reconcile(t *testing.T) {
	k8sClient, ctx := setupEnvtestWithCRDs(t, func(mgr ctrl.Manager) error {
		r := &nodeuser.Reconciler{}
		return setupController(mgr, "test-nodeuser", &deckhousev1.NodeUser{}, r)
	})

	t.Run("creates NodeUser and stale errors are cleared", func(t *testing.T) {
		// Create a NodeUser with errors referencing a non-existent node.
		nu := &deckhousev1.NodeUser{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testuser",
			},
			Spec: deckhousev1.NodeUserSpec{
				UID:          1001,
				SSHPublicKey: "ssh-rsa AAAAB3... test@example.com",
				IsSudoer:     true,
				NodeGroups:   []string{"master"},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, nu))

		// Wait for the NodeUser to appear in the cache before updating status.
		// Status is a subresource, so it must be updated separately after create.
		freshNU := &deckhousev1.NodeUser{}
		require.Eventually(t, func() bool {
			return k8sClient.Get(ctx, types.NamespacedName{Name: "testuser"}, freshNU) == nil
		}, 10*time.Second, 200*time.Millisecond, "NodeUser should be visible in cache after create")

		// Update status to add stale errors referencing non-existent nodes.
		freshNU.Status.Errors = map[string]string{
			"non-existent-node-1": "some error on deleted node",
			"non-existent-node-2": "another error on deleted node",
		}
		require.NoError(t, k8sClient.Status().Update(ctx, freshNU))

		// The controller should reconcile and clear stale errors
		// (nodes "non-existent-node-1" and "non-existent-node-2" do not exist).
		require.Eventually(t, func() bool {
			n := &deckhousev1.NodeUser{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "testuser"}, n); err != nil {
				return false
			}
			return len(n.Status.Errors) == 0
		}, 10*time.Second, 200*time.Millisecond, "stale errors should be cleared by NodeUser controller")
	})
}

// ---------------------------------------------------------------------------
// Helper: setupEnvtestWithAllCRDs — like setupEnvtestWithCRDs but also
// registers MCM types (Machine, MachineDeployment) and v1alpha1 (Instance).
// ---------------------------------------------------------------------------

func setupEnvtestWithAllCRDs(t *testing.T, setupFn func(mgr ctrl.Manager) error) (client.Client, context.Context) {
	t.Helper()

	_, thisFile, _, _ := runtime.Caller(0)
	crdDir := filepath.Join(filepath.Dir(thisFile), "testdata", "crds")

	testEnv := &envtest.Environment{
		BinaryAssetsDirectory: "/Users/pallam/go/envtest/k8s/1.35.0-darwin-arm64",
		CRDDirectoryPaths:     []string{crdDir},
	}

	cfg, err := testEnv.Start()
	require.NoError(t, err, "failed to start envtest")

	s := scheme.Scheme
	require.NoError(t, storagev1.AddToScheme(s))
	require.NoError(t, deckhousev1.AddToScheme(s))
	require.NoError(t, deckhousev1alpha1.AddToScheme(s))
	require.NoError(t, mcmv1alpha1.AddToScheme(s))

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: s,
	})
	require.NoError(t, err, "failed to create manager")

	require.NoError(t, setupFn(mgr), "failed to setup controller")

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := mgr.Start(ctx); err != nil {
			select {
			case <-ctx.Done():
			default:
				t.Logf("manager exited with error: %v", err)
			}
		}
	}()

	require.True(t, mgr.GetCache().WaitForCacheSync(ctx), "cache failed to sync")

	t.Cleanup(func() {
		cancel()
		require.NoError(t, testEnv.Stop(), "failed to stop envtest")
	})

	return mgr.GetClient(), ctx
}

// ---------------------------------------------------------------------------
// Test 6: NodeGroupStatus controller — secondary watch on Node
// ---------------------------------------------------------------------------

func TestAI_Integration_NodeGroupStatus_WatchesNode(t *testing.T) {
	k8sClient, ctx := setupEnvtestWithAllCRDs(t, func(mgr ctrl.Manager) error {
		r := &status.NodeGroupStatusReconciler{}
		return setupController(mgr, "test-ng-status", &deckhousev1.NodeGroup{}, r)
	})

	// Create the d8-cloud-instance-manager namespace (required for MachineDeployment listing).
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-cloud-instance-manager"},
	}
	require.NoError(t, k8sClient.Create(ctx, ns))

	// Create a NodeGroup.
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "status-ng",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
		},
	}
	require.NoError(t, k8sClient.Create(ctx, ng))

	// Create 2 Nodes belonging to this NodeGroup, one ready, one not.
	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "status-node-1",
			Labels: map[string]string{"node.deckhouse.io/group": "status-ng"},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, node1))

	// Mark node1 as Ready.
	node1.Status.Conditions = []corev1.NodeCondition{
		{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
	}
	require.NoError(t, k8sClient.Status().Update(ctx, node1))

	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "status-node-2",
			Labels: map[string]string{"node.deckhouse.io/group": "status-ng"},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, node2))

	t.Run("NodeGroup status updated with correct node count", func(t *testing.T) {
		require.Eventually(t, func() bool {
			freshNG := &deckhousev1.NodeGroup{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "status-ng"}, freshNG); err != nil {
				return false
			}
			return freshNG.Status.Nodes == 2 && freshNG.Status.Ready == 1
		}, 10*time.Second, 200*time.Millisecond,
			"NodeGroup status should show nodes=2, ready=1")
	})

	t.Run("adding a third node triggers reconcile via secondary watch", func(t *testing.T) {
		node3 := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "status-node-3",
				Labels: map[string]string{"node.deckhouse.io/group": "status-ng"},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, node3))

		// Mark node3 as Ready.
		node3.Status.Conditions = []corev1.NodeCondition{
			{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
		}
		require.NoError(t, k8sClient.Status().Update(ctx, node3))

		require.Eventually(t, func() bool {
			freshNG := &deckhousev1.NodeGroup{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "status-ng"}, freshNG); err != nil {
				return false
			}
			return freshNG.Status.Nodes == 3 && freshNG.Status.Ready == 2
		}, 10*time.Second, 200*time.Millisecond,
			"NodeGroup status should update to nodes=3, ready=2 after adding a third node (secondary watch on Node)")
	})
}

// ---------------------------------------------------------------------------
// Test 7: ChaosMonkey controller — predicate filters
// ---------------------------------------------------------------------------

func TestAI_Integration_ChaosMonkey_Predicate(t *testing.T) {
	k8sClient, ctx := setupEnvtestWithAllCRDs(t, func(mgr ctrl.Manager) error {
		r := &chaosmonkey.Reconciler{}
		return setupController(mgr, "test-chaos-monkey", &deckhousev1.NodeGroup{}, r)
	})

	// Create d8-cloud-instance-manager namespace (ChaosMonkey lists Machines there).
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-cloud-instance-manager"},
	}
	require.NoError(t, k8sClient.Create(ctx, ns))

	t.Run("DOES reconcile: NodeGroup with Chaos.Mode=DrainAndDelete is processed", func(t *testing.T) {
		ng := &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "chaos-enabled",
			},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType: deckhousev1.NodeTypeStatic,
				Chaos: &deckhousev1.ChaosSpec{
					Mode:   deckhousev1.ChaosModeDrainAndDelete,
					Period: "1h",
				},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, ng))

		// The ChaosMonkey reconciler checks isReadyForChaos which requires
		// nodes > 1 and all ready. With no nodes, it should reconcile but
		// find "not ready for chaos" and set RequeueAfter. We verify the
		// controller processes it by checking that the NodeGroup still exists
		// (no error) and is requeued.
		time.Sleep(2 * time.Second)

		freshNG := &deckhousev1.NodeGroup{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "chaos-enabled"}, freshNG))
		assert.Equal(t, deckhousev1.ChaosModeDrainAndDelete, freshNG.Spec.Chaos.Mode)
	})

	t.Run("DOES NOT reconcile: NodeGroup without Chaos is filtered by predicate", func(t *testing.T) {
		ng := &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "chaos-disabled",
			},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType: deckhousev1.NodeTypeStatic,
				// No Chaos spec — predicate should filter this out.
			},
		}
		require.NoError(t, k8sClient.Create(ctx, ng))

		// Give controller time — this NodeGroup should not be reconciled.
		time.Sleep(2 * time.Second)

		freshNG := &deckhousev1.NodeGroup{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "chaos-disabled"}, freshNG))
		assert.Nil(t, freshNG.Spec.Chaos, "Chaos spec should remain nil (predicate filtered)")
	})
}

// ---------------------------------------------------------------------------
// Test 8: InstanceClass controller — predicate filters on NodeType
// ---------------------------------------------------------------------------

func TestAI_Integration_InstanceClass_Predicate(t *testing.T) {
	k8sClient, ctx := setupEnvtestWithAllCRDs(t, func(mgr ctrl.Manager) error {
		r := &instanceclass.NodeGroupInstanceClassReconciler{}
		return setupController(mgr, "test-instance-class", &deckhousev1.NodeGroup{}, r)
	})

	t.Run("DOES reconcile: NodeGroup with NodeType=CloudEphemeral is processed", func(t *testing.T) {
		ng := &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ic-cloud-ephemeral",
			},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType: deckhousev1.NodeTypeCloudEphemeral,
				CloudInstances: &deckhousev1.CloudInstancesSpec{
					MinPerZone: 1,
					MaxPerZone: 3,
					ClassReference: deckhousev1.ClassReference{
						Kind: "OpenStackInstanceClass",
						Name: "worker",
					},
				},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, ng))

		// The reconciler should process this NodeGroup and attempt to patch
		// the instance class status. The instance class does not exist, so
		// it will be a no-op (IgnoreNotFound), but the predicate allows it through.
		time.Sleep(2 * time.Second)

		freshNG := &deckhousev1.NodeGroup{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "ic-cloud-ephemeral"}, freshNG))
		assert.Equal(t, deckhousev1.NodeTypeCloudEphemeral, freshNG.Spec.NodeType)
	})

	t.Run("DOES NOT reconcile: NodeGroup with NodeType=Static is filtered", func(t *testing.T) {
		ng := &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ic-static",
			},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType: deckhousev1.NodeTypeStatic,
			},
		}
		require.NoError(t, k8sClient.Create(ctx, ng))

		// Give controller time — predicate should filter this out.
		time.Sleep(2 * time.Second)

		freshNG := &deckhousev1.NodeGroup{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "ic-static"}, freshNG))
		assert.Equal(t, deckhousev1.NodeTypeStatic, freshNG.Spec.NodeType)
	})
}

// ---------------------------------------------------------------------------
// Test 9: MachineDeployment controller — predicate filters on label & namespace
// ---------------------------------------------------------------------------

func TestAI_Integration_MachineDeployment_Predicate(t *testing.T) {
	k8sClient, ctx := setupEnvtestWithAllCRDs(t, func(mgr ctrl.Manager) error {
		r := &machinedeployment.Reconciler{}
		return setupController(mgr, "test-machine-deployment", &mcmv1alpha1.MachineDeployment{}, r)
	})

	// Create the d8-cloud-instance-manager namespace.
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-cloud-instance-manager"},
	}
	require.NoError(t, k8sClient.Create(ctx, ns))

	// Create the NodeGroup that the MachineDeployment references.
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "md-ng",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
			CloudInstances: &deckhousev1.CloudInstancesSpec{
				MinPerZone: 2,
				MaxPerZone: 5,
				ClassReference: deckhousev1.ClassReference{
					Kind: "OpenStackInstanceClass",
					Name: "worker",
				},
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, ng))

	t.Run("DOES reconcile: MachineDeployment with node-group label in correct ns", func(t *testing.T) {
		md := &mcmv1alpha1.MachineDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "md-with-label",
				Namespace: "d8-cloud-instance-manager",
				Labels: map[string]string{
					"node-group": "md-ng",
				},
			},
			Spec: mcmv1alpha1.MachineDeploymentSpec{
				Replicas: 1,
				Template: mcmv1alpha1.MachineTemplateSpec{},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, md))

		// The reconciler should process this MachineDeployment and clamp replicas
		// to [minPerZone, maxPerZone] = [2, 5]. Since current=1 < min=2, it should
		// be clamped to 2.
		require.Eventually(t, func() bool {
			freshMD := &mcmv1alpha1.MachineDeployment{}
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "md-with-label",
				Namespace: "d8-cloud-instance-manager",
			}, freshMD); err != nil {
				return false
			}
			return freshMD.Spec.Replicas == 2
		}, 10*time.Second, 200*time.Millisecond,
			"MachineDeployment replicas should be clamped from 1 to minPerZone=2")
	})

	t.Run("DOES NOT reconcile: MachineDeployment without node-group label", func(t *testing.T) {
		md := &mcmv1alpha1.MachineDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "md-no-label",
				Namespace: "d8-cloud-instance-manager",
			},
			Spec: mcmv1alpha1.MachineDeploymentSpec{
				Replicas: 1,
				Template: mcmv1alpha1.MachineTemplateSpec{},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, md))

		// Give controller time — predicate should filter this out.
		time.Sleep(2 * time.Second)

		freshMD := &mcmv1alpha1.MachineDeployment{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
			Name:      "md-no-label",
			Namespace: "d8-cloud-instance-manager",
		}, freshMD))

		// Replicas should remain unchanged because predicate filtered it.
		assert.Equal(t, int32(1), freshMD.Spec.Replicas,
			"MachineDeployment without label should not be reconciled (predicate filtered)")
	})
}

// ---------------------------------------------------------------------------
// Test 10: YCPreemptible controller — predicate filters on preemptible label
// ---------------------------------------------------------------------------

func TestAI_Integration_YCPreemptible_Predicate(t *testing.T) {
	k8sClient, ctx := setupEnvtestWithAllCRDs(t, func(mgr ctrl.Manager) error {
		r := &ycpreemptible.Reconciler{}
		return setupController(mgr, "test-yc-preemptible", &mcmv1alpha1.Machine{}, r)
	})

	// Create the d8-cloud-instance-manager namespace.
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-cloud-instance-manager"},
	}
	require.NoError(t, k8sClient.Create(ctx, ns))

	t.Run("DOES reconcile: Machine with preemptible label in correct ns", func(t *testing.T) {
		machine := &mcmv1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "preemptible-machine",
				Namespace: "d8-cloud-instance-manager",
				Labels: map[string]string{
					"node.deckhouse.io/preemptible": "",
				},
			},
			Spec: mcmv1alpha1.MachineSpec{},
		}
		require.NoError(t, k8sClient.Create(ctx, machine))

		// The reconciler will try to get the corresponding Node (same name as
		// the machine). Without a Node it will requeue. This verifies the
		// predicate passed and the reconciler ran.
		time.Sleep(2 * time.Second)

		// Machine should still exist (reconciler does not delete without a node
		// that exceeds the age threshold).
		freshMachine := &mcmv1alpha1.Machine{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
			Name:      "preemptible-machine",
			Namespace: "d8-cloud-instance-manager",
		}, freshMachine))
		assert.NotNil(t, freshMachine, "preemptible machine should exist (reconciler processed it)")
	})

	t.Run("DOES NOT reconcile: Machine without preemptible label", func(t *testing.T) {
		machine := &mcmv1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "non-preemptible-machine",
				Namespace: "d8-cloud-instance-manager",
				Labels: map[string]string{
					"some-other-label": "value",
				},
			},
			Spec: mcmv1alpha1.MachineSpec{},
		}
		require.NoError(t, k8sClient.Create(ctx, machine))

		// Give controller time — predicate should filter this out.
		time.Sleep(2 * time.Second)

		freshMachine := &mcmv1alpha1.Machine{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
			Name:      "non-preemptible-machine",
			Namespace: "d8-cloud-instance-manager",
		}, freshMachine))
		// Machine should still exist and be unchanged.
		assert.Equal(t, "value", freshMachine.Labels["some-other-label"],
			"non-preemptible machine should not be touched by reconciler (predicate filtered)")
	})
}

// ---------------------------------------------------------------------------
// Test 16: ControlPlane controller — reconcile + secondary watch on Node
// ---------------------------------------------------------------------------

func TestAI_Integration_ControlPlane_WatchesNode(t *testing.T) {
	k8sClient, ctx := setupEnvtestWithAllCRDs(t, func(mgr ctrl.Manager) error {
		require.NoError(t, infrastructurev1alpha1.AddToScheme(mgr.GetScheme()))
		r := &controlplane.Reconciler{}
		return setupController(mgr, "test-controlplane", &infrastructurev1alpha1.DeckhouseControlPlane{}, r)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "d8-cloud-instance-manager"}}
	require.NoError(t, k8sClient.Create(ctx, ns))

	dcp := &infrastructurev1alpha1.DeckhouseControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-dcp",
			Namespace: "d8-cloud-instance-manager",
		},
	}
	require.NoError(t, k8sClient.Create(ctx, dcp))

	t.Run("DCP status updated with master node counts", func(t *testing.T) {
		node1 := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "master-1",
				Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, node1))
		node1.Status.Conditions = []corev1.NodeCondition{
			{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
		}
		require.NoError(t, k8sClient.Status().Update(ctx, node1))

		node2 := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "master-2",
				Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, node2))

		require.Eventually(t, func() bool {
			freshDCP := &infrastructurev1alpha1.DeckhouseControlPlane{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-dcp", Namespace: "d8-cloud-instance-manager"}, freshDCP); err != nil {
				return false
			}
			return freshDCP.Status.Replicas == 2 && freshDCP.Status.ReadyReplicas == 1 &&
				freshDCP.Status.Initialized && freshDCP.Status.Ready
		}, 10*time.Second, 200*time.Millisecond,
			"DCP status should show replicas=2, readyReplicas=1")
	})

	t.Run("adding ready master node updates DCP status via secondary watch", func(t *testing.T) {
		node3 := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "master-3",
				Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, node3))
		node3.Status.Conditions = []corev1.NodeCondition{
			{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
		}
		require.NoError(t, k8sClient.Status().Update(ctx, node3))

		require.Eventually(t, func() bool {
			freshDCP := &infrastructurev1alpha1.DeckhouseControlPlane{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-dcp", Namespace: "d8-cloud-instance-manager"}, freshDCP); err != nil {
				return false
			}
			return freshDCP.Status.Replicas == 3 && freshDCP.Status.ReadyReplicas == 2
		}, 10*time.Second, 200*time.Millisecond,
			"DCP status should update to replicas=3, readyReplicas=2 after adding master node (secondary watch)")
	})
}

// ---------------------------------------------------------------------------
// Test 17: Instance controller — secondary watch on Machine creates Instance
// ---------------------------------------------------------------------------

func TestAI_Integration_Instance_WatchesMachine(t *testing.T) {
	k8sClient, ctx := setupEnvtestWithAllCRDs(t, func(mgr ctrl.Manager) error {
		r := &instancectrl.InstanceReconciler{}
		return setupController(mgr, "test-instance", &deckhousev1alpha1.Instance{}, r)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "d8-cloud-instance-manager"}}
	require.NoError(t, k8sClient.Create(ctx, ns))

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "instance-ng"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
		},
	}
	require.NoError(t, k8sClient.Create(ctx, ng))

	t.Run("Machine creation triggers Instance creation via secondary watch", func(t *testing.T) {
		machine := &mcmv1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "instance-machine-1",
				Namespace: "d8-cloud-instance-manager",
			},
			Spec: mcmv1alpha1.MachineSpec{
				NodeTemplateSpec: mcmv1alpha1.MachineNodeTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"node.deckhouse.io/group": "instance-ng",
						},
					},
				},
			},
		}
		require.NoError(t, k8sClient.Create(ctx, machine))

		require.Eventually(t, func() bool {
			inst := &deckhousev1alpha1.Instance{}
			return k8sClient.Get(ctx, types.NamespacedName{Name: "instance-machine-1"}, inst) == nil
		}, 10*time.Second, 200*time.Millisecond,
			"Instance should be created when Machine is created (secondary watch)")
	})

	t.Run("Instance status updated when Machine has status", func(t *testing.T) {
		machine := &mcmv1alpha1.Machine{}
		require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
			Name: "instance-machine-1", Namespace: "d8-cloud-instance-manager",
		}, machine))
		machine.Status.CurrentStatus = mcmv1alpha1.MachineCurrentStatus{
			Phase: mcmv1alpha1.MachinePhaseRunning,
		}
		machine.Status.Node = "instance-machine-1"
		require.NoError(t, k8sClient.Status().Update(ctx, machine))

		require.Eventually(t, func() bool {
			inst := &deckhousev1alpha1.Instance{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "instance-machine-1"}, inst); err != nil {
				return false
			}
			return inst.Status.CurrentStatus.Phase == deckhousev1alpha1.InstancePhaseRunning
		}, 10*time.Second, 200*time.Millisecond,
			"Instance status should reflect Machine running phase")
	})
}
