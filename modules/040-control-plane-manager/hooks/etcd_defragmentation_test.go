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

	"github.com/deckhouse/deckhouse/go_lib/filter"

	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: controler-plane-manager :: hooks :: etcd-defragmentation ::", func() {
	var (
		initValuesString        = `{"controlPlaneManager":{"internal": {}, "apiserver": {"authn": {}, "authz": {}}}}`
		endpointToDbSize        = make(map[string]int64)
		endpointTriggeredDefrag = make(map[string]struct{})
		endpointDefragError     = make(map[string]string)
	)

	AfterEach(func() {
		endpointToDbSize = make(map[string]int64)
		endpointTriggeredDefrag = make(map[string]struct{})
		endpointDefragError = make(map[string]string)
	})

	f := HookExecutionConfigInit(initValuesString, "")
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

	assertSuccessMetricCorrect := func(metric operation.MetricOperation, podName, node string) {
		Expect(metric.Name).To(Equal("d8_etcd_defragmentation_success_total"))
		Expect(metric.Labels).To(HaveKey("pod_name"))
		Expect(metric.Labels["pod_name"]).To(Equal(podName))
		Expect(metric.Labels["node"]).To(Equal(node))
	}

	assertSetSuccessMetric := func(f *HookExecutionConfig, podName, node string) {
		metrics := f.MetricsCollector.CollectedMetrics()

		Expect(metrics).To(HaveLen(1))
		assertSuccessMetricCorrect(metrics[0], podName, node)
	}

	assertErrorMetricCorrect := func(metric operation.MetricOperation, podName, node, errMsg string) {
		Expect(metric.Name).To(Equal("d8_etcd_defragmentation_failed_total"))
		Expect(metric.Labels).To(HaveKey("pod_name"))
		Expect(metric.Labels["node"]).To(Equal(node))
		Expect(metric.Labels["pod_name"]).To(Equal(podName))
	}

	assertSetErrorMetric := func(f *HookExecutionConfig, podName, node, errMsg string) {
		metrics := f.MetricsCollector.CollectedMetrics()

		Expect(metrics).To(HaveLen(1))
		assertErrorMetricCorrect(metrics[0], podName, node, errMsg)
	}

	Context("Single master", func() {
		Context("etcd does not have quota-backend-bytes parameter", func() {
			Context("etcd db size is 0 bytes", func() {
				ip := "192.168.0.1"
				endpoint := filter.EtcdEndpoint(ip)
				node := "node-1"
				BeforeEach(func() {
					podManifest := etcdPodManifest(map[string]interface{}{
						"name":     "etcd-pod1",
						"hostIP":   ip,
						"nodeName": node,
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
				endpoint := filter.EtcdEndpoint(ip)
				node := "node-2"
				BeforeEach(func() {
					podManifest := etcdPodManifest(map[string]interface{}{
						"name":     "etcd-pod2",
						"hostIP":   ip,
						"nodeName": node,
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

			Context("etcd db size is 90% of default maximum", func() {
				ip := "192.168.0.3"
				endpoint := filter.EtcdEndpoint(ip)
				podName := "etcd-pod3"
				node := "node-3"
				BeforeEach(func() {
					podManifest := etcdPodManifest(map[string]interface{}{
						"name":     podName,
						"hostIP":   ip,
						"nodeName": node,
					})
					endpointToDbSize[endpoint] = 1932735284
					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest)

					f.RunHook()
				})

				It("Defragmentation was triggered", func() {
					Expect(f).Should(ExecuteSuccessfully())

					Expect(endpointTriggeredDefrag).To(HaveKey(endpoint))
				})

				It("Set success metric", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertSetSuccessMetric(f, podName, node)
				})
			})

			Context("etcd db size is great than 90% and less than default maximum", func() {
				ip := "192.168.0.4"
				endpoint := filter.EtcdEndpoint(ip)
				podName := "etcd-pod4"
				node := "node-4"
				BeforeEach(func() {
					podManifest := etcdPodManifest(map[string]interface{}{
						"name":     podName,
						"hostIP":   ip,
						"nodeName": node,
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

					assertSetSuccessMetric(f, podName, node)
				})
			})

			Context("etcd db size is default maximum", func() {
				ip := "192.168.0.5"
				endpoint := filter.EtcdEndpoint(ip)
				podName := "etcd-pod5"
				node := "node-5"
				BeforeEach(func() {
					podManifest := etcdPodManifest(map[string]interface{}{
						"name":     podName,
						"hostIP":   ip,
						"nodeName": node,
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

					assertSetSuccessMetric(f, podName, node)
				})
			})

			Context("defragmentation returns error", func() {
				ip := "192.168.0.6"
				endpoint := filter.EtcdEndpoint(ip)
				podName := "etcd-pod6"
				errMsg := "some connection error"
				node := "node-6"
				BeforeEach(func() {
					podManifest := etcdPodManifest(map[string]interface{}{
						"name":     podName,
						"hostIP":   ip,
						"nodeName": node,
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

					assertSetErrorMetric(f, podName, node, errMsg)
				})
			})

			Context("getting endpoint status returns error", func() {
				ip := "192.168.0.7"
				endpoint := filter.EtcdEndpoint(ip)
				podName := "etcd-pod7"
				node := "node-7"
				BeforeEach(func() {
					podManifest := etcdPodManifest(map[string]interface{}{
						"name":     podName,
						"hostIP":   ip,
						"nodeName": node,
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
				endpoint := filter.EtcdEndpoint(ip)
				node := "node-1-1"
				BeforeEach(func() {
					podManifest := manifest(map[string]interface{}{
						"name":     "etcd-pod2-1",
						"hostIP":   ip,
						"nodeName": node,
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
				endpoint := filter.EtcdEndpoint(ip)
				node := "node-2-2"
				BeforeEach(func() {
					podManifest := manifest(map[string]interface{}{
						"name":     "etcd-pod2-2",
						"hostIP":   ip,
						"nodeName": node,
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

			Context("etcd db size is 90% of maximum", func() {
				ip := "192.168.1.3"
				endpoint := filter.EtcdEndpoint(ip)
				podName := "etcd-pod2-3"
				node := "node-2-1"
				BeforeEach(func() {
					podManifest := manifest(map[string]interface{}{
						"name":     podName,
						"hostIP":   ip,
						"nodeName": node,
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

					assertSetSuccessMetric(f, podName, node)
				})
			})

			Context("etcd db size is great than 90% and less than current maximum", func() {
				ip := "192.168.1.4"
				endpoint := filter.EtcdEndpoint(ip)
				podName := "etcd-pod2-4"
				node := "node-2-4"
				BeforeEach(func() {
					podManifest := manifest(map[string]interface{}{
						"name":     podName,
						"hostIP":   ip,
						"nodeName": node,
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

					assertSetSuccessMetric(f, podName, node)
				})
			})

			Context("etcd db size is current maximum", func() {
				ip := "192.168.1.5"
				endpoint := filter.EtcdEndpoint(ip)
				podName := "etcd-pod2-5"
				node := "node-2-5"
				BeforeEach(func() {
					podManifest := manifest(map[string]interface{}{
						"name":     podName,
						"hostIP":   ip,
						"nodeName": node,
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

					assertSetSuccessMetric(f, podName, node)
				})
			})
		})
	})

	Context("Multi-master", func() {
		manifests := func(ips []string, namePrefix, nodePrefix string) []string {
			res := make([]string, 0, len(ips))
			for i, ip := range ips {
				res = append(res, etcdPodManifest(map[string]interface{}{
					"name":     fmt.Sprintf("%s-%d", namePrefix, i),
					"nodeName": fmt.Sprintf("%s-%d", nodePrefix, i),
					"hostIP":   ip,
				}))
			}

			return res
		}

		Context("all instances don't have quota-backend-bytes", func() {
			Context("all instances have current db size less than 90%", func() {
				ips := []string{"192.18.10.1", "192.18.10.2", "192.18.10.3"}
				BeforeEach(func() {
					resources := manifests(ips, "etcd-pod-10", "node-10")
					resources = append(resources, testETCDSecret)

					for _, ip := range ips {
						endpointToDbSize[ip] = 500 * 1024 * 1024
					}

					JoinKubeResourcesAndSet(f, resources...)

					f.RunHook()
				})

				It("should not trigger defrag for all instances", func() {
					Expect(f).Should(ExecuteSuccessfully())

					for _, ip := range ips {
						Expect(endpointTriggeredDefrag).ToNot(HaveKey(filter.EtcdEndpoint(ip)))
					}
				})

				It("should not set any metrics", func() {
					Expect(f).Should(ExecuteSuccessfully())

					metrics := f.MetricsCollector.CollectedMetrics()
					Expect(metrics).To(HaveLen(0))
				})
			})

			Context("two instances have current db size greater than 90%", func() {
				ips := []string{"192.18.11.1", "192.18.11.2", "192.18.11.3"}
				BeforeEach(func() {
					resources := manifests(ips, "etcd-pod-11", "node-11")
					resources = append(resources, testETCDSecret)
					JoinKubeResourcesAndSet(f, resources...)

					endpointToDbSize[filter.EtcdEndpoint(ips[0])] = 2 * 1024 * 1024 * 1024
					endpointToDbSize[filter.EtcdEndpoint(ips[1])] = 2 * 1024 * 1024 * 1024

					f.RunHook()
				})

				It("should trigger defrag for instances have current db size greater than 90%", func() {
					Expect(f).Should(ExecuteSuccessfully())

					Expect(endpointTriggeredDefrag).To(HaveKey(filter.EtcdEndpoint(ips[0])))
					Expect(endpointTriggeredDefrag).To(HaveKey(filter.EtcdEndpoint(ips[1])))
				})

				It("should not trigger defrag for instance has current db size less than 90%", func() {
					Expect(f).Should(ExecuteSuccessfully())

					Expect(endpointTriggeredDefrag).ToNot(HaveKey(filter.EtcdEndpoint(ips[2])))
				})

				It("should set success metrics for or instances have current db size greater than 90%", func() {
					Expect(f).Should(ExecuteSuccessfully())

					metrics := f.MetricsCollector.CollectedMetrics()
					Expect(metrics).To(HaveLen(2))

					assertSuccessMetricCorrect(metrics[0], "etcd-pod-11-0", "node-11-0")
					assertSuccessMetricCorrect(metrics[1], "etcd-pod-11-1", "node-11-1")
				})
			})

			Context("all instances have current db size greater than 90%", func() {
				Context("one instance has defrag error", func() {
					errMsg := "defrag error"
					ips := []string{"192.18.12.1", "192.18.12.2", "192.18.12.3"}
					BeforeEach(func() {
						resources := manifests(ips, "etcd-pod-12", "node-12")
						resources = append(resources, testETCDSecret)
						JoinKubeResourcesAndSet(f, resources...)

						for _, ip := range ips {
							endpointToDbSize[filter.EtcdEndpoint(ip)] = 2 * 1024 * 1024 * 1024
						}

						endpointDefragError[filter.EtcdEndpoint(ips[1])] = errMsg

						f.RunHook()
					})

					It("should trigger defrag for all instances", func() {
						Expect(f).Should(ExecuteSuccessfully())

						for _, ip := range ips {
							Expect(endpointTriggeredDefrag).To(HaveKey(filter.EtcdEndpoint(ip)))
						}
					})

					It("should set success metrics for two instances", func() {
						Expect(f).Should(ExecuteSuccessfully())

						metrics := f.MetricsCollector.CollectedMetrics()
						Expect(metrics).To(HaveLen(3))

						assertSuccessMetricCorrect(metrics[0], "etcd-pod-12-0", "node-12-0")
						assertSuccessMetricCorrect(metrics[2], "etcd-pod-12-2", "node-12-2")
					})

					It("should set error metric for second instance", func() {
						Expect(f).Should(ExecuteSuccessfully())

						metrics := f.MetricsCollector.CollectedMetrics()
						Expect(metrics).To(HaveLen(3))

						assertErrorMetricCorrect(metrics[1], "etcd-pod-12-1", "node-12-1", errMsg)
					})
				})

				Context("one instance returned status error", func() {
					ips := []string{"192.18.13.1", "192.18.13.2", "192.18.13.3"}
					BeforeEach(func() {
						resources := manifests(ips, "etcd-pod-13", "node-13")
						resources = append(resources, testETCDSecret)
						JoinKubeResourcesAndSet(f, resources...)

						endpointToDbSize[filter.EtcdEndpoint(ips[1])] = 2 * 1024 * 1024 * 1024
						endpointToDbSize[filter.EtcdEndpoint(ips[2])] = 2 * 1024 * 1024 * 1024

						f.RunHook()
					})

					It("should trigger defrag for instances without status error", func() {
						Expect(f).Should(ExecuteSuccessfully())

						Expect(endpointTriggeredDefrag).To(HaveKey(filter.EtcdEndpoint(ips[1])))
						Expect(endpointTriggeredDefrag).To(HaveKey(filter.EtcdEndpoint(ips[2])))
					})

					It("should not trigger defrag for instance with status error", func() {
						Expect(f).Should(ExecuteSuccessfully())

						Expect(endpointTriggeredDefrag).ToNot(HaveKey(filter.EtcdEndpoint(ips[0])))
					})

					It("should set success metrics for two instances", func() {
						Expect(f).Should(ExecuteSuccessfully())

						metrics := f.MetricsCollector.CollectedMetrics()
						Expect(metrics).To(HaveLen(2))

						assertSuccessMetricCorrect(metrics[0], "etcd-pod-13-1", "node-13-1")
						assertSuccessMetricCorrect(metrics[1], "etcd-pod-13-2", "node-13-2")
					})
				})
			})
		})
	})

	Context("Defragmentation disabled from config", func() {
		ip := "192.168.20.1"
		endpoint := filter.EtcdEndpoint(ip)

		BeforeEach(func() {
			podManifest := etcdPodManifest(map[string]interface{}{
				"name":     "etcd-pod20",
				"hostIP":   ip,
				"nodeName": "node-20-0",
			})
			endpointToDbSize[endpoint] = filter.EtcdDefaultMaxSize - 100

			f.ValuesSetFromYaml("controlPlaneManager.etcd", []byte(`{"disableAutoDefragmentation": true}`))

			JoinKubeResourcesAndSet(f, testETCDSecret, podManifest)
			f.RunHook()
		})

		It("Does not trigger auto defragmentation", func() {
			Expect(f).Should(ExecuteSuccessfully())

			Expect(endpointTriggeredDefrag).ToNot(HaveKey(endpoint))
		})
	})

	Context("Defragmentation enabled from config", func() {
		ip := "192.168.20.2"
		endpoint := filter.EtcdEndpoint(ip)

		BeforeEach(func() {
			podManifest := etcdPodManifest(map[string]interface{}{
				"name":     "etcd-pod20",
				"hostIP":   ip,
				"nodeName": "node-21-0",
			})
			endpointToDbSize[endpoint] = filter.EtcdDefaultMaxSize - 100

			f.ValuesSetFromYaml("controlPlaneManager.etcd", []byte(`{"disableAutoDefragmentation": false}`))

			JoinKubeResourcesAndSet(f, testETCDSecret, podManifest)
			f.RunHook()
		})

		It("Does not trigger auto defragmentation", func() {
			Expect(f).Should(ExecuteSuccessfully())

			Expect(endpointTriggeredDefrag).To(HaveKey(endpoint))
		})
	})
})
