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

package hooks

import (
	"context"
	"errors"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func TestDeleteLegacyBindings_RemovesPerCustomRoleBindingsOnly(t *testing.T) {
	crbGVR := ruleBindingResources[0]
	rbGVR := ruleBindingResources[1]
	c := newLegacyBindingsFakeClient(legacyFixture()...)

	deleted, err := deleteLegacyBindings(context.Background(), c, 4)
	if err != nil {
		t.Fatalf("deleteLegacyBindings: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("deleted = %d, want 3", deleted)
	}

	for _, name := range []string{legacyCRB, legacyKeptCRB} {
		if s := legacyBindingState(t, c, crbGVR, "", name); s != "absent" {
			t.Errorf("%s = %s, want absent", name, s)
		}
	}
	if s := legacyBindingState(t, c, rbGVR, legacyRBNs, legacyRB); s != "absent" {
		t.Errorf("%s = %s, want absent", legacyRB, s)
	}
	for _, name := range []string{aggregatedCRB, levelCRB, lookalikeCRB, foreignCRB, nonRuleCRB} {
		if s := legacyBindingState(t, c, crbGVR, "", name); s == "absent" {
			t.Errorf("%s must survive", name)
		}
	}
}

func TestDeleteLegacyBindings_NoopWithoutLegacyBindingsAndReportsErrors(t *testing.T) {
	clean := newLegacyBindingsFakeClient(legacyTestBinding("ClusterRoleBinding", "", aggregatedCRB, moduleLabels(), nil))
	if deleted, err := deleteLegacyBindings(context.Background(), clean, 4); err != nil || deleted != 0 {
		t.Fatalf("deleted = %d, err = %v, want 0 and no error", deleted, err)
	}

	failing := newLegacyBindingsFakeClient(legacyTestBinding("ClusterRoleBinding", "", legacyCRB, moduleLabels(), keepAnnotations()))
	failing.PrependReactor("delete", "clusterrolebindings", func(_ clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})
	if _, err := deleteLegacyBindings(context.Background(), failing, 4); err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("err = %v, want the delete error", err)
	}
}

var _ = Describe("User Authz hooks :: delete legacy custom-role bindings ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)

	var fakeClient *dynamicfake.FakeDynamicClient

	BeforeEach(func() {
		fakeClient = newLegacyBindingsFakeClient(
			legacyTestBinding("ClusterRoleBinding", "", legacyCRB, moduleLabels(), keepAnnotations()),
			legacyTestBinding("ClusterRoleBinding", "", aggregatedCRB, moduleLabels(), nil),
		)
		newModuleBindingsClient = func(dependency.Container) (dynamic.Interface, error) { return fakeClient, nil }

		f.BindingContexts.Set(f.GenerateAfterHelmContext())
		f.RunHook()
	})

	It("Removes the legacy binding after the release and keeps the aggregated one", func() {
		Expect(f).To(ExecuteSuccessfully())

		_, err := fakeClient.Resource(ruleBindingResources[0]).Get(context.Background(), legacyCRB, metav1.GetOptions{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		_, err = fakeClient.Resource(ruleBindingResources[0]).Get(context.Background(), aggregatedCRB, metav1.GetOptions{})
		Expect(err).ShouldNot(HaveOccurred())
	})
})
