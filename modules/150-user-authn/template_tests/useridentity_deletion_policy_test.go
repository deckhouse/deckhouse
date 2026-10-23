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

package template_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	admissioncel "k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/authentication/user"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const userIdentityDeletionPolicyName = "user-authn-useridentity-deletion.deckhouse.io"

// A UserIdentity deleted by hand comes back at generation 0, below the revocations that
// dex-authenticator, the user-authz webhook and basic-auth-proxy keep, so they would refuse the
// new sign-ins of the user until the revocations expire. Only the owners and the cluster garbage
// collectors may delete one.
var _ = Describe("Module :: user-authn :: helm template :: UserIdentity deletion policy", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.33.0")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")
		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", "plainstring")
	})

	It("Should not render the policy without its API", func() {
		hec.HelmRender()
		Expect(hec.RenderError).ToNot(HaveOccurred())
		Expect(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", userIdentityDeletionPolicyName).Exists()).To(BeFalse())
		Expect(hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", userIdentityDeletionPolicyName).Exists()).To(BeFalse())
	})

	It("Should let only the owners and the garbage collectors delete a UserIdentity", func() {
		hec.HelmRender(WithAPIVersions(
			validatingAdmissionPolicyAPIVersion,
			validatingAdmissionPolicyBindingAPIVersion,
		))
		Expect(hec.RenderError).ToNot(HaveOccurred())

		policy := hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", userIdentityDeletionPolicyName)
		Expect(policy.Exists()).To(BeTrue())
		Expect(policy.Field("spec.failurePolicy").String()).To(Equal("Fail"))
		Expect(policy.Field("spec.matchConstraints.resourceRules.0.operations").AsStringSlice()).To(Equal([]string{"DELETE"}))
		Expect(policy.Field("spec.matchConstraints.resourceRules.0.resources").AsStringSlice()).To(Equal([]string{"useridentities"}))

		binding := hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", userIdentityDeletionPolicyName)
		Expect(binding.Exists()).To(BeTrue())
		Expect(binding.Field("spec.validationActions").AsStringSlice()).To(Equal([]string{"Deny"}))

		compiled := compileUserAPIPolicy(policy.ToYaml())
		for username, admitted := range map[string]bool{
			"system:serviceaccount:d8-user-authn:controller":              true,
			"system:serviceaccount:d8-user-authn:dex":                     true,
			"system:serviceaccount:kube-system:namespace-controller":      true,
			"system:serviceaccount:kube-system:generic-garbage-collector": true,
			"admin@example.com":                                       false,
			"kubernetes-admin":                                        false,
			"system:serviceaccount:d8-system:deckhouse":               false,
			"system:serviceaccount:d8-user-authn:user-api":            false,
			"system:serviceaccount:d8-user-authn:controller-impostor": false,
			"system:serviceaccount:other-namespace:controller":        false,
		} {
			ok, err := compiled.admitUserIdentityDeletion(username)
			Expect(err).ToNot(HaveOccurred(), username)
			Expect(ok).To(Equal(admitted), username)
		}
	})
})

// admitUserIdentityDeletion reports whether the policy admits the deletion of a UserIdentity by
// username.
func (p compiledPolicy) admitUserIdentityDeletion(username string) (bool, error) {
	gvk := schema.GroupVersionKind{Group: "dex.coreos.com", Version: "v1", Kind: "UserIdentity"}
	gvr := schema.GroupVersionResource{Group: "dex.coreos.com", Version: "v1", Resource: "useridentities"}

	old := &unstructured.Unstructured{}
	old.SetGroupVersionKind(gvk)
	old.SetNamespace("d8-user-authn")
	old.SetName("abcdefghijklm")

	attributes := admission.NewAttributesRecord(
		nil, old, gvk, "d8-user-authn", "abcdefghijklm", gvr, "", admission.Delete, nil, false,
		&user.DefaultInfo{Name: username, Groups: defaultGroups(username)},
	)
	versionedAttributes, err := admission.NewVersionedAttributes(attributes, gvk, nil)
	if err != nil {
		return false, err
	}

	request := admissioncel.CreateAdmissionRequest(
		attributes, metav1.GroupVersionResource(gvr), metav1.GroupVersionKind(gvk),
	)
	bindings := admissioncel.OptionalVariableBindings{}

	selected, err := p.evaluate(p.matchConditions, versionedAttributes, request, bindings)
	if err != nil || !selected {
		return true, err
	}

	return p.evaluate(p.validations, versionedAttributes, request, bindings)
}
