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

package v1alpha1_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/testenv"
)

const (
	agentNamespace = "d8-cloud-instance-manager"
	// nodeNameKey is the claim of a token that is bound to a pod. The API
	// server puts it into the extra of the user.
	nodeNameKey = "authentication.kubernetes.io/node-name"
	approvalKey = "node-manager.deckhouse.io/fencing-approve-eviction"
)

// TestFencingFailedNodeStateAdmissionEnvtest sends the writes of the agents and
// of the controller to a real API server with the CRD and the admission policy
// of the module. The agent uses the token of its pod on the node x, as on a
// real node. The writes are dry runs, so each case starts from the same
// objects.
func TestFencingFailedNodeStateAdmissionEnvtest(t *testing.T) {
	env := testenv.Start(t)
	api := env.Client
	ctx := t.Context()

	for _, obj := range testenv.Manifests(t, "templates/fencing-agent/validation.yaml") {
		create(t, api, obj)
	}

	create(t, api,
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: agentNamespace}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: agentNamespace, Name: "fencing-agent"}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: agentNamespace, Name: "fencing-controller"}},
		// RBAC is not under test here: both service accounts can write the
		// objects, and only the admission policy limits the agent.
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "fencing-test"},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{v1alpha1.GroupVersion.Group},
				Resources: []string{"fencingfailednodestates", "fencingfailednodestates/status"},
				Verbs:     []string{"get", "update", "patch"},
			}},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "fencing-test"},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "fencing-test"},
			Subjects: []rbacv1.Subject{
				{Kind: rbacv1.ServiceAccountKind, Namespace: agentNamespace, Name: "fencing-agent"},
				{Kind: rbacv1.ServiceAccountKind, Namespace: agentNamespace, Name: "fencing-controller"},
			},
		},
	)

	for _, name := range []string{"x", "v", "w"} {
		create(t, api, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}}, failedState(name))
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: agentNamespace, Name: "fencing-agent-x"},
		Spec: corev1.PodSpec{
			NodeName:           "x",
			ServiceAccountName: "fencing-agent",
			Containers:         []corev1.Container{{Name: "agent", Image: "agent"}},
		},
	}
	create(t, api, pod)

	agent := clientFor(t, env, "fencing-agent", &authenticationv1.BoundObjectReference{
		Kind: "Pod", APIVersion: "v1", Name: pod.Name, UID: pod.UID,
	})
	noClaim := clientFor(t, env, "fencing-agent", nil)
	controller := clientFor(t, env, "fencing-controller", nil)

	t.Run("token of the pod of the agent has the node name of the pod", func(t *testing.T) {
		review := &authenticationv1.SelfSubjectReview{}
		if err := agent.Create(ctx, review); err != nil {
			t.Fatalf("create selfsubjectreview: %v", err)
		}

		if got := review.Status.UserInfo.Extra[nodeNameKey]; !slices.Equal(got, authenticationv1.ExtraValue{"x"}) {
			t.Errorf("extra %s of the agent is %v, want [x]", nodeNameKey, got)
		}
	})

	heartbeat := func(s *v1alpha1.FencingFailedNodeState) {
		s.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: microTime()}
	}

	// The policy starts to work some time after its creation.
	for deadline := time.Now().Add(30 * time.Second); write(t, noClaim, "x", true, heartbeat) == nil; time.Sleep(100 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("admission policy does not deny the writes of a token without the claim")
		}
	}

	t.Run("change of the spec is refused", func(t *testing.T) {
		err := write(t, api, "v", false, func(s *v1alpha1.FencingFailedNodeState) { s.Spec.DetectedBy = "x" })
		if !apierrors.IsInvalid(err) || !strings.Contains(err.Error(), "spec is immutable") {
			t.Errorf("change of the spec returned %v, want the refusal of the CRD", err)
		}
	})

	for _, tc := range []struct {
		name   string
		client client.Client
		object string
		status bool
		change func(*v1alpha1.FencingFailedNodeState)
		// denial is a part of the message of the policy. Empty means that
		// the API server accepts the write.
		denial string
	}{
		{
			name: "agent of x writes the heartbeat of v", client: agent, object: "v", status: true, change: heartbeat,
			denial: "Only the agent of the failed node may change its heartbeat",
		},
		{name: "agent of x writes its own heartbeat", client: agent, object: "x", status: true, change: heartbeat},
		{
			name: "agent of x attests v with the witness w", client: agent, object: "v", status: true, change: attest("w"),
			denial: "only with its own node as the witness",
		},
		{name: "agent of x attests v with the witness x", client: agent, object: "v", status: true, change: attest("x")},
		{
			name: "agent of x writes the contact of v reported by w", client: agent, object: "v", status: true, change: contact("w"),
			denial: "only with its own node as the reporter",
		},
		{name: "agent of x writes the contact of v reported by x", client: agent, object: "v", status: true, change: contact("x")},
		{
			name: "agent of x approves the eviction of v", client: agent, object: "v", change: approve,
			denial: "may not add, change or remove the approval",
		},
		{
			name: "token without the claim writes the heartbeat of x", client: noClaim, object: "x", status: true, change: heartbeat,
			denial: "only with the node name in its token",
		},
		{
			name: "controller writes the phase and the heartbeat of v", client: controller, object: "v", status: true,
			change: func(s *v1alpha1.FencingFailedNodeState) {
				s.Status.Phase = v1alpha1.PhaseSuspected
				heartbeat(s)
			},
		},
		{name: "controller approves the eviction of v", client: controller, object: "v", change: approve},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := write(t, tc.client, tc.object, tc.status, tc.change)

			switch {
			case tc.denial == "" && err != nil:
				t.Errorf("write returned %v, want it accepted", err)
			case tc.denial != "" && (!apierrors.IsForbidden(err) || !strings.Contains(err.Error(), tc.denial)):
				t.Errorf("write returned %v, want the denial %q of the admission policy", err, tc.denial)
			}
		})
	}
}

// write reads the object as the administrator, changes it and sends the change
// as a dry run with the client. A status change goes to the status
// subresource.
func write(t *testing.T, c client.Client, name string, status bool, change func(*v1alpha1.FencingFailedNodeState)) error {
	t.Helper()

	state := &v1alpha1.FencingFailedNodeState{}
	if err := c.Get(t.Context(), types.NamespacedName{Name: name}, state); err != nil {
		t.Fatalf("get fencingfailednodestate %s: %v", name, err)
	}

	change(state)

	if status {
		return c.Status().Update(t.Context(), state, client.DryRunAll)
	}

	return c.Update(t.Context(), state, client.DryRunAll)
}

// clientFor returns a client with a token of the service account. The token is
// bound to the object, if it is set.
func clientFor(t *testing.T, env *testenv.Env, account string, bound *authenticationv1.BoundObjectReference) client.Client {
	t.Helper()

	request := &authenticationv1.TokenRequest{Spec: authenticationv1.TokenRequestSpec{BoundObjectRef: bound}}
	serviceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: agentNamespace, Name: account}}

	if err := env.Client.SubResource("token").Create(t.Context(), serviceAccount, request); err != nil {
		t.Fatalf("request token of %s: %v", account, err)
	}

	cfg := rest.AnonymousClientConfig(env.Config)
	cfg.BearerToken = request.Status.Token

	return testenv.NewClient(t, cfg)
}

func create(t *testing.T, api client.Client, objects ...client.Object) {
	t.Helper()

	for _, obj := range objects {
		if err := api.Create(t.Context(), obj); err != nil {
			t.Fatalf("create %T %s: %v", obj, obj.GetName(), err)
		}
	}
}

func failedState(node string) *v1alpha1.FencingFailedNodeState {
	return &v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: node},
		Spec: v1alpha1.FencingFailedNodeStateSpec{
			Source:     v1alpha1.SourceMemberlistDead,
			DetectedBy: "w",
			DetectedAt: *microTime(),
			BootID:     "0f1e2d3c-1111-2222-3333-444455556666",
			Armed:      true,
			ProfileRef: &v1alpha1.ProfileRef{Name: v1alpha1.ProfileCritical},
		},
	}
}

func attest(witness string) func(*v1alpha1.FencingFailedNodeState) {
	return func(s *v1alpha1.FencingFailedNodeState) {
		s.Status.Attestation = &v1alpha1.FencingFailedNodeStateAttestation{Witness: witness, At: *microTime()}
	}
}

func contact(reportedBy string) func(*v1alpha1.FencingFailedNodeState) {
	return func(s *v1alpha1.FencingFailedNodeState) {
		s.Status.Contact = &v1alpha1.FencingFailedNodeStateContact{ReportedBy: reportedBy, At: *microTime()}
	}
}

func approve(s *v1alpha1.FencingFailedNodeState) {
	s.Annotations = map[string]string{approvalKey: "true"}
}

func microTime() *metav1.MicroTime {
	now := metav1.NewMicroTime(time.Now())

	return &now
}
