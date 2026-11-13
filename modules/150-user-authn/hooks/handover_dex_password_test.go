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
	"maps"
	"slices"
	"testing"

	"github.com/werf/nelm/v2/pkg/common"
	"github.com/werf/nelm/v2/pkg/resource"
	"github.com/werf/nelm/v2/pkg/resource/spec"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	metadatafake "k8s.io/client-go/metadata/fake"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
	"github.com/deckhouse/deckhouse/modules/150-user-authn/hooks/internal/handover"
)

// The hash Dex wrote when the user changed the password, base64 of the bcrypt hash as Dex stores it.
const changedPasswordHash = "JDJ5JDEwJGRNWGVGUVBkdUdYYVMyWDFPcGdZdk9HSy81LkdsNm5sdU9mUkhnNWlQdDhuSlh6SzhpeS5H"

// chartPassword is a Password as the chart of 1.76 rendered it from a User and Dex then updated.
// The hook reads and patches the metadata only, so the fields Dex wrote, the hash among them, are
// out of its reach; the fake client keeps the metadata alone.
func chartPassword(email string, annotations map[string]string) *metav1.PartialObjectMetadata {
	return handoverObject(passwordGVR, "Password", dexNamespace, encoding.ToFnvLikeDex(email),
		map[string]string{"heritage": "deckhouse", "module": "user-authn", "app": "dex"}, annotations)
}

// getPassword returns the Password as nelm reads it: the live metadata and the fields Dex wrote.
func getPassword(t *testing.T, c *metadatafake.FakeMetadataClient, email string) *unstructured.Unstructured {
	t.Helper()
	live, err := c.Resource(passwordGVR).Namespace(dexNamespace).Get(context.Background(), encoding.ToFnvLikeDex(email), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get Password of %s: %v", email, err)
	}
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(live)
	if err != nil {
		t.Fatalf("convert Password of %s: %v", email, err)
	}
	obj := &unstructured.Unstructured{Object: content}
	obj.SetGroupVersionKind(passwordGVR.GroupVersion().WithKind("Password"))
	obj.Object["email"] = email
	obj.Object["username"] = email
	obj.Object["hash"] = changedPasswordHash
	return obj
}

// prunedByNelm tells whether nelm, deploying a chart that no longer renders obj, deletes it: the
// policies come from the live object, the way nelm reads them before deleting a resource of the
// previous release.
func prunedByNelm(obj *unstructured.Unstructured) bool {
	meta := spec.NewResourceMetaFromUnstructured(obj, "d8-system", "")
	if err := resource.ValidateResourcePolicy(meta); err != nil {
		return false
	}
	return !slices.Contains(resource.ResourcePolicies(meta, "d8-system"), common.ResourcePolicySkipDelete)
}

// The update from 1.76 to 1.77 recreated every Password from User.spec.password: the chart of 1.77
// stopped rendering Passwords, the release pruned the ones it owned, and user-authn-controller
// created them anew from the password the User was created with.
func TestHandoverKeepsChartPasswords(t *testing.T) {
	foreignAnnotations := map[string]string{"meta.helm.sh/release-name": "other", "meta.helm.sh/release-namespace": "d8-system"}
	c := newHandoverFakeClient(
		chartPassword("jane@example.com", maps.Clone(chartAnnotations)),
		// Created by user-authn-controller: no release annotations, the keep policy set.
		chartPassword("john@example.com", map[string]string{handover.ResourcePolicyAnnotation: handover.ResourcePolicyKeep}),
		chartPassword("bob@example.com", foreignAnnotations),
	)

	if !prunedByNelm(getPassword(t, c, "jane@example.com")) {
		t.Fatal("nelm keeps the Password of the 1.76 chart without the hook; the test no longer reproduces the loss")
	}

	res, err := handover.Protect(context.Background(), c, handoverKinds)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if res.Stamped != 1 || res.Owned != 1 {
		t.Errorf("result = %+v, want 1 owned and stamped", res)
	}

	jane := getPassword(t, c, "jane@example.com")
	if prunedByNelm(jane) {
		t.Errorf("Password of the chart is pruned after Protect: annotations %v", jane.GetAnnotations())
	}
	if got := policyOf(t, c, passwordGVR, dexNamespace, encoding.ToFnvLikeDex("bob@example.com")); got != "" {
		t.Errorf("Password of another release policy = %q, want untouched", got)
	}

	// A protected Password stays protected: the controller keeps the policy. Nothing is owned any
	// more, so the hook stops looking.
	res, err = handover.Protect(context.Background(), c, handoverKinds)
	if err != nil || res != (handover.Result{}) {
		t.Errorf("second Protect = %+v, %v; want nothing owned", res, err)
	}
}
