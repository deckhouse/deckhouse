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

package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/common"
	"fencing-controller/internal/config"
	"fencing-controller/internal/testenv"
)

// TestLeaderElectionEnvtest runs the manager of the binary against a real API
// server. Another replica holds the Lease of the leader, so this replica does
// not decide on the incident. When the other replica lets the Lease go, this
// replica becomes the leader and decides. The node of the incident is off, so
// the new leader sets the taint after the silent delay from its own first
// observation. The replica that is not the leader serves the probes and the
// metrics on one local address.
func TestLeaderElectionEnvtest(t *testing.T) {
	env := testenv.Start(t)
	api := env.Client
	ctx := t.Context()

	user, err := env.AddUser(envtest.User{Name: "fencing-controller", Groups: []string{"system:masters"}}, nil)
	if err != nil {
		t.Fatalf("add user: %v", err)
	}

	kubeconfig, err := user.KubeConfig()
	if err != nil {
		t.Fatalf("make kubeconfig: %v", err)
	}

	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(path, kubeconfig, 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	t.Setenv("KUBECONFIG", path)

	const namespace = "d8-cloud-instance-manager"

	holder := "other-replica"
	duration := int32(time.Hour.Seconds())
	renewed := metav1.NewMicroTime(time.Now())
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: common.LeaderElectionID},
		Spec:       coordinationv1.LeaseSpec{HolderIdentity: &holder, LeaseDurationSeconds: &duration, RenewTime: &renewed},
	}

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-3"}}

	for _, obj := range []client.Object{
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}},
		testenv.Manifests(t, "templates/fencing-agent/sla-profiles/critical.yaml")[0],
		lease,
		node,
	} {
		if err := api.Create(ctx, obj); err != nil {
			t.Fatalf("create %T %s: %v", obj, obj.GetName(), err)
		}
	}

	incident := &v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{
			Name: node.Name,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1", Kind: "Node", Name: node.Name, UID: node.UID,
			}},
		},
		Spec: v1alpha1.FencingFailedNodeStateSpec{
			Source:     v1alpha1.SourceMemberlistDead,
			DetectedBy: "worker-1",
			DetectedAt: metav1.NewMicroTime(time.Now()),
			BootID:     "0f1e2d3c-1111-2222-3333-444455556666",
			Armed:      true,
			ProfileRef: &v1alpha1.ProfileRef{Name: v1alpha1.ProfileCritical},
		},
	}

	// The agent of the node wrote that the watchdog is armed. Then the node
	// stopped: it writes no heartbeat, and no witness attests it.
	node.Status.Conditions = []corev1.NodeCondition{{Type: "Fencing", Status: corev1.ConditionTrue, Reason: "Armed"}}
	if err := api.Status().Update(ctx, node); err != nil {
		t.Fatalf("write status of the node: %v", err)
	}

	if err := api.Create(ctx, incident); err != nil {
		t.Fatalf("create fencingfailednodestate: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find a free port: %v", err)
	}

	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("free the port: %v", err)
	}

	managerCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})

	// The manager can run only once in a process, because the name of each
	// controller must be unique.
	var managerErr error

	go func() {
		defer close(done)

		managerErr = runManager(managerCtx, &config.Config{
			MetricsBindAddress: address,
			LeaderElection:     true,
			Namespace:          namespace,
			LogLevel:           config.DefaultLogLevel,
		}, log.NewLogger(log.WithOutput(io.Discard)))
	}()

	t.Cleanup(func() {
		stop()
		<-done

		if managerErr != nil {
			t.Errorf("manager stopped with an error: %v", managerErr)
		}
	})

	// phase returns the phase of the incident. It stops the test when the
	// manager stopped.
	phase := func() v1alpha1.FencingFailedNodeStatePhase {
		select {
		case <-done:
			t.Fatalf("manager stopped: %v", managerErr)
		default:
		}

		var got v1alpha1.FencingFailedNodeState
		if err := api.Get(ctx, types.NamespacedName{Name: incident.Name}, &got); err != nil {
			t.Fatalf("get fencingfailednodestate: %v", err)
		}

		return got.Status.Phase
	}

	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if got := phase(); got != "" {
			t.Fatalf("the replica that is not the leader wrote the phase %q", got)
		}
	}

	// status returns the HTTP status of a path on the address of the metrics, or 0 when the server does not answer.
	status := func(path string) int {
		resp, err := http.Get("http://" + address + path)
		if err != nil {
			return 0
		}
		defer resp.Body.Close()

		return resp.StatusCode
	}

	for _, path := range []string{"/healthz", "/readyz", "/metrics"} {
		for deadline := time.Now().Add(10 * time.Second); status(path) != http.StatusOK; time.Sleep(100 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("%s on %s does not answer %d", path, address, http.StatusOK)
			}
		}
	}

	takeover := time.Now()

	if err := api.Delete(ctx, lease); err != nil {
		t.Fatalf("delete lease of the other replica: %v", err)
	}

	for deadline := time.Now().Add(30 * time.Second); phase() != v1alpha1.PhaseSuspected; time.Sleep(100 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the new leader did not write the phase %q", v1alpha1.PhaseSuspected)
		}
	}

	if err := api.Get(ctx, client.ObjectKeyFromObject(lease), lease); err != nil {
		t.Fatalf("get lease: %v", err)
	}

	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity == holder {
		t.Errorf("lease holder is %v, want this replica", lease.Spec.HolderIdentity)
	}

	// The new leader sees the object first after the takeover. It sets the
	// taint not earlier than the silent delay of the critical profile after
	// that: 4.15s + 0.2s + 5s + 0.3s + 0.9s.
	const silent = 10550 * time.Millisecond

	tainted := func() bool {
		var got corev1.Node
		if err := api.Get(ctx, client.ObjectKeyFromObject(node), &got); err != nil {
			t.Fatalf("get node: %v", err)
		}

		for _, taint := range got.Spec.Taints {
			if taint.Key == corev1.TaintNodeOutOfService {
				return true
			}
		}

		return false
	}

	for deadline := time.Now().Add(silent + 10*time.Second); !tainted(); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the new leader did not set the taint in %s after the takeover", silent+10*time.Second)
		}
	}

	if elapsed := time.Since(takeover); elapsed < silent {
		t.Errorf("the new leader set the taint %s after the takeover, want at least %s", elapsed, silent)
	}
}
