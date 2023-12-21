// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func createNs(namespace string) error {
	var ns *v1.Namespace

	err := yaml.Unmarshal([]byte(namespace), &ns)
	if err != nil {
		return err
	}

	_, err = dependency.TestDC.K8sClient.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

var _ = Describe("Modules :: external module manager :: hooks :: migration create module update policies ::", func() {

	const (
		d8SystemWithoutAnnotation = `
---
apiVersion: v1
data:
kind: Namespace
metadata:
  name: d8-system
`
		d8SystemWithAnnotation = `
---
apiVersion: v1
data:
kind: Namespace
metadata:
  annotations:
    modules.deckhouse.io/ensured-update-policies: ""
  name: d8-system
`
		deckhouseMs = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  labels:
    heritage: deckhouse
  name: deckhouse
spec:
  registry:
    ca: ""
    dockerCfg: cfg
    repo: dev-registry.deckhouse.io/sys/deckhouse-oss/modules
    scheme: HTTPS
  releaseChannel: Stable
`
		customMss = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  labels:
  name: tetris
spec:
  registry:
    ca: ""
    dockerCfg: cfg
    repo: dev-registry.deckhouse.io/sys/deckhouse-oss/modules
    scheme: HTTPS
  releaseChannel: Stable
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: pingpong
spec:
  registry:
    ca: ""
    dockerCfg: cfg
    repo: dev-registry.deckhouse.io/sys/deckhouse-oss/modules
    scheme: HTTPS
  releaseChannel: Alpha
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: tictactoe
spec:
  registry:
    ca: ""
    dockerCfg: cfg
    repo: dev-registry.deckhouse.io/sys/deckhouse-oss/modules
    scheme: HTTPS
  releaseChannel: RockSolid
`
		customMssWithCustomReleaseChannels = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  labels:
  name: gtaV
spec:
  registry:
    ca: ""
    dockerCfg: cfg
    repo: dev-registry.deckhouse.io/sys/deckhouse-oss/modules
    scheme: HTTPS
  releaseChannel: early-access
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: sleepingDogs
spec:
spec:
  registry:
    ca: ""
    dockerCfg: cfg
    repo: dev-registry.deckhouse.io/sys/deckhouse-oss/modules
    scheme: HTTPS
  releaseChannel: rocksolid
`
	)

	f := HookExecutionConfigInit(`{}`, `{}`)

	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleSource", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleUpdatePolicy", false)

	Context("An empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(d8SystemWithoutAnnotation))

			err := createNs(d8SystemWithoutAnnotation)
			if err != nil {
				Fail(err.Error())
			}

			f.RunHook()
		})

		It("Should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("A cluster with deckhouse ModuleSource", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(d8SystemWithoutAnnotation + deckhouseMs))

			err := createNs(d8SystemWithoutAnnotation)
			if err != nil {
				Fail(err.Error())
			}

			f.RunHook()
		})

		It("Should ignore deckhouse ModuleSource", func() {
			Expect(f).To(ExecuteSuccessfully())

			mupDeckhouse := f.KubernetesGlobalResource("ModuleUpdatePolicy", "deckhouse")
			Expect(mupDeckhouse.Exists()).To(BeTrue())
			msDeckhouse := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(msDeckhouse.Exists()).To(BeTrue())
		})
	})

	Context("A cluster with client's ModuleSources", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(d8SystemWithoutAnnotation + deckhouseMs + customMss))

			err := createNs(d8SystemWithoutAnnotation)
			if err != nil {
				Fail(err.Error())
			}

			f.RunHook()
		})

		It("Should create ModuleUpdatePolicies and annotate d8-system", func() {
			Expect(f).To(ExecuteSuccessfully())

			mupDeckhouse := f.KubernetesGlobalResource("ModuleUpdatePolicy", "deckhouse")
			Expect(mupDeckhouse.Exists()).To(BeTrue())
			msDeckhouse := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(msDeckhouse.Exists()).To(BeTrue())
			for _, ms := range []string{"tetris", "pingpong", "tictactoe"} {
				mup := f.KubernetesGlobalResource("ModuleUpdatePolicy", ms)
				Expect(mup.Exists()).To(BeTrue())
			}
			ns := f.KubernetesGlobalResource("Namespace", "d8-system")
			Expect(ns.Field(`metadata.annotations.modules\.deckhouse\.io/ensured-update-policies`).Exists()).To(BeTrue())
		})
	})

	Context("A cluster with client's ModuleSources and custom release channels", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(d8SystemWithoutAnnotation + deckhouseMs + customMss + customMssWithCustomReleaseChannels))

			err := createNs(d8SystemWithoutAnnotation)
			if err != nil {
				Fail(err.Error())
			}

			f.RunHook()
		})

		It("Should create ModuleUpdatePolicies for ModulesSources with correct release channels and annotate d8-system", func() {
			Expect(f).To(ExecuteSuccessfully())

			mupDeckhouse := f.KubernetesGlobalResource("ModuleUpdatePolicy", "deckhouse")
			Expect(mupDeckhouse.Exists()).To(BeTrue())
			msDeckhouse := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(msDeckhouse.Exists()).To(BeTrue())
			for _, ms := range []string{"tetris", "pingpong", "tictactoe", "gtaV"} {
				mup := f.KubernetesGlobalResource("ModuleUpdatePolicy", ms)
				Expect(mup.Exists()).To(BeTrue())
			}
			for _, ms := range []string{"sleepingDogs"} {
				mup := f.KubernetesGlobalResource("ModuleUpdatePolicy", ms)
				Expect(mup.Exists()).To(BeFalse())
			}
			ns := f.KubernetesGlobalResource("Namespace", "d8-system")
			Expect(ns.Field(`metadata.annotations.modules\.deckhouse\.io/ensured-update-policies`).Exists()).To(BeTrue())
		})
	})

	Context("A cluster with client's ModuleSources with annotation", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(d8SystemWithAnnotation + deckhouseMs + customMss))

			err := createNs(d8SystemWithAnnotation)
			if err != nil {
				Fail(err.Error())
			}

			f.RunHook()
		})

		It("Shouldn't create/update any ModuleUpdatePolicies", func() {
			Expect(f).To(ExecuteSuccessfully())

			mupDeckhouse := f.KubernetesGlobalResource("ModuleUpdatePolicy", "deckhouse")
			Expect(mupDeckhouse.Exists()).To(BeFalse())
			msDeckhouse := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(msDeckhouse.Exists()).To(BeTrue())
			ns := f.KubernetesGlobalResource("Namespace", "d8-system")
			Expect(ns.Field(`metadata.annotations.modules\.deckhouse\.io/ensured-update-policies`).Exists()).To(BeTrue())
			for _, ms := range []string{"tetris", "pingpong", "tictactoe"} {
				mup := f.KubernetesGlobalResource("ModuleUpdatePolicy", ms)
				Expect(mup.Exists()).To(BeFalse())
			}
		})
	})
})
