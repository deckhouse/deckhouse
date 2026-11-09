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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: discovery_network_name ::", func() {
	const (
		initValues = `{"global":{"discovery":{"clusterDomain":"my.domain","clusterUUID":"aa-bb-cc"}},"istio":{"internal":{}}}`
		// Rendered by earlier releases.
		legacyName = "network-my-domain-199688871"
		// network- and the first 16 hex digits of sha256("aa-bb-cc").
		newName = "network-09710b4801ba99ae"

		longClusterDomain = "aaaaaaaaaa.bbbbbbbbbb.cccccccccc.dddddddddd.eee"
		// The legacy name for longClusterDomain. It is not a valid label value.
		longName = "network-aaaaaaaaaa-bbbbbbbbbb-cccccccccc-dddddddddd-eee-199688871"
	)

	invalidNameMetrics := []operation.MetricOperation{
		{
			Group:  networkNameMetricsGroup,
			Action: operation.ActionExpireMetrics,
		},
		{
			Name:   networkNameInvalidMetric,
			Group:  networkNameMetricsGroup,
			Action: operation.ActionGaugeSet,
			Value:  ptr.To(1.0),
			Labels: map[string]string{"network_name": longName},
		},
	}

	namespace := func(annotation, label string) *corev1.Namespace {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "d8-istio"}}
		if annotation != "" {
			ns.Annotations = map[string]string{"istio.deckhouse.io/network-name": annotation}
		}
		if label != "" {
			ns.Labels = map[string]string{"topology.istio.io/network": label}
		}
		return ns
	}

	f := HookExecutionConfigInit(initValues, "")

	// KubeStateSet fills only the dynamic client, and the hook uses the typed one.
	prepare := func(ns *corev1.Namespace) {
		f.BindingContexts.Set(f.GenerateBeforeHelmContext())
		f.KubeStateSet(``)
		if ns != nil {
			_, err := f.KubeClient().CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		}
	}

	run := func(ns *corev1.Namespace) {
		prepare(ns)
		f.RunHook()
	}

	networkName := func() string {
		return f.ValuesGet("istio.internal.networkName").String()
	}

	Context("A new installation", func() {
		BeforeEach(func() {
			run(nil)
		})

		It("Generates a name that does not depend on the cluster domain", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(networkName()).To(Equal(newName))
		})

		It("Sets no metrics", func() {
			Expect(f.MetricsCollector.CollectedMetrics()).To(Equal([]operation.MetricOperation{{
				Group:  networkNameMetricsGroup,
				Action: operation.ActionExpireMetrics,
			}}))
		})
	})

	Context("A new installation on a cluster with a long cluster domain", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterDomain", longClusterDomain)
			run(nil)
		})

		It("Still gets a short name", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(networkName()).To(Equal(newName))
		})
	})

	Context("The namespace cannot be read", func() {
		BeforeEach(func() {
			prepare(nil)
			f.KubeClient().Interface.(*kubefake.Clientset).PrependReactor("get", "namespaces",
				func(clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("connection refused")
				})
			f.RunHook()
		})

		It("Fails instead of taking the cluster for a new installation", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("connection refused"))
			Expect(networkName()).To(BeEmpty())
		})
	})

	Context("An existing installation of a release that did not set the label", func() {
		BeforeEach(func() {
			run(namespace("", ""))
		})

		It("Keeps the name the module has always derived", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(networkName()).To(Equal(legacyName))
		})
	})

	// Rare: the cluster domain changed, and the old release had no Helm run after that.
	Context("An existing installation with a label that differs from the legacy name", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterDomain", "other.domain")
			run(namespace("", legacyName))
		})

		It("Keeps the name from the label", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(networkName()).To(Equal(legacyName))
		})
	})

	Context("A new-style installation lost the annotation", func() {
		BeforeEach(func() {
			run(namespace("", newName))
		})

		It("Takes the name from the label instead of deriving the legacy one", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(networkName()).To(Equal(newName))
		})
	})

	Context("The annotation holds a name", func() {
		BeforeEach(func() {
			run(namespace("network-from-annotation", ""))
		})

		It("Uses the stored name", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(networkName()).To(Equal("network-from-annotation"))
		})
	})

	Context("The annotation and the label hold different names", func() {
		BeforeEach(func() {
			run(namespace("network-from-annotation", newName))
		})

		It("Prefers the annotation", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(networkName()).To(Equal("network-from-annotation"))
		})
	})

	Context("The annotation holds a name derived from an earlier cluster domain", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterDomain", "other.domain")
			run(namespace(legacyName, ""))
		})

		It("Keeps the stored name instead of following the cluster domain", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(networkName()).To(Equal(legacyName))
		})
	})

	Context("The name is already known and there is no namespace", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.networkName", "network-known")
			run(nil)
		})

		It("Keeps the known name for Helm to store", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(networkName()).To(Equal("network-known"))
		})
	})

	// An edit of the annotation is how the network is renamed on purpose.
	Context("The known name and the annotation differ", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.networkName", "network-known")
			run(namespace("network-from-annotation", newName))
		})

		It("Renames the network to the annotation", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(networkName()).To(Equal("network-from-annotation"))
		})
	})

	// The first run after an upgrade, when Helm failed to store the name.
	Context("The name is known and the namespace has no annotation yet", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.networkName", "network-known")
			run(namespace("", ""))
		})

		It("Keeps the known name instead of deriving the legacy one", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(networkName()).To(Equal("network-known"))
		})
	})

	Context("A new installation without the cluster UUID", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterUUID", "")
			run(nil)
		})

		It("Fails instead of generating a name from nothing", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("global.discovery.clusterUUID is not set"))
		})
	})

	Context("An existing installation without the annotation and the cluster UUID", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterUUID", "")
			run(namespace("", ""))
		})

		It("Fails instead of deriving a name that differs from the one in use", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("cannot derive the istio network name"))
		})
	})

	// A broken annotation must fail the hook, not the render.
	Context("The annotation holds a name that is not safe for the manifests", func() {
		BeforeEach(func() {
			run(namespace("network: x", ""))
		})

		It("Fails and names the annotation", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring(`the istio.deckhouse.io/network-name annotation of the d8-istio namespace holds "network: x"`))
			Expect(networkName()).To(BeEmpty())
		})
	})

	Context("The stored name is not a valid label value", func() {
		BeforeEach(func() {
			run(namespace(longName, ""))
		})

		It("Keeps the name and reports it as invalid", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(networkName()).To(Equal(longName))
			Expect(f.MetricsCollector.CollectedMetrics()).To(Equal(invalidNameMetrics))
		})
	})

	// The usual upgrade path for such clusters: earlier releases did not set the label.
	Context("An existing installation with a long cluster domain and no label", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterDomain", longClusterDomain)
			run(namespace("", ""))
		})

		It("Keeps the long legacy name and reports it as invalid", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(networkName()).To(Equal(longName))
			Expect(f.MetricsCollector.CollectedMetrics()).To(Equal(invalidNameMetrics))
		})
	})
})
