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

package fencing

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// systemNetworkClient is a client of the hook with a fake dynamic client: the hook does nothing else.
type systemNetworkClient struct {
	kubernetes.Interface
	dynamic dynamic.Interface
}

func (c systemNetworkClient) Dynamic() dynamic.Interface { return c.dynamic }

func moduleConfigYAML(network string) string {
	return `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: node-manager
spec:
  enabled: true
  settings:
    fencing:
      systemNetworkFallback:
        systemNetworkName: "` + network + `"
`
}

var _ = Describe("Modules :: node-manager :: hooks :: fencing :: system_network ::", func() {
	var gets []string
	var dyn *dynamicfake.FakeDynamicClient

	// serve makes the API answer a read of a SystemNetwork with the given result.
	serve := func(err error) {
		gets = nil
		dyn = dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
		dyn.PrependReactor("get", "systemnetworks", func(action k8stesting.Action) (bool, runtime.Object, error) {
			gets = append(gets, action.(k8stesting.GetAction).GetName())
			if err != nil {
				return true, nil, err
			}
			network := &unstructured.Unstructured{}
			network.SetGroupVersionKind(schema.GroupVersionKind{Group: "network.deckhouse.io", Version: "v1alpha1", Kind: "SystemNetwork"})
			network.SetName(action.(k8stesting.GetAction).GetName())
			return true, network, nil
		})
	}

	expire := operation.MetricOperation{Group: systemNetworkNotFoundMetric, Action: operation.ActionExpireMetrics}
	notFound := func(network string) operation.MetricOperation {
		return operation.MetricOperation{
			Name:   systemNetworkNotFoundMetric,
			Group:  systemNetworkNotFoundMetric,
			Action: operation.ActionGaugeSet,
			Value:  ptr.To(1.0),
			Labels: map[string]string{"network": network},
		}
	}

	run := func(f *HookExecutionConfig, moduleConfig string) {
		f.BindingContexts.Set(f.KubeStateSet(moduleConfig))
		// The framework puts its own client in the container when it builds the state of the cluster.
		dependency.TestDC.K8sClient = systemNetworkClient{dynamic: dyn}
		f.RunGoHook()
	}

	Context("Edition EE", func() {
		f := HookExecutionConfigInit(`{"global":{"deckhouseEdition":"EE"}}`, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

		It("Must publish the series when the network was deleted, and change no setting", func() {
			serve(apierrors.NewNotFound(schema.GroupResource{Group: "network.deckhouse.io", Resource: "systemnetworks"}, "sysnet-a"))
			values, configValues := f.ValuesGet("").String(), f.ConfigValuesGet("").String()
			run(f, moduleConfigYAML("sysnet-a"))

			Expect(f).To(ExecuteSuccessfully())
			Expect(gets).To(Equal([]string{"sysnet-a"}))
			Expect(f.MetricsCollector.CollectedMetrics()).To(Equal([]operation.MetricOperation{expire, notFound("sysnet-a")}))
			Expect(f.PatchCollector.Operations()).To(BeEmpty())
			Expect(f.ValuesGet("").String()).To(Equal(values))
			Expect(f.ConfigValuesGet("").String()).To(Equal(configValues))
		})

		It("Must remove the series when the network came back", func() {
			serve(nil)
			run(f, moduleConfigYAML("sysnet-a"))

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(Equal([]operation.MetricOperation{expire}))
		})

		It("Must replace the series of the old name when the name changes", func() {
			serve(apierrors.NewNotFound(schema.GroupResource{}, "sysnet-b"))
			run(f, moduleConfigYAML("sysnet-b"))

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(Equal([]operation.MetricOperation{expire, notFound("sysnet-b")}))
		})

		It("Must treat a missing CRD as a missing network", func() {
			serve(&apierrors.StatusError{ErrStatus: metav1.Status{
				Status: metav1.StatusFailure, Code: 404, Reason: metav1.StatusReasonNotFound,
				Message: "the server could not find the requested resource",
			}})
			run(f, moduleConfigYAML("sysnet-a"))

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(Equal([]operation.MetricOperation{expire, notFound("sysnet-a")}))
		})

		for name, err := range map[string]error{
			"the API is unavailable": apierrors.NewServiceUnavailable("etcd is not ready"),
			"the hook has no rights": apierrors.NewForbidden(schema.GroupResource{Group: "network.deckhouse.io", Resource: "systemnetworks"}, "sysnet-a", nil),
		} {
			It("Must fail and leave the metric as it is when "+name, func() {
				serve(err)
				run(f, moduleConfigYAML("sysnet-a"))

				Expect(f).NotTo(ExecuteSuccessfully())
				Expect(f.MetricsCollector.CollectedMetrics()).To(BeEmpty())
			})
		}

		It("Must only expire the series, and read nothing, when the name is empty", func() {
			serve(nil)
			run(f, moduleConfigYAML(""))

			Expect(f).To(ExecuteSuccessfully())
			Expect(gets).To(BeEmpty())
			Expect(f.MetricsCollector.CollectedMetrics()).To(Equal([]operation.MetricOperation{expire}))
		})

		It("Must only expire the series, and read nothing, without the ModuleConfig", func() {
			serve(nil)
			run(f, ``)

			Expect(f).To(ExecuteSuccessfully())
			Expect(gets).To(BeEmpty())
			Expect(f.MetricsCollector.CollectedMetrics()).To(Equal([]operation.MetricOperation{expire}))
		})
	})

	Context("Edition CE, where the setting does not exist", func() {
		f := HookExecutionConfigInit(`{"global":{"deckhouseEdition":"CE"}}`, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

		It("Must always expire the series and read nothing", func() {
			serve(apierrors.NewNotFound(schema.GroupResource{}, "sysnet-a"))
			run(f, moduleConfigYAML("sysnet-a"))

			Expect(f).To(ExecuteSuccessfully())
			Expect(gets).To(BeEmpty())
			Expect(f.MetricsCollector.CollectedMetrics()).To(Equal([]operation.MetricOperation{expire}))
		})
	})
})
