/*
Copyright 2022 Flant JSC

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
	"fmt"

	clientv3 "go.etcd.io/etcd/client/v3"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = FDescribe("Modules :: controler-plane-manager :: hooks :: etcd-defragmentation ::", func() {
	var (
		initValuesString        = `{"controlPlaneManager":{"internal": {}, "apiserver": {"authn": {}, "authz": {}}}}`
		endpointToDbSize        = make(map[string]int64)
		endpointTriggeredDefrag = make(map[string]struct{})
		endpointDefragError     = make(map[string]string)
	)
	const (
		initConfigValuesString = ``
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	testHelperRegisterEtcdMemberUpdate()

	dependency.TestDC.EtcdClient.StatusMock.Set(func(ctx context.Context, endpoint string) (sp1 *clientv3.StatusResponse, err error) {
		size, ok := endpointToDbSize[endpoint]

		if !ok {
			return nil, fmt.Errorf("some error")
		}

		return &clientv3.StatusResponse{
			DbSize: size,
		}, nil
	})

	dependency.TestDC.EtcdClient.DefragmentMock.Set(func(ctx context.Context, endpoint string) (dp1 *clientv3.DefragmentResponse, err error) {
		endpointTriggeredDefrag[endpoint] = struct{}{}

		if msg, ok := endpointDefragError[endpoint]; ok {
			return nil, fmt.Errorf(msg)
		}

		return &clientv3.DefragmentResponse{}, nil
	})

	assertSetSuccessMetric := func(f *HookExecutionConfig, podName string) {
		metrics := f.MetricsCollector.CollectedMetrics()

		Expect(metrics).To(HaveLen(1))
		Expect(metrics[0].Name).To(Equal("etcd_defragmentation_success"))
		Expect(metrics[0].Labels).To(HaveKey("pod_name"))
		Expect(metrics[0].Labels["pod_name"]).To(Equal(podName))
	}

	assertSetErrorMetric := func(f *HookExecutionConfig, podName string, errMsg string) {
		metrics := f.MetricsCollector.CollectedMetrics()

		Expect(metrics).To(HaveLen(1))
		Expect(metrics[0].Name).To(Equal("etcd_defragmentation_failed"))
		Expect(metrics[0].Labels).To(HaveKey("pod_name"))
		Expect(metrics[0].Labels["pod_name"]).To(Equal(podName))
		Expect(metrics[0].Labels["defrag_error"]).To(Equal(errMsg))
	}

	Context("Single master", func() {
		AfterEach(func() {
			endpointToDbSize = make(map[string]int64)
			endpointTriggeredDefrag = make(map[string]struct{})
			endpointDefragError = make(map[string]string)
		})

		Context("etcd does not have quota-backend-bytes parameter", func() {
			Context("etcd db size is 0 bytes", func() {
				ip := "192.168.0.1"
				endpoint := etcdEndpoint(ip)
				BeforeEach(func() {
					podManifest := etcdPodManifest(map[string]interface{}{
						"name":   "etcd-pod1",
						"hostIP": ip,
					})
					endpointToDbSize[endpoint] = 0

					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest)
					f.RunHook()
				})

				It("Defragmentation was not triggered", func() {
					Expect(f).Should(ExecuteSuccessfully())

					Expect(endpointTriggeredDefrag).ToNot(HaveKey(endpoint))
				})
			})

			Context("etcd db size is between zero and default maximum", func() {
				ip := "192.168.0.2"
				endpoint := etcdEndpoint(ip)
				BeforeEach(func() {
					podManifest := etcdPodManifest(map[string]interface{}{
						"name":   "etcd-pod2",
						"hostIP": ip,
					})
					endpointToDbSize[endpoint] = 750 * 1024 * 1024 // 750 MB
					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest)

					f.RunHook()
				})

				It("Defragmentation was not triggered", func() {
					Expect(f).Should(ExecuteSuccessfully())

					Expect(endpointTriggeredDefrag).ToNot(HaveKey(endpoint))
				})
			})

			Context("etcd db size is 95% of default maximum", func() {
				ip := "192.168.0.3"
				endpoint := etcdEndpoint(ip)
				podName := "etcd-pod3"
				BeforeEach(func() {
					podManifest := etcdPodManifest(map[string]interface{}{
						"name":   podName,
						"hostIP": ip,
					})
					endpointToDbSize[endpoint] = 2040109466
					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest)

					f.RunHook()
				})

				It("Defragmentation was triggered", func() {
					Expect(f).Should(ExecuteSuccessfully())

					Expect(endpointTriggeredDefrag).To(HaveKey(endpoint))
				})

				It("Set success metric", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertSetSuccessMetric(f, podName)
				})
			})

			Context("etcd db size is great than 95% and less than default maximum", func() {
				ip := "192.168.0.4"
				endpoint := etcdEndpoint(ip)
				podName := "etcd-pod4"
				BeforeEach(func() {
					podManifest := etcdPodManifest(map[string]interface{}{
						"name":   podName,
						"hostIP": ip,
					})
					endpointToDbSize[endpoint] = 2104533975
					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest)

					f.RunHook()
				})

				It("Defragmentation was triggered", func() {
					Expect(f).Should(ExecuteSuccessfully())

					Expect(endpointTriggeredDefrag).To(HaveKey(endpoint))
				})

				It("Set success metric", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertSetSuccessMetric(f, podName)
				})
			})

			Context("etcd db size is default maximum", func() {
				ip := "192.168.0.5"
				endpoint := etcdEndpoint(ip)
				podName := "etcd-pod5"
				BeforeEach(func() {
					podManifest := etcdPodManifest(map[string]interface{}{
						"name":   podName,
						"hostIP": ip,
					})
					endpointToDbSize[endpoint] = 2 * 1024 * 1024 * 1024
					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest)

					f.RunHook()
				})

				It("Defragmentation was triggered", func() {
					Expect(f).Should(ExecuteSuccessfully())

					Expect(endpointTriggeredDefrag).To(HaveKey(endpoint))
				})

				It("Set success metric", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertSetSuccessMetric(f, podName)
				})
			})

			Context("defragmentation returns error", func() {
				ip := "192.168.0.6"
				endpoint := etcdEndpoint(ip)
				podName := "etcd-pod6"
				errMsg := "some connection error"
				BeforeEach(func() {
					podManifest := etcdPodManifest(map[string]interface{}{
						"name":   podName,
						"hostIP": ip,
					})
					endpointToDbSize[endpoint] = 2 * 1024 * 1024 * 1024
					endpointDefragError[endpoint] = errMsg
					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest)

					f.RunHook()
				})

				It("Defragmentation was triggered", func() {
					Expect(f).Should(ExecuteSuccessfully())

					Expect(endpointTriggeredDefrag).To(HaveKey(endpoint))
				})

				It("Set error metric", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertSetErrorMetric(f, podName, errMsg)
				})
			})

			Context("getting endpoint status returns error", func() {
				ip := "192.168.0.7"
				endpoint := etcdEndpoint(ip)
				podName := "etcd-pod7"
				BeforeEach(func() {
					podManifest := etcdPodManifest(map[string]interface{}{
						"name":   podName,
						"hostIP": ip,
					})
					// no dbSize return error
					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest)

					f.RunHook()
				})

				It("Defragmentation was not triggered", func() {
					Expect(f).Should(ExecuteSuccessfully())

					Expect(endpointTriggeredDefrag).ToNot(HaveKey(endpoint))
				})

				It("Should not set metrics", func() {
					Expect(f).Should(ExecuteSuccessfully())

					Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(0))
				})
			})
		})

		Context("etcd has quota-backend-bytes parameter", func() {
			manifest := func(data map[string]interface{}) string {
				data["maxDbSize"] = 4 * 1024 * 1024 * 1024
				return etcdPodManifest(data)
			}

			Context("etcd db size is 0 bytes", func() {
				ip := "192.168.1.1"
				endpoint := etcdEndpoint(ip)
				BeforeEach(func() {
					podManifest := manifest(map[string]interface{}{
						"name":   "etcd-pod2-1",
						"hostIP": ip,
					})
					endpointToDbSize[endpoint] = 0

					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest)
					f.RunHook()
				})

				It("Defragmentation was not triggered", func() {
					Expect(f).Should(ExecuteSuccessfully())

					Expect(endpointTriggeredDefrag).ToNot(HaveKey(endpoint))
				})
			})

			Context("etcd db size is between zero and maximum", func() {
				ip := "192.168.1.2"
				endpoint := etcdEndpoint(ip)
				BeforeEach(func() {
					podManifest := manifest(map[string]interface{}{
						"name":   "etcd-pod2-2",
						"hostIP": ip,
					})
					endpointToDbSize[endpoint] = 3 * 1024 * 1024 * 1024
					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest)

					f.RunHook()
				})

				It("Defragmentation was not triggered", func() {
					Expect(f).Should(ExecuteSuccessfully())

					Expect(endpointTriggeredDefrag).ToNot(HaveKey(endpoint))
				})
			})

			Context("etcd db size is 95% of maximum", func() {
				ip := "192.168.1.3"
				endpoint := etcdEndpoint(ip)
				podName := "etcd-pod2-3"
				BeforeEach(func() {
					podManifest := manifest(map[string]interface{}{
						"name":   podName,
						"hostIP": ip,
					})
					endpointToDbSize[endpoint] = 4080218932
					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest)

					f.RunHook()
				})

				It("Defragmentation was triggered", func() {
					Expect(f).Should(ExecuteSuccessfully())

					Expect(endpointTriggeredDefrag).To(HaveKey(endpoint))
				})

				It("Set success metric", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertSetSuccessMetric(f, podName)
				})
			})

			Context("etcd db size is great than 95% and less than default maximum", func() {
				ip := "192.168.1.4"
				endpoint := etcdEndpoint(ip)
				podName := "etcd-pod2-4"
				BeforeEach(func() {
					podManifest := manifest(map[string]interface{}{
						"name":   podName,
						"hostIP": ip,
					})
					endpointToDbSize[endpoint] = 4209067951
					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest)

					f.RunHook()
				})

				It("Defragmentation was triggered", func() {
					Expect(f).Should(ExecuteSuccessfully())

					Expect(endpointTriggeredDefrag).To(HaveKey(endpoint))
				})

				It("Set success metric", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertSetSuccessMetric(f, podName)
				})
			})

			Context("etcd db size is default maximum", func() {
				ip := "192.168.1.5"
				endpoint := etcdEndpoint(ip)
				podName := "etcd-pod2-5"
				BeforeEach(func() {
					podManifest := manifest(map[string]interface{}{
						"name":   podName,
						"hostIP": ip,
					})
					endpointToDbSize[endpoint] = 4 * 1024 * 1024 * 1024
					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest)

					f.RunHook()
				})

				It("Defragmentation was triggered", func() {
					Expect(f).Should(ExecuteSuccessfully())

					Expect(endpointTriggeredDefrag).To(HaveKey(endpoint))
				})

				It("Set success metric", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertSetSuccessMetric(f, podName)
				})
			})
		})
	})
})
