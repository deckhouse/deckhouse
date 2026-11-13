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

package systemnetwork

import (
	"os"
	"strings"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/node-controller/internal/testenv"
)

const chartRBAC = "../../../../../../../templates/node-controller/rbac-for-us.yaml"

// chartRole reads the ClusterRole of node-controller from the template of the chart. kubectl auth can-i
// answers wrongly for a resource that does not exist, so the check asks the API with a SubjectAccessReview.
func chartRole(t *testing.T) *rbacv1.ClusterRole {
	t.Helper()
	text, err := os.ReadFile(chartRBAC)
	if err != nil {
		t.Fatal(err)
	}
	for _, document := range strings.Split(string(text), "\n---\n") {
		if !strings.Contains(document, "kind: ClusterRole\n") || !strings.Contains(document, "name: d8:node-manager:node-controller\n") {
			continue
		}
		var lines []string
		for _, line := range strings.Split(document, "\n") {
			if !strings.Contains(line, "{{") {
				lines = append(lines, line)
			}
		}
		role := &rbacv1.ClusterRole{}
		if err := yaml.Unmarshal([]byte(strings.Join(lines, "\n")), role); err != nil {
			t.Fatal(err)
		}
		return role
	}
	t.Fatal("no ClusterRole d8:node-manager:node-controller in the chart")
	return nil
}

func TestNodeControllerReadsSystemNetworks(t *testing.T) {
	if !testenv.AssetsAvailable() {
		t.Skip("envtest assets not found; run `make envtest` or set KUBEBUILDER_ASSETS")
	}
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	env, _, c, err := testenv.Start(scheme)
	if err != nil {
		t.Fatalf("start envtest: %v", err)
	}
	defer func() {
		if err := env.Stop(); err != nil {
			t.Errorf("stop envtest: %v", err)
		}
	}()

	role := chartRole(t)
	if err := c.Create(t.Context(), role); err != nil {
		t.Fatal(err)
	}
	binding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: role.Name},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: role.Name},
		Subjects:   []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: "node-controller", Namespace: "d8-cloud-instance-manager"}},
	}
	if err := c.Create(t.Context(), binding); err != nil {
		t.Fatal(err)
	}

	for verb, want := range map[string]bool{"get": true, "list": false, "watch": false, "delete": false} {
		t.Run(verb, func(t *testing.T) {
			review := &authorizationv1.SubjectAccessReview{Spec: authorizationv1.SubjectAccessReviewSpec{
				User: "system:serviceaccount:d8-cloud-instance-manager:node-controller",
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Verb: verb, Group: "network.deckhouse.io", Resource: "systemnetworks", Name: "sysnet-a",
				},
			}}
			if err := c.Create(t.Context(), review); err != nil {
				t.Fatal(err)
			}
			if review.Status.Allowed != want {
				t.Fatalf("allowed: %v, want %v (%s)", review.Status.Allowed, want, review.Status.Reason)
			}
		})
	}
}
