// Copyright 2025 Flant JSC
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

	"github.com/flant/addon-operator/sdk"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: cleanup_d8_cni_secret ::", func() {
	createFakeCNISecret := func(name, data string) {
		secretData := make(map[string][]byte)
		secretData["cni"] = []byte(name)
		if data != "" {
			secretData[name] = []byte(data)
		}

		s := &v1core.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},

			ObjectMeta: metav1.ObjectMeta{
				Name:      "d8-cni-configuration",
				Namespace: "kube-system",
			},

			Data: secretData,
		}

		_, err := dependency.TestDC.MustGetK8sClient().CoreV1().Secrets("kube-system").Create(context.TODO(), s, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
	}
	createCNIModuleConfig := func(name string, enabled *bool, settings config.SettingsValues) {
		mc := config.ModuleConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1alpha1",
				Kind:       "ModuleConfig",
			},

			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},

			Spec: config.ModuleConfigSpec{
				Version:  1,
				Settings: settings,
				Enabled:  enabled,
			},
		}

		mcu, err := sdk.ToUnstructured(&mc)
		if err != nil {
			panic(err)
		}

		_, err = dependency.TestDC.MustGetK8sClient().Dynamic().Resource(config.ModuleConfigGVR).Create(context.TODO(), mcu, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
	}

	f := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	Context("Cluster has no d8-cni-configuration secret and has no enabled moduleConfig of CNI", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("d8-cni-configuration secret does not exist, skipping cleanup"))
			secret, err := f.KubeClient().CoreV1().Secrets("kube-system").Get(context.TODO(), "d8-cni-configuration", metav1.GetOptions{})
			Expect(err).NotTo(BeNil())
			Expect(secret).To(BeNil())
			mc, err := f.KubeClient().Dynamic().Resource(config.ModuleConfigGVR).Get(context.TODO(), "cni-cilium", metav1.GetOptions{})
			Expect(err).NotTo(BeNil())
			Expect(mc).To(BeNil())
			mc, err = f.KubeClient().Dynamic().Resource(config.ModuleConfigGVR).Get(context.TODO(), "cni-flannel", metav1.GetOptions{})
			Expect(err).NotTo(BeNil())
			Expect(mc).To(BeNil())
			mc, err = f.KubeClient().Dynamic().Resource(config.ModuleConfigGVR).Get(context.TODO(), "cni-simple-bridge", metav1.GetOptions{})
			Expect(err).NotTo(BeNil())
			Expect(mc).To(BeNil())
		})
	})

	Context("The cluster has a d8-cni-configuration secret, but there are no enabled moduleConfig of CNI in it.", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			createFakeCNISecret("flannel", `{"podNetworkMode": "VXLAN"}`)
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("No one enabled moduleConfig of CNI is found, skipping cleanup"))
			secret, err := f.KubeClient().CoreV1().Secrets("kube-system").Get(context.TODO(), "d8-cni-configuration", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(secret).NotTo(BeNil())
			mc, err := f.KubeClient().Dynamic().Resource(config.ModuleConfigGVR).Get(context.TODO(), "cni-cilium", metav1.GetOptions{})
			Expect(err).NotTo(BeNil())
			Expect(mc).To(BeNil())
			mc, err = f.KubeClient().Dynamic().Resource(config.ModuleConfigGVR).Get(context.TODO(), "cni-flannel", metav1.GetOptions{})
			Expect(err).NotTo(BeNil())
			Expect(mc).To(BeNil())
			mc, err = f.KubeClient().Dynamic().Resource(config.ModuleConfigGVR).Get(context.TODO(), "cni-simple-bridge", metav1.GetOptions{})
			Expect(err).NotTo(BeNil())
			Expect(mc).To(BeNil())
		})
	})

	Context("The cluster has a d8-cni-configuration secret, and has disabled moduleConfig of cni-flannel.", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			createFakeCNISecret("flannel", `{"podNetworkMode": "VXLAN"}`)
			createCNIModuleConfig("cni-flannel", pointer.Bool(false), nil)
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("No one enabled moduleConfig of CNI is found, skipping cleanup"))
			secret, err := f.KubeClient().CoreV1().Secrets("kube-system").Get(context.TODO(), "d8-cni-configuration", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(secret).NotTo(BeNil())
			mc, err := f.KubeClient().Dynamic().Resource(config.ModuleConfigGVR).Get(context.TODO(), "cni-cilium", metav1.GetOptions{})
			Expect(err).NotTo(BeNil())
			Expect(mc).To(BeNil())
			mc, err = f.KubeClient().Dynamic().Resource(config.ModuleConfigGVR).Get(context.TODO(), "cni-flannel", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(mc).NotTo(BeNil())
			mc, err = f.KubeClient().Dynamic().Resource(config.ModuleConfigGVR).Get(context.TODO(), "cni-simple-bridge", metav1.GetOptions{})
			Expect(err).NotTo(BeNil())
			Expect(mc).To(BeNil())
		})
	})

	Context("The cluster has a d8-cni-configuration secret, and has enabled moduleConfig of cni-flannel.", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			createFakeCNISecret("flannel", `{"podNetworkMode": "VXLAN"}`)
			createCNIModuleConfig("cni-flannel", pointer.Bool(true), nil)
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("Module config for cni-flannel found, secret will be cleaned"))
			secret, err := f.KubeClient().CoreV1().Secrets("kube-system").Get(context.TODO(), "d8-cni-configuration", metav1.GetOptions{})
			Expect(err).NotTo(BeNil())
			Expect(secret).To(BeNil())
			mc, err := f.KubeClient().Dynamic().Resource(config.ModuleConfigGVR).Get(context.TODO(), "cni-cilium", metav1.GetOptions{})
			Expect(err).NotTo(BeNil())
			Expect(mc).To(BeNil())
			mc, err = f.KubeClient().Dynamic().Resource(config.ModuleConfigGVR).Get(context.TODO(), "cni-flannel", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(mc).NotTo(BeNil())
			mc, err = f.KubeClient().Dynamic().Resource(config.ModuleConfigGVR).Get(context.TODO(), "cni-simple-bridge", metav1.GetOptions{})
			Expect(err).NotTo(BeNil())
			Expect(mc).To(BeNil())
		})
	})
})
