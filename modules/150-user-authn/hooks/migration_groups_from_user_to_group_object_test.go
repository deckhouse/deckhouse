/*
Copyright 2023 Flant JSC

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

	"github.com/flant/addon-operator/sdk"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/set"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: migration to Group object ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "User", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "Group", false)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})
	})

	Context("Cluster with a configmap", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: user-authn-groups-migrated
  namespace: d8-system
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should run and do nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			groups := getGroups(f)
			Expect(groups).To(HaveLen(0))
		})
	})

	Context("Cluster with User objects", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  groups:
  - Admins
  - Everyone
  password: password
---
apiVersion: deckhouse.io/v1alpha1
kind: User
metadata:
  name: user
spec:
  email: user@example.com
  groups:
  - admins
  - Everyone
  - -Test
  - flant/auth
  - /path/style
  - longname.longname.longname.longname.longname.longname.longname.longname.longname.longname.longname.longname.longname.longname.longname.longname.longname.longname.longname.longname
  - "%$$flant////auth-das@3123@"

  password: passwordNext
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should synchronize objects and fill internal values", func() {
			Expect(f).To(ExecuteSuccessfully())

			groups := getGroups(f)

			names := set.New()
			nameToMembers := map[string][]string{}

			for _, ugroup := range groups {
				group := &DexGroup{}

				err := sdk.FromUnstructured(&ugroup, group)
				Expect(err).To(BeNil())

				s := set.New()
				for _, member := range group.Spec.Members {
					if member.Kind == DexGroupKind {
						Fail("Only Users can be members in the test")
					}
					s.Add(member.Name)
				}

				names.Add(group.Name)
				nameToMembers[group.Name] = s.Slice()
			}

			Expect(names).To(HaveLen(8))
			Expect(nameToMembers).To(HaveLen(8))

			sanitizedNames := names.Slice()
			Expect(sanitizedNames).To(Equal([]string{
				"admins",
				"admins-2909721157",
				"everyone-622959000",
				"flant----auth-das-3123-1790689325",
				"flant-auth-2950615535",
				"longname-longname-longname-longname-longname-longnam-4160692933",
				"path-style-2010161443",
				"test-3218847196",
			}))

			for _, name := range sanitizedNames {
				Expect(name).To(MatchRegexp(metadataNamePattern))
				Expect(len(names)).To(BeNumerically("<=", maxMetadataNameLength))
			}

			assertMembers := func(name string, members []string) {
				By(name, func() {
					Expect(nameToMembers[name]).To(Equal(members))
				})
			}

			assertMembers("admins", []string{"user"})
			assertMembers("admins-2909721157", []string{"admin"})
			assertMembers("everyone-622959000", []string{"admin", "user"})
			assertMembers("flant----auth-das-3123-1790689325", []string{"user"})
			assertMembers("flant-auth-2950615535", []string{"user"})
			assertMembers("longname-longname-longname-longname-longname-longnam-4160692933", []string{"user"})
			assertMembers("path-style-2010161443", []string{"user"})
			assertMembers("test-3218847196", []string{"user"})

			Expect(f.KubernetesResource("ConfigMap", "d8-system", "user-authn-groups-migrated").Exists()).To(BeTrue())
		})
	})
})

func getGroups(f *HookExecutionConfig) []unstructured.Unstructured {
	gvr := schema.GroupVersionResource{
		Group:    DexGroupGroup,
		Version:  DexGroupVersion,
		Resource: DexGroupResource,
	}

	groups, err := f.KubeClient().Dynamic().Resource(gvr).List(context.TODO(), v1.ListOptions{})
	Expect(err).To(BeNil())

	return groups.Items
}
