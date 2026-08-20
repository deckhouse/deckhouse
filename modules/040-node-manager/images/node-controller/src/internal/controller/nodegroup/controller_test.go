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

package nodegroup

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/cloudprovider"
	"github.com/deckhouse/node-controller/internal/register"
)

func newReconciler(t *testing.T, objs ...runtime.Object) (*Status, *record.FakeRecorder) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := v1.AddToScheme(scheme); err != nil {
		t.Fatalf("add v1 scheme: %v", err)
	}
	// derived_status reads the target Kubernetes version from d8-cluster-kubernetes only; without
	// it the version degrades to the running kube-apiserver, which these fixtures do not have.
	// Tests that assert a specific version pass their own ConfigMap; everyone else gets a default.
	if !hasClusterKubernetesConfigMap(objs) {
		objs = append([]runtime.Object{clusterKubernetesConfigMap("1.32")}, objs...)
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		WithStatusSubresource(&v1.NodeGroup{}).
		Build()
	rec := record.NewFakeRecorder(10)
	return &Status{
		Base: register.Base{Client: cl, Recorder: rec},
	}, rec
}

func clusterKubernetesConfigMap(desiredVersion string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-cluster-kubernetes", Namespace: "kube-system"},
		Data: map[string]string{
			"spec": "desiredVersion: \"" + desiredVersion + "\"\nupdateMode: Manual\n",
		},
	}
}

func hasClusterKubernetesConfigMap(objs []runtime.Object) bool {
	for _, obj := range objs {
		cm, ok := obj.(*corev1.ConfigMap)
		if ok && cm.Name == "d8-cluster-kubernetes" && cm.Namespace == "kube-system" {
			return true
		}
	}
	return false
}

func doReconcile(t *testing.T, r *Status, name string) ctrl.Result {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	if err != nil {
		t.Fatalf("reconcile %s: %v", name, err)
	}
	return res
}

func getNodeGroup(t *testing.T, r *Status, name string) *v1.NodeGroup {
	t.Helper()
	ng := &v1.NodeGroup{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: name}, ng); err != nil {
		t.Fatalf("get nodegroup %s: %v", name, err)
	}
	return ng
}

func makeNode(name, ngName string, ready bool) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"node.deckhouse.io/group": ngName},
		},
	}
	if ready {
		node.Status.Conditions = []corev1.NodeCondition{
			{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
		}
	} else {
		node.Status.Conditions = []corev1.NodeCondition{
			{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
		}
	}
	return node
}

func makeChecksumSecret(data map[string]string) *corev1.Secret {
	secretData := make(map[string][]byte, len(data))
	for k, v := range data {
		secretData[k] = []byte(v)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configuration-checksums",
			Namespace: "d8-cloud-instance-manager",
		},
		Data: secretData,
	}
}

func setEnv(t *testing.T) {
	t.Helper()
	t.Setenv("TEST_CONDITIONS_CALC_NOW_TIME", "2021-01-01T13:30:00Z")
	t.Setenv("TEST_CONDITIONS_CALC_CHKSUM", "testchecksum")
}

func TestReconcile_NodeGroupNotFound_NoError(t *testing.T) {
	setEnv(t)
	r, _ := newReconciler(t)
	doReconcile(t, r, "nonexistent")
}

func TestReconcile_StaticNG_NoNodes_StatusZeros(t *testing.T) {
	setEnv(t)
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	r, _ := newReconciler(t, ng)
	doReconcile(t, r, "worker")

	updated := getNodeGroup(t, r, "worker")
	if updated.Status.Nodes != 0 {
		t.Fatalf("expected Nodes=0, got %d", updated.Status.Nodes)
	}
	if updated.Status.Ready != 0 {
		t.Fatalf("expected Ready=0, got %d", updated.Status.Ready)
	}
	if updated.Status.UpToDate != 0 {
		t.Fatalf("expected UpToDate=0, got %d", updated.Status.UpToDate)
	}
	if updated.Status.Desired != 0 {
		t.Fatalf("expected Desired=0, got %d", updated.Status.Desired)
	}
	if updated.Status.ConditionSummary == nil {
		t.Fatal("expected ConditionSummary to be set")
	}
	if updated.Status.ConditionSummary.Ready != "True" {
		t.Fatalf("expected ConditionSummary.Ready=True, got %q", updated.Status.ConditionSummary.Ready)
	}
}

func TestReconcile_StaticNG_MixedReadiness(t *testing.T) {
	setEnv(t)
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	r, _ := newReconciler(t,
		ng,
		makeNode("n1", "worker", true),
		makeNode("n2", "worker", true),
		makeNode("n3", "worker", false),
	)
	doReconcile(t, r, "worker")

	updated := getNodeGroup(t, r, "worker")
	if updated.Status.Nodes != 3 {
		t.Fatalf("expected Nodes=3, got %d", updated.Status.Nodes)
	}
	if updated.Status.Ready != 2 {
		t.Fatalf("expected Ready=2, got %d", updated.Status.Ready)
	}
	if updated.Status.UpToDate != 0 {
		t.Fatalf("expected UpToDate=0, got %d", updated.Status.UpToDate)
	}
}

func TestReconcile_StaticNG_ChecksumMatch_UpToDate(t *testing.T) {
	setEnv(t)
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}
	n1 := makeNode("n1", "worker", true)
	n1.Annotations = map[string]string{"node.deckhouse.io/configuration-checksum": "abc123"}
	n2 := makeNode("n2", "worker", true)
	n2.Annotations = map[string]string{"node.deckhouse.io/configuration-checksum": "abc123"}
	n3 := makeNode("n3", "worker", true)
	n3.Annotations = map[string]string{"node.deckhouse.io/configuration-checksum": "old"}
	secret := makeChecksumSecret(map[string]string{"worker": "abc123"})

	r, _ := newReconciler(t, ng, n1, n2, n3, secret)
	doReconcile(t, r, "worker")

	updated := getNodeGroup(t, r, "worker")
	if updated.Status.Nodes != 3 {
		t.Fatalf("expected Nodes=3, got %d", updated.Status.Nodes)
	}
	if updated.Status.Ready != 3 {
		t.Fatalf("expected Ready=3, got %d", updated.Status.Ready)
	}
	if updated.Status.UpToDate != 2 {
		t.Fatalf("expected UpToDate=2, got %d", updated.Status.UpToDate)
	}
}

func TestReconcile_StaticNG_Idempotent(t *testing.T) {
	setEnv(t)
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	r, _ := newReconciler(t, ng, makeNode("n1", "worker", true))
	doReconcile(t, r, "worker")
	first := getNodeGroup(t, r, "worker")

	doReconcile(t, r, "worker")
	second := getNodeGroup(t, r, "worker")

	if first.Status.Nodes != second.Status.Nodes {
		t.Fatalf("Nodes changed: %d -> %d", first.Status.Nodes, second.Status.Nodes)
	}
	if first.Status.Ready != second.Status.Ready {
		t.Fatalf("Ready changed: %d -> %d", first.Status.Ready, second.Status.Ready)
	}
	if first.Status.UpToDate != second.Status.UpToDate {
		t.Fatalf("UpToDate changed: %d -> %d", first.Status.UpToDate, second.Status.UpToDate)
	}
}

func TestReconcile_StaticNG_ExistingErrorCleared(t *testing.T) {
	setEnv(t)
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	r, rec := newReconciler(t, ng)

	// Set initial Status.Error via status subresource
	ngWithError := ng.DeepCopy()
	ngWithError.Status.Error = "some error from webhook"
	if err := r.Client.Status().Update(context.Background(), ngWithError); err != nil {
		t.Fatalf("set initial status: %v", err)
	}

	doReconcile(t, r, "worker")

	updated := getNodeGroup(t, r, "worker")
	if updated.Status.Error != "" {
		t.Fatalf("expected stale Status.Error to be cleared, got %q", updated.Status.Error)
	}
	if updated.Status.ConditionSummary == nil {
		t.Fatal("expected ConditionSummary to be set")
	}
	if updated.Status.ConditionSummary.Ready != "True" {
		t.Fatalf("expected ConditionSummary.Ready=True, got %q", updated.Status.ConditionSummary.Ready)
	}

	select {
	case event := <-rec.Events:
		t.Fatalf("expected no event for cleared stale error, got %q", event)
	default:
	}
}

func TestReconcile_StatusKubernetesVersionUpdated(t *testing.T) {
	setEnv(t)
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	r, _ := newReconciler(t, ng, clusterKubernetesConfigMap("1.32"))
	doReconcile(t, r, "worker")

	updated := getNodeGroup(t, r, "worker")
	if updated.Status.KubernetesVersion != "1.32" {
		t.Fatalf("expected KubernetesVersion=1.32, got %q", updated.Status.KubernetesVersion)
	}
}

func TestReconcile_CloudValidationErrorPublished(t *testing.T) {
	setEnv(t)
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "cloud"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				MinPerZone: 1,
				MaxPerZone: 3,
				ClassReference: v1.ClassReference{
					Kind: "WrongInstanceClass",
					Name: "system",
				},
			},
		},
	}
	cloudProvider := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cloudprovider.SecretNamePrefix,
			Namespace: cloudprovider.SecretNamespace,
			// Registrations are found by this label, never by name.
			Labels: map[string]string{cloudprovider.SecretLabel: ""},
		},
		Data: map[string][]byte{
			"type":                    []byte("aws"),
			"instanceClassKind":       []byte("AWSInstanceClass"),
			"instanceClassAPIVersion": []byte("v1alpha1"),
		},
	}
	// The cluster configuration is what hands a NodeGroup its provider, and the provider is what
	// names the only InstanceClass kind the group may reference.
	clusterConfig := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-cluster-configuration", Namespace: "kube-system"},
		Data: map[string][]byte{
			"cluster-configuration.yaml": []byte("clusterType: Cloud\ncloud:\n  provider: AWS\n"),
		},
	}

	r, rec := newReconciler(t, ng, cloudProvider, clusterConfig)
	doReconcile(t, r, "cloud")

	updated := getNodeGroup(t, r, "cloud")
	if !strings.Contains(updated.Status.Error, "Invalid classReference.kind") {
		t.Fatalf("expected validation error in status, got %q", updated.Status.Error)
	}
	if updated.Status.ConditionSummary == nil || updated.Status.ConditionSummary.Ready != "False" {
		t.Fatalf("expected Ready=False due to validation error, got %#v", updated.Status.ConditionSummary)
	}
	select {
	case event := <-rec.Events:
		if !strings.Contains(event, "Invalid classReference.kind") {
			t.Fatalf("expected validation event, got %q", event)
		}
	default:
		t.Fatal("expected validation event")
	}
}

// A NodeGroup whose provider cannot be resolved stops the pass before any status is computed:
// everything below the resolution depends on it. Which declarations hold is
// cloudprovider.TestForNodeGroup.
func TestReconcile_UnresolvedProviderFailsTheReconcile(t *testing.T) {
	registration := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cloudprovider.SecretNamePrefix,
			Namespace: cloudprovider.SecretNamespace,
			Labels:    map[string]string{cloudprovider.SecretLabel: ""},
		},
		Data: map[string][]byte{"type": []byte("yandex")},
	}
	cloudCluster := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-cluster-configuration", Namespace: "kube-system"},
		Data: map[string][]byte{
			// ClusterConfiguration spells it Yandex, the registration yandex.
			"cluster-configuration.yaml": []byte("clusterType: Cloud\ncloud:\n  provider: Yandex\n"),
		},
	}

	for _, tc := range []struct {
		name     string
		nodeType v1.NodeType
		declared string
		wantErr  string
	}{
		{name: "the-resolved-provider", nodeType: v1.NodeTypeCloudStatic, declared: "Yandex"},
		{
			name: "another-provider", nodeType: v1.NodeTypeCloudStatic, declared: "aws",
			wantErr: "Invalid providerType 'aws'. Expected 'yandex'",
		},
		{
			// A Static group runs in no cloud, so naming one is wrong even when the cluster has it.
			name: "a-provider-on-static", nodeType: v1.NodeTypeStatic, declared: "yandex",
			wantErr: "The nodes of this group run in no cloud",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setEnv(t)
			ng := &v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: tc.name},
				Spec:       v1.NodeGroupSpec{NodeType: tc.nodeType, ProviderType: tc.declared},
			}

			r, _ := newReconciler(t, ng, registration.DeepCopy(), cloudCluster.DeepCopy())
			_, err := r.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{Name: tc.name},
			})

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("reconcile %s: %v", tc.name, err)
				}
				return
			}
			if err == nil {
				t.Fatal("an unresolved provider must fail the reconcile")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want it to contain %q", err, tc.wantErr)
			}
			// The pass stops before the rest of the status is computed, so status.error is the
			// only place the NodeGroup gets to say why.
			if got := getNodeGroup(t, r, tc.name).Status.Error; !strings.Contains(got, tc.wantErr) {
				t.Fatalf("status.error = %q, want it to contain %q", got, tc.wantErr)
			}
		})
	}
}
