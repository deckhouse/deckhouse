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

// Package testenv holds controller-agnostic helpers for envtest-based integration suites.
// Any node-controller controller can use it to bootstrap a real apiserver, wire its
// registered controllers into a manager, and get the common test plumbing (unique names,
// finalizer stripping, kubectl-style dumps, a kubectl pause hook). It is intentionally
// free of Ginkgo/Gomega so it can back any test runner.
package testenv

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/deckhouse/node-controller/internal/register"
)

// Common timeouts for asynchronous (Eventually/Consistently) assertions.
const (
	EventuallyTimeout     = 20 * time.Second
	EventuallyPoll        = 200 * time.Millisecond
	NegativeCheckDuration = 2 * time.Second
)

// KubeconfigPath is where PauseForKubectl writes the envtest kubeconfig.
const KubeconfigPath = "/tmp/envtest.kubeconfig"

// BinaryAssetsDir returns the kubebuilder assets directory: KUBEBUILDER_ASSETS if set,
// otherwise the first versioned directory under the nearest bin/k8s (populated by
// `make envtest`), discovered by walking up from the test working directory.
func BinaryAssetsDir() string {
	if v := os.Getenv("KUBEBUILDER_ASSETS"); v != "" {
		return v
	}
	binDir := findDirUp("bin")
	if binDir == "" {
		return ""
	}
	entries, err := os.ReadDir(filepath.Join(binDir, "k8s"))
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			return filepath.Join(binDir, "k8s", e.Name())
		}
	}
	return ""
}

// AssetsAvailable reports whether the envtest binaries are present, so a suite can Skip
// gracefully instead of failing when run without `make envtest`.
func AssetsAvailable() bool {
	return BinaryAssetsDir() != ""
}

// CRDPaths resolves the given CRD file names against the module's crds/ directory (found by
// walking up from the test working directory), e.g. CRDPaths("instance.yaml", "machine.yaml").
func CRDPaths(files ...string) []string {
	crds := findDirUp("crds")
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, filepath.Join(crds, f))
	}
	return paths
}

func findDirUp(name string) string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(dir, name)
		if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// Start boots an envtest apiserver with the given scheme and CRD files and returns a client.
// Call testEnv.Stop() in AfterSuite.
func Start(scheme *runtime.Scheme, crdPaths []string) (*envtest.Environment, *rest.Config, client.Client, error) {
	env := &envtest.Environment{
		CRDDirectoryPaths:     crdPaths,
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: BinaryAssetsDir(),
	}
	cfg, err := env.Start()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("start envtest: %w", err)
	}
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return env, cfg, nil, fmt.Errorf("build client: %w", err)
	}
	return env, cfg, c, nil
}

// NewManager builds a manager (metrics/leader-election off) and wires every controller that
// registered itself via register.RegisterController in the test binary. Import only the
// controller package under test so only it gets wired. Start it with `go mgr.Start(ctx)`.
func NewManager(cfg *rest.Config, scheme *runtime.Scheme) (manager.Manager, error) {
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:         scheme,
		Metrics:        metricsserver.Options{BindAddress: "0"},
		LeaderElection: false,
	})
	if err != nil {
		return nil, fmt.Errorf("new manager: %w", err)
	}
	if err := register.SetupAll(mgr, mgr.GetClient(), "", 1, nil); err != nil {
		return nil, fmt.Errorf("setup controllers: %w", err)
	}
	return mgr, nil
}

// SetupLogger silences controller-runtime logs by default so spec output stays readable;
// ENVTEST_LOGS=1 turns them on, written to w (e.g. GinkgoWriter).
func SetupLogger(w io.Writer) {
	if os.Getenv("ENVTEST_LOGS") != "" {
		logf.SetLogger(zap.New(zap.WriteTo(w), zap.UseDevMode(true)))
	} else {
		logf.SetLogger(logr.Discard())
	}
}

// DebugEnabled reports whether ENVTEST_DEBUG is set (used to gate before/after state dumps).
func DebugEnabled() bool {
	return os.Getenv("ENVTEST_DEBUG") != ""
}

var nameCounter atomic.Int64

// UniqueName yields a unique, DNS-safe, lowercase name per call so specs do not collide on
// cluster-scoped objects.
func UniqueName(base string) string {
	return fmt.Sprintf("e2e-%s-%d", base, nameCounter.Add(1))
}

// RemoveFinalizers strips finalizers, re-getting and retrying on conflict so a concurrent
// controller mutation during teardown does not leave the object stuck.
func RemoveFinalizers(ctx context.Context, c client.Client, obj client.Object) {
	for range 5 {
		if len(obj.GetFinalizers()) == 0 {
			return
		}
		patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
		obj.SetFinalizers(nil)
		err := c.Patch(ctx, obj, patch)
		if err == nil || apierrors.IsNotFound(err) {
			return
		}
		if !apierrors.IsConflict(err) {
			return
		}
		_ = c.Get(ctx, client.ObjectKeyFromObject(obj), obj)
	}
}

// KubectlDumpNodeObjects runs the envtest-bundled kubectl against the running apiserver and writes
// real `kubectl get … -o wide` output for the node-controller objects to w (Nodes plus the
// NodeGroup/Instance/Machine CRs). Resource types whose CRD is not installed in the current suite
// are skipped silently, so it is safe to call from any controller suite. header (e.g. the spec name)
// labels the dump. Callers gate it on DebugEnabled(). kubectl is the binary that ships in the
// kubebuilder assets, so no kubectl on PATH is required.
func KubectlDumpNodeObjects(w io.Writer, env *envtest.Environment, cfg *rest.Config, header string) {
	if err := WriteKubeconfig(env, cfg, KubeconfigPath); err != nil {
		fmt.Fprintf(w, "kubectl dump: %v\n", err)
		return
	}
	kubectl := filepath.Join(BinaryAssetsDir(), "kubectl")
	fmt.Fprintf(w, "\n===== state after: %s =====\n", header)
	kubectlGet(w, kubectl, "nodes")
	kubectlGet(w, kubectl, "nodegroups.deckhouse.io")
	kubectlGet(w, kubectl, "instances.deckhouse.io")
	kubectlGet(w, kubectl, "machines.cluster.x-k8s.io", "-A")
}

func kubectlGet(w io.Writer, kubectl, resource string, extra ...string) {
	args := append([]string{"--kubeconfig", KubeconfigPath, "get", resource, "-o", "wide"}, extra...)
	out, _ := exec.CommandContext(context.Background(), kubectl, args...).CombinedOutput()
	s := strings.TrimRight(string(out), "\n")
	if strings.Contains(s, "doesn't have a resource type") || strings.Contains(s, "could not find the requested resource") {
		return // CRD not installed in this suite — skip
	}
	fmt.Fprintf(w, "$ kubectl get %s -o wide\n%s\n", resource, s)
}

// WriteKubeconfig writes a system:masters kubeconfig for the running envtest apiserver.
func WriteKubeconfig(env *envtest.Environment, cfg *rest.Config, path string) error {
	au, err := env.AddUser(envtest.User{Name: "envtest-admin", Groups: []string{"system:masters"}}, cfg)
	if err != nil {
		return fmt.Errorf("add envtest user: %w", err)
	}
	kc, err := au.KubeConfig()
	if err != nil {
		return fmt.Errorf("render kubeconfig: %w", err)
	}
	return os.WriteFile(path, kc, 0o600)
}

// PauseForKubectl writes a kubeconfig and blocks for d so a paused spec can be inspected with
// real kubectl (the apiserver only runs during the suite):
//
//	KUBECONFIG=/tmp/envtest.kubeconfig kubectl get -A -o wide
func PauseForKubectl(w io.Writer, env *envtest.Environment, cfg *rest.Config, d time.Duration) {
	if err := WriteKubeconfig(env, cfg, KubeconfigPath); err != nil {
		fmt.Fprintf(w, "pauseForKubectl: %v\n", err)
		return
	}
	fmt.Fprintf(w, "\n>>> envtest paused for %s — run:\n    KUBECONFIG=%s kubectl get -A -o wide\n\n", d, KubeconfigPath)
	time.Sleep(d)
}
