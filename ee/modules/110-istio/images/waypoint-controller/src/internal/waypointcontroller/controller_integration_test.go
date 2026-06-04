//go:build integration

/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package waypointcontroller

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	networkv1alpha1 "waypoint-controller/pkg/apis/network.deckhouse.io/v1alpha1"
)

const (
	pollInterval = 250 * time.Millisecond
	pollTimeout  = 30 * time.Second
)

// liveEnvs tracks every envtest Environment started by this test binary.
// The TestMain signal handler uses it to stop them on Ctrl-C — without this
// registry, Go's testing package does not run t.Cleanup callbacks when the
// process is killed by a signal, and the etcd/kube-apiserver subprocesses
// would be orphaned.
var (
	liveEnvsMu sync.Mutex
	liveEnvs   = map[*envtest.Environment]struct{}{}
)

func registerEnv(e *envtest.Environment) {
	liveEnvsMu.Lock()
	defer liveEnvsMu.Unlock()
	liveEnvs[e] = struct{}{}
}

func unregisterEnv(e *envtest.Environment) {
	liveEnvsMu.Lock()
	defer liveEnvsMu.Unlock()
	delete(liveEnvs, e)
}

func stopAllEnvs() {
	liveEnvsMu.Lock()
	envs := make([]*envtest.Environment, 0, len(liveEnvs))
	for e := range liveEnvs {
		envs = append(envs, e)
	}
	liveEnvsMu.Unlock()

	for _, e := range envs {
		_ = e.Stop()
	}
}

// TestMain installs a SIGINT handler so that envtest subprocesses are stopped
// when the test run is interrupted with Ctrl-C. Without this, an interrupted
// run leaves orphaned etcd / kube-apiserver processes.
//
// Note: SIGTERM is not handled here — `go test` does not forward SIGTERM to
// the compiled test binary, so we never receive it. If you need to kill a
// test run with SIGTERM (e.g. from CI), follow it with manual cleanup:
//
//	pkill -f kubebuilder-envtest && rm -rf /tmp/k8s_test_framework_*
func TestMain(m *testing.M) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		fmt.Fprintf(os.Stderr, "received %s, stopping envtest environments...\n", sig)
		stopAllEnvs()
		// Re-raise with the default disposition so the process exits with
		// the conventional 128+signo code.
		signal.Reset(sig)
		_ = syscall.Kill(os.Getpid(), sig.(syscall.Signal))
	}()

	code := m.Run()
	// A panicking test may bypass t.Cleanup; stop anything still registered.
	stopAllEnvs()
	os.Exit(code)
}

type testEnv struct {
	env    *envtest.Environment
	mgr    manager.Manager
	client client.Client
}

// managerStopTimeout bounds how long we wait for mgr.Start to return after
// its context is cancelled. A wedged manager must not prevent envtest from
// being stopped.
const managerStopTimeout = 30 * time.Second

// setupEnv starts an envtest apiserver+etcd, installs the required CRDs,
// wires up the WaypointController, and starts the manager in a goroutine.
//
// Cleanups are registered with t.Cleanup immediately after the resources
// they refer to come into existence. Because t.Cleanup runs callbacks in
// LIFO order, this guarantees:
//   - the manager is stopped before envtest, and
//   - any t.Fatalf later in this function still triggers the cleanups
//     that have already been registered.
func setupEnv(t *testing.T) *testEnv {
	t.Helper()

	// modules/110-istio/crds holds the WaypointInstance CRD; testdata/crds
	// holds permissive Gateway and VPA stand-ins. Locate them relative to this
	// file so `go test` works from any cwd.
	_, thisFile, _, _ := runtime.Caller(0)
	thisDir := filepath.Dir(thisFile)
	repoCRDDir := filepath.Join(thisDir, "..", "..", "..", "..", "..", "crds")
	testCRDDir := filepath.Join(thisDir, "testdata", "crds")

	env := &envtest.Environment{
		CRDDirectoryPaths: []string{
			repoCRDDir,
			testCRDDir,
		},
		ErrorIfCRDPathMissing: true,
	}

	// Register the env BEFORE Start. envtest spawns subprocesses partway
	// through Start; if a SIGINT arrives during that window, the TestMain
	// handler still needs to know about this env to stop it.
	registerEnv(env)

	cfg, err := env.Start()
	if err != nil {
		// Start may have spawned children before erroring out.
		_ = env.Stop()
		unregisterEnv(env)
		t.Fatalf("start envtest: %v", err)
	}
	t.Cleanup(func() {
		if err := env.Stop(); err != nil {
			t.Logf("envtest stop: %v", err)
		}
		unregisterEnv(env)
	})

	mgrScheme := clientgoscheme.Scheme
	utilruntime.Must(policyv1.AddToScheme(mgrScheme))
	utilruntime.Must(autoscalingv2.AddToScheme(mgrScheme))
	utilruntime.Must(vpav1.AddToScheme(mgrScheme))
	utilruntime.Must(gatewayv1.Install(mgrScheme))
	utilruntime.Must(networkv1alpha1.AddToScheme(mgrScheme))

	managedResourcesCache := cache.ByObject{
		Namespaces: map[string]cache.Config{
			cache.AllNamespaces: {},
		},
		Label: labels.SelectorFromSet(map[string]string{
			AppLabelKey: AppLabelValue,
		}),
	}

	mgr, err := ctrl.NewManager(cfg, manager.Options{
		Scheme: mgrScheme,
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&networkv1alpha1.WaypointInstance{}: {
					Namespaces: map[string]cache.Config{
						cache.AllNamespaces: {},
					},
				},
				&appsv1.Deployment{}:                     managedResourcesCache,
				&corev1.Service{}:                        managedResourcesCache,
				&corev1.ServiceAccount{}:                 managedResourcesCache,
				&policyv1.PodDisruptionBudget{}:          managedResourcesCache,
				&autoscalingv2.HorizontalPodAutoscaler{}: managedResourcesCache,
				&vpav1.VerticalPodAutoscaler{}:           managedResourcesCache,
				&gatewayv1.Gateway{}:                     managedResourcesCache,
			},
		},
	})
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}

	wc := &WaypointController{
		VPAEnabled: true,
	}

	if err := wc.SetupWithManager(mgr); err != nil {
		t.Fatalf("setup controller: %v", err)
	}

	// SetupWithManager reads env vars for these fields. Override with
	// deterministic test values so assertions can pin against them.
	wc.proxyImage = "registry.example.com/istio/proxyv2:test"
	wc.clusterDomain = "cluster.local"
	wc.istioRevision = "v1x25x2"
	wc.istioNetworkName = "test-network"
	wc.istioCloudPlatform = "none"
	wc.istioClusterID = "test-cluster"

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- mgr.Start(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Logf("manager exited with error: %v", err)
			}
		case <-time.After(managerStopTimeout):
			t.Logf("manager did not exit within %s; proceeding with envtest stop", managerStopTimeout)
		}
	})

	if !mgr.GetCache().WaitForCacheSync(ctx) {
		t.Fatalf("cache failed to sync")
	}

	return &testEnv{
		env:    env,
		mgr:    mgr,
		client: mgr.GetClient(),
	}
}

// TestWaypointInstanceLifecycle exercises the full happy path:
// create an WaypointInstance with replicas=2 and VPA mode (so every
// optional child kind is exercised), verify each managed child is reconciled
// with the expected labels and owner reference, then delete the instance
// and verify the finalizer cleans every child up.
func TestWaypointInstanceLifecycle(t *testing.T) {
	te := setupEnv(t)
	ctx := context.Background()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "test-app"},
	}
	if err := te.client.Create(ctx, ns); err != nil {
		t.Fatalf("create namespace: %v", err)
	}

	replicas := int32(2)
	instance := &networkv1alpha1.WaypointInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "main",
			Namespace: ns.Name,
		},
		Spec: networkv1alpha1.WaypointInstanceSpec{
			WaypointFor: "All",
			ReplicasManagement: &networkv1alpha1.ReplicasManagement{
				Mode: "Static",
				Static: &networkv1alpha1.ReplicasStatic{
					Replicas: replicas,
				},
			},
			ResourcesManagement: &networkv1alpha1.ResourcesManagement{
				Mode: "VPA",
				VPA: &networkv1alpha1.ResourcesVPA{
					Mode: "Initial",
				},
			},
		},
	}
	if err := te.client.Create(ctx, instance); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	baseName := resourceBaseName(instance.Name)
	key := func(name string) types.NamespacedName {
		return types.NamespacedName{Namespace: ns.Name, Name: name}
	}

	// Use apiReader (uncached) to read child resources. The manager's cache
	// is label-filtered to app=d8-waypoint and would also work, but
	// bypassing it removes a potential cache-staleness flake source.
	reader := te.mgr.GetAPIReader()

	// Children that must exist. HPA is intentionally absent: replicasManagement
	// mode is Static.
	checks := []struct {
		name string
		key  types.NamespacedName
		obj  client.Object
	}{
		{"ServiceAccount", key(baseName), &corev1.ServiceAccount{}},
		{"Deployment", key(baseName), &appsv1.Deployment{}},
		{"Service", key(baseName), &corev1.Service{}},
		{"Gateway", key(baseName), &gatewayv1.Gateway{}},
		{"VPA", key(baseName), &vpav1.VerticalPodAutoscaler{}},
		{"PDB", key(baseName), &policyv1.PodDisruptionBudget{}},
	}

	for _, c := range checks {
		c := c
		t.Run("create/"+c.name, func(t *testing.T) {
			if err := waitForObject(ctx, reader, c.key, c.obj); err != nil {
				t.Fatalf("%s not created: %v", c.name, err)
			}
			assertOwnedBy(t, c.obj, instance)
			assertHasLabel(t, c.obj, AppLabelKey, AppLabelValue)
			assertHasLabel(t, c.obj, WaypointInstanceLabelKey, instance.Name)
			assertHasLabel(t, c.obj, HeritageLabelKey, HeritageLabelValue)
		})
	}

	t.Run("create/HPA-absent", func(t *testing.T) {
		hpa := &autoscalingv2.HorizontalPodAutoscaler{}
		err := reader.Get(ctx, key(baseName), hpa)
		if !apierrors.IsNotFound(err) {
			t.Fatalf("HPA should not exist in Static mode, got err=%v", err)
		}
	})

	t.Run("create/Deployment-spec", func(t *testing.T) {
		dep := &appsv1.Deployment{}
		if err := reader.Get(ctx, key(baseName), dep); err != nil {
			t.Fatalf("get deployment: %v", err)
		}
		if dep.Spec.Replicas == nil || *dep.Spec.Replicas != replicas {
			got := "<nil>"
			if dep.Spec.Replicas != nil {
				got = fmt.Sprintf("%d", *dep.Spec.Replicas)
			}
			t.Errorf("deployment replicas = %s, want %d", got, replicas)
		}
		if len(dep.Spec.Template.Spec.Containers) != 1 {
			t.Fatalf("expected exactly one container, got %d", len(dep.Spec.Template.Spec.Containers))
		}
		c := dep.Spec.Template.Spec.Containers[0]
		if c.Name != "istio-proxy" {
			t.Errorf("container name = %q, want istio-proxy", c.Name)
		}
		if c.Image != "registry.example.com/istio/proxyv2:test" {
			t.Errorf("container image = %q, want test image", c.Image)
		}
		// Anti-affinity is added only when effective minimum replicas >= 2.
		if dep.Spec.Template.Spec.Affinity == nil || dep.Spec.Template.Spec.Affinity.PodAntiAffinity == nil {
			t.Errorf("expected pod anti-affinity for replicas >= 2")
		}
	})

	t.Run("create/Service-ports", func(t *testing.T) {
		svc := &corev1.Service{}
		if err := reader.Get(ctx, key(baseName), svc); err != nil {
			t.Fatalf("get service: %v", err)
		}
		if svc.Spec.Type != corev1.ServiceTypeClusterIP {
			t.Errorf("service type = %s, want ClusterIP", svc.Spec.Type)
		}
		ports := portsByName(svc.Spec.Ports)
		if ports["mesh"] != 15008 {
			t.Errorf("mesh port = %d, want 15008", ports["mesh"])
		}
		if ports["status-port"] != 15021 {
			t.Errorf("status-port = %d, want 15021", ports["status-port"])
		}
	})

	// envtest runs only apiserver+etcd — no kube-controller-manager, so there
	// is no built-in owner-reference garbage collector. Children are removed
	// here via the controller's own finalizer (handleFinalizers ->
	// pruneOwnerReference), which is the same code path that runs in
	// production. This is therefore the more meaningful path to verify.
	t.Run("delete/cascade", func(t *testing.T) {
		if err := te.client.Delete(ctx, instance); err != nil {
			t.Fatalf("delete instance: %v", err)
		}

		for _, c := range checks {
			c := c
			obj := c.obj.DeepCopyObject().(client.Object)
			if err := waitForObjectGone(ctx, reader, c.key, obj); err != nil {
				t.Errorf("%s not cleaned up: %v", c.name, err)
			}
		}

		// The instance itself should be gone once the finalizer is removed.
		got := &networkv1alpha1.WaypointInstance{}
		if err := waitForObjectGone(ctx, reader, types.NamespacedName{Namespace: ns.Name, Name: instance.Name}, got); err != nil {
			t.Errorf("instance not cleaned up: %v", err)
		}
	})
}

// waitForObject polls until the object exists or pollTimeout elapses. The
// fetched object is stored into obj so callers can assert against it.
func waitForObject(ctx context.Context, c client.Reader, key types.NamespacedName, obj client.Object) error {
	return wait.PollUntilContextTimeout(ctx, pollInterval, pollTimeout, true, func(ctx context.Context) (bool, error) {
		err := c.Get(ctx, key, obj)
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return true, nil
	})
}

// waitForObjectGone polls until the object is absent (NotFound) or
// pollTimeout elapses.
func waitForObjectGone(ctx context.Context, c client.Reader, key types.NamespacedName, obj client.Object) error {
	return wait.PollUntilContextTimeout(ctx, pollInterval, pollTimeout, true, func(ctx context.Context) (bool, error) {
		err := c.Get(ctx, key, obj)
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	})
}

func assertOwnedBy(t *testing.T, obj client.Object, owner *networkv1alpha1.WaypointInstance) {
	t.Helper()
	for _, ref := range obj.GetOwnerReferences() {
		if ref.UID == owner.UID {
			if ref.Controller == nil || !*ref.Controller {
				t.Errorf("%s/%s owner ref to instance is not marked as controller",
					obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
			}
			return
		}
	}
	t.Errorf("%s/%s missing owner reference to instance %s (UID=%s)",
		gvkString(obj), obj.GetName(), owner.Name, owner.UID)
}

func assertHasLabel(t *testing.T, obj client.Object, key, value string) {
	t.Helper()
	got, ok := obj.GetLabels()[key]
	if !ok {
		t.Errorf("%s/%s missing label %q", gvkString(obj), obj.GetName(), key)
		return
	}
	if got != value {
		t.Errorf("%s/%s label %q = %q, want %q", gvkString(obj), obj.GetName(), key, got, value)
	}
}

// gvkString returns a human-readable identifier for obj. Typed clients
// usually leave TypeMeta empty, so we fall back to the Go type name.
func gvkString(obj client.Object) string {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Kind == "" {
		return fmt.Sprintf("%T", obj)
	}
	return schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind}.String()
}

func portsByName(ports []corev1.ServicePort) map[string]int32 {
	m := make(map[string]int32, len(ports))
	for _, p := range ports {
		m[p.Name] = p.Port
	}
	return m
}
