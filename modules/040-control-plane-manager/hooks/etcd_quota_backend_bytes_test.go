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
	"fmt"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: control-plane-manager :: hooks :: etcd-quota-backend-bytes ::", func() {
	Context("CalcNewQuota", func() {
		casesIncrementalIncrease := []struct {
			nodeSize, newQuota int64
		}{
			{
				nodeSize: gb(4),
				newQuota: gb(2),
			},

			{
				nodeSize: gb(8),
				newQuota: gb(2),
			},

			{
				nodeSize: gb(16),
				newQuota: gb(2),
			},

			{
				nodeSize: gbFloat(19.69),
				newQuota: gb(2),
			},

			{
				nodeSize: gbFloat(23.48),
				newQuota: gb(3),
			},

			{
				nodeSize: gb(24),
				newQuota: gb(3),
			},

			{
				nodeSize: gbFloat(31.01),
				newQuota: gb(4),
			},

			{
				nodeSize: gb(32),
				newQuota: gb(4),
			},

			{
				nodeSize: gbFloat(37.91),
				newQuota: gb(5),
			},

			{
				nodeSize: gb(40),
				newQuota: gb(5),
			},

			{
				nodeSize: gbFloat(45.64),
				newQuota: gb(6),
			},

			{
				nodeSize: gb(48),
				newQuota: gb(6),
			},

			{
				nodeSize: gbFloat(54.21),
				newQuota: gb(7),
			},

			{
				nodeSize: gb(56),
				newQuota: gb(7),
			},

			{
				nodeSize: gbFloat(61.82),
				newQuota: gb(8),
			},

			{
				nodeSize: gb(64),
				newQuota: gb(8),
			},

			{
				nodeSize: gbFloat(69.52),
				newQuota: gb(8),
			},

			{
				nodeSize: gb(72),
				newQuota: gb(8),
			},

			{
				nodeSize: gb(80),
				newQuota: gb(8),
			},

			{
				nodeSize: gb(88),
				newQuota: gb(8),
			},

			{
				nodeSize: gb(96),
				newQuota: gb(8),
			},

			{
				nodeSize: gb(128),
				newQuota: gb(8),
			},

			{
				nodeSize: gb(256),
				newQuota: gb(8),
			},
		}

		for _, c := range casesIncrementalIncrease {
			It(fmt.Sprintf("Node size %d", c.nodeSize/1024/1024/1024), func() {
				newQuota := calcNewQuotaForMemory(c.nodeSize)

				Expect(newQuota).To(Equal(c.newQuota))
			})
		}
	})

	Context("getNodeWithMinimalMemory", func() {
		cases := []struct {
			title        string
			nodes        []go_hook.FilterResult
			expectedNode *etcdNode
		}{
			{
				title: "For single node always return this node",
				nodes: []go_hook.FilterResult{
					&etcdNode{
						Memory:      gb(8),
						IsDedicated: true,
					}},
				expectedNode: &etcdNode{
					Memory:      gb(8),
					IsDedicated: true,
				},
			},

			{
				title: "For all different nodes return with minimal memory",
				nodes: []go_hook.FilterResult{
					&etcdNode{
						Memory:      gb(8),
						IsDedicated: true,
					},
					&etcdNode{
						Memory:      gb(12),
						IsDedicated: true,
					},
				},

				expectedNode: &etcdNode{
					Memory:      gb(8),
					IsDedicated: true,
				},
			},

			{
				title: "For all different nodes return with minimal memory",
				nodes: []go_hook.FilterResult{
					&etcdNode{
						Memory:      gb(16),
						IsDedicated: true,
					},
					&etcdNode{
						Memory:      gb(8),
						IsDedicated: true,
					},
					&etcdNode{
						Memory:      gb(12),
						IsDedicated: true,
					},
				},

				expectedNode: &etcdNode{
					Memory:      gb(8),
					IsDedicated: true,
				},
			},

			{
				title: "For all different nodes return with minimal memory",
				nodes: []go_hook.FilterResult{
					&etcdNode{
						Memory:      gb(16),
						IsDedicated: true,
					},
					&etcdNode{
						Memory:      gb(12),
						IsDedicated: true,
					},
					&etcdNode{
						Memory:      gb(8),
						IsDedicated: true,
					},
				},

				expectedNode: &etcdNode{
					Memory:      gb(8),
					IsDedicated: true,
				},
			},

			{
				title: "For all different nodes return with minimal memory",
				nodes: []go_hook.FilterResult{
					&etcdNode{
						Memory:      gb(8),
						IsDedicated: true,
					},
					&etcdNode{
						Memory:      gb(12),
						IsDedicated: true,
					},
					&etcdNode{
						Memory:      gb(16),
						IsDedicated: true,
					},
				},

				expectedNode: &etcdNode{
					Memory:      gb(8),
					IsDedicated: true,
				},
			},

			{
				title: "If have dedicated node, return dedicated node",
				nodes: []go_hook.FilterResult{
					&etcdNode{
						Memory:      gb(8),
						IsDedicated: true,
					},
					&etcdNode{
						Memory:      gb(12),
						IsDedicated: false,
					},
					&etcdNode{
						Memory:      gb(16),
						IsDedicated: true,
					},
				},

				expectedNode: &etcdNode{
					Memory:      gb(12),
					IsDedicated: false,
				},
			},

			{
				title: "If have two dedicated nodes, return first dedicated node",
				nodes: []go_hook.FilterResult{
					&etcdNode{
						Memory:      gb(8),
						IsDedicated: true,
					},
					&etcdNode{
						Memory:      gb(12),
						IsDedicated: false,
					},
					&etcdNode{
						Memory:      gb(16),
						IsDedicated: false,
					},
				},

				expectedNode: &etcdNode{
					Memory:      gb(12),
					IsDedicated: false,
				},
			},
		}

		for _, c := range cases {
			It(c.title, func() {
				node := getNodeWithMinimalMemory(c.nodes)

				Expect(node).To(Equal(c.expectedNode))
			})
		}
	})

	var (
		initValuesString = `{"controlPlaneManager":{"internal": {}, "apiserver": {"authn": {}, "authz": {}}}}`
	)

	f := HookExecutionConfigInit(initValuesString, "")

	getNodeManifest := func(name string, memory int64, setTaint bool) string {
		spec := ""
		if setTaint {
			spec = `
spec:
  taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
`
		}
		return fmt.Sprintf(`
apiVersion: v1
kind: Node
metadata:
  name: %s
  labels:
    node-role.kubernetes.io/control-plane: ""
%s
status:
  addresses:
    - address: 192.168.1.2
      type: InternalIP
  capacity:
    cpu: "4"
    ephemeral-storage: 20145724Ki
    hugepages-1Gi: "0"
    hugepages-2Mi: "0"
    memory: %d
    pods: "110"

`, name, spec, memory)
	}

	assertNewQuotaBackendsWithMetric := func(f *HookExecutionConfig, newSize int64) {
		size := f.ValuesGet("controlPlaneManager.internal.etcdQuotaBackendBytes").String()
		Expect(size).To(Equal(strconv.FormatInt(newSize, 10)))

		metrics := f.MetricsCollector.CollectedMetrics()
		Expect(metrics).ToNot(BeEmpty())

		found := false
		for _, m := range metrics {
			if m.Name == "d8_etcd_quota_backend_total" {
				Expect(*m.Value).To(Equal(float64(newSize)))
				found = true
			}
		}

		Expect(found).To(BeTrue())
	}

	assertClearMetrics := func(f *HookExecutionConfig) {
		metrics := f.MetricsCollector.CollectedMetrics()

		Expect(metrics).ToNot(BeEmpty())

		Expect(metrics[0].Action).To(Equal("expire"))
		Expect(metrics[0].Group).To(Equal(etcdBackendBytesGroup))
	}

	assertAddErrorMetric := func(f *HookExecutionConfig) {
		metrics := f.MetricsCollector.CollectedMetrics()

		Expect(metrics).ToNot(BeEmpty())

		Expect(metrics[0].Action).To(Equal("expire"))
		Expect(metrics[0].Group).To(Equal(etcdBackendBytesGroup))

		found := false
		for _, m := range metrics {
			if m.Name == "d8_etcd_quota_backend_should_decrease" {
				Expect(m.Group).To(Equal(etcdBackendBytesGroup))
				Expect(m.Value).To(Equal(pointer.Float64(1.0)))

				found = true
			}
		}

		Expect(found).To(BeTrue())
	}

	Context("User set quota in config", func() {
		userValue := gb(4)
		BeforeEach(func() {
			etcdConf := fmt.Sprintf(`{"maxDbSize": %d}`, userValue)

			f.ValuesSetFromYaml("controlPlaneManager.etcd", []byte(etcdConf))

			f.RunHook()
		})

		It("set quota-backend-bytes from config", func() {
			Expect(f).Should(ExecuteSuccessfully())

			assertNewQuotaBackendsWithMetric(f, userValue)
		})

	})

	Context("Single master", func() {
		Context("etcd does not have quota-backend-bytes parameter", func() {
			nodeName := "control-plane-0"
			ip := "192.162.0.1"
			Context("node has less than 24 Gb", func() {
				BeforeEach(func() {
					podManifest := etcdPodManifest(map[string]interface{}{
						"name":     "etcd-pod1",
						"nodeName": nodeName,
						"hostIP":   ip,
					})
					nodeManifest := getNodeManifest(nodeName, gb(8), true)

					JoinKubeResourcesAndSet(f, podManifest, nodeManifest)

					f.RunHook()
				})

				It("set default backend size as quota-backend-bytes", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackendsWithMetric(f, gb(2))
				})

				It("clean all metrics in group", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertClearMetrics(f)
				})
			})

			Context("node has great equal 24 Gb", func() {
				BeforeEach(func() {
					podManifest := etcdPodManifest(map[string]interface{}{
						"name":     "etcd-pod1",
						"nodeName": nodeName,
						"hostIP":   ip,
					})
					nodeManifest := getNodeManifest(nodeName, gb(24), true)

					JoinKubeResourcesAndSet(f, podManifest, nodeManifest)

					f.RunHook()
				})

				It("set increase on 1 gb of default size as quota-backend-bytes", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackendsWithMetric(f, gb(3))
				})

				It("clean all metrics in group", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertClearMetrics(f)
				})
			})

			Context("node has 64 Gb", func() {
				BeforeEach(func() {
					podManifest := etcdPodManifest(map[string]interface{}{
						"name":     "etcd-pod1",
						"nodeName": nodeName,
						"hostIP":   ip,
					})
					nodeManifest := getNodeManifest(nodeName, gb(64), true)

					JoinKubeResourcesAndSet(f, podManifest, nodeManifest)

					f.RunHook()
				})

				It("set maximum size for quota-backend-bytes", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackendsWithMetric(f, gb(8))
				})

				It("clean all metrics in group", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertClearMetrics(f)
				})
			})

			Context("node has greater than 24 Gb and is not dedicated", func() {
				BeforeEach(func() {
					podManifest := etcdPodManifest(map[string]interface{}{
						"name":     "etcd-pod1",
						"nodeName": nodeName,
						"hostIP":   ip,
					})
					nodeManifest := getNodeManifest(nodeName, gb(32), false)

					JoinKubeResourcesAndSet(f, podManifest, nodeManifest)

					f.RunHook()
				})

				It("set default backend size as quota-backend-bytes", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackendsWithMetric(f, gb(2))
				})

				It("clean all metrics in group", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertClearMetrics(f)
				})
			})
		})

		Context("etcd has quota-backend-bytes parameter", func() {
			nodeName := "control-plane-1"

			etcdManifest := func(quotaBackendSize int64) string {
				data := map[string]interface{}{
					"maxDbSize": quotaBackendSize,
					"nodeName":  nodeName,
					"hostIP":    "192.162.1.1",
				}

				return etcdPodManifest(data)
			}

			Context("node has less than 24 Gb", func() {
				quotaBackend := gb(2)
				BeforeEach(func() {
					podManifest := etcdManifest(quotaBackend)
					nodeManifest := getNodeManifest(nodeName, gb(16), true)

					JoinKubeResourcesAndSet(f, podManifest, nodeManifest)

					f.RunHook()
				})

				It("does not change quota-backend-bytes", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackendsWithMetric(f, gb(2))
				})

				It("clean all metrics in group", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertClearMetrics(f)
				})
			})

			Context("node has 24 Gb, quota backends is default", func() {
				BeforeEach(func() {
					podManifest := etcdManifest(2)
					nodeManifest := getNodeManifest(nodeName, gb(24), true)

					JoinKubeResourcesAndSet(f, podManifest, nodeManifest)

					f.RunHook()
				})

				It("increase on 1 gb of default size as quota-backend-bytes", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackendsWithMetric(f, gb(3))
				})

				It("clean all metrics in group", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertClearMetrics(f)
				})
			})

			Context("node has 64 Gb, current backend size is 3 gb, increase node by 40 Gb", func() {
				BeforeEach(func() {
					podManifest := etcdManifest(gb(3))
					nodeManifest := getNodeManifest(nodeName, gb(64), true)

					JoinKubeResourcesAndSet(f, podManifest, nodeManifest)

					f.RunHook()
				})

				It("set maximum size for quota-backend-bytes", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackendsWithMetric(f, gb(8))
				})

				It("clean all metrics in group", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertClearMetrics(f)
				})
			})

			Context("node has greater than 24 Gb and node is not dedicated", func() {
				quotaBackend := gb(3)

				BeforeEach(func() {
					podManifest := etcdManifest(quotaBackend)
					nodeManifest := getNodeManifest(nodeName, gb(32), false)

					JoinKubeResourcesAndSet(f, podManifest, nodeManifest)

					f.RunHook()
				})

				It("set current backend size", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackendsWithMetric(f, quotaBackend)
				})

				It("clean all metrics in group", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertClearMetrics(f)
				})
			})

			Context("all nodes less memory than need for apply current quota-backend-bytes (decrease node case)", func() {
				quotaBackend := gb(3)

				BeforeEach(func() {
					podManifest := etcdManifest(quotaBackend)
					nodeManifest := getNodeManifest(nodeName, gb(8), true)

					JoinKubeResourcesAndSet(f, podManifest, nodeManifest)

					f.RunHook()
				})

				It("set current backend size", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackendsWithMetric(f, quotaBackend)
				})

				It("clean all metrics in group", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertAddErrorMetric(f)
				})
			})
		})
	})

	Context("Multi-master", func() {
		type testNode struct {
			memory   int64
			etcQuota int64
			setTaint bool
		}
		getManifests := func(nodePrefix string, nodes []testNode) []string {
			res := make([]string, 0, len(nodes))
			for i, n := range nodes {
				node := fmt.Sprintf("%s-%d", nodePrefix, i)
				data := map[string]interface{}{
					"name":     fmt.Sprintf("%s-control-plane-%d", node, i),
					"nodeName": node,
					"hostIP":   fmt.Sprintf("192.162.0.%d", i),
				}
				if n.etcQuota > 0 {
					data["maxDbSize"] = n.etcQuota
				}

				res = append(res, etcdPodManifest(data))
				res = append(res, getNodeManifest(node, n.memory, n.setTaint))
			}

			return res
		}

		Context("all etcd instances don't have quota-backend-bytes", func() {
			Context("all nodes have less than 24 Gb memory", func() {
				BeforeEach(func() {
					manifests := getManifests("control-plane-1", []testNode{
						{
							memory:   gb(8),
							etcQuota: 0,
							setTaint: true,
						},
						{
							memory:   gb(12),
							etcQuota: 0,
							setTaint: true,
						},
						{
							memory:   gb(16),
							etcQuota: 0,
							setTaint: true,
						},
					})

					JoinKubeResourcesAndSet(f, manifests...)

					f.RunHook()
				})

				It("set default backend quota", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackendsWithMetric(f, gb(2))
				})

				It("clean all metrics in group", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertClearMetrics(f)
				})
			})

			Context("one node have great equal than 24 Gb memory", func() {
				BeforeEach(func() {
					manifests := getManifests("control-plane-2", []testNode{
						{
							memory:   gb(8),
							etcQuota: 0,
							setTaint: true,
						},
						{
							memory:   gb(24),
							etcQuota: 0,
							setTaint: true,
						},
						{
							memory:   gb(16),
							etcQuota: 0,
							setTaint: true,
						},
					})

					JoinKubeResourcesAndSet(f, manifests...)

					f.RunHook()
				})

				It("set default backend quota", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackendsWithMetric(f, gb(2))
				})

				It("clean all metrics in group", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertClearMetrics(f)
				})
			})

			Context("all nodes have 24 Gb memory", func() {
				BeforeEach(func() {
					manifests := getManifests("control-plane-2", []testNode{
						{
							memory:   gb(24),
							etcQuota: 0,
							setTaint: true,
						},
						{
							memory:   gb(24),
							etcQuota: 0,
							setTaint: true,
						},
						{
							memory:   gb(24),
							etcQuota: 0,
							setTaint: true,
						},
					})

					JoinKubeResourcesAndSet(f, manifests...)

					f.RunHook()
				})

				It("increase quota backends on 1 gb (set 3gb)", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackendsWithMetric(f, gb(3))
				})

				It("clean all metrics in group", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertClearMetrics(f)
				})
			})

			Context("all nodes have 32 Gb memory, but one not dedicated", func() {
				BeforeEach(func() {
					manifests := getManifests("control-plane-2", []testNode{
						{
							memory:   gb(32),
							etcQuota: 0,
							setTaint: true,
						},
						{
							memory:   gb(32),
							etcQuota: 0,
							setTaint: true,
						},
						{
							memory:   gb(32),
							etcQuota: 0,
							setTaint: false,
						},
					})

					JoinKubeResourcesAndSet(f, manifests...)

					f.RunHook()
				})

				It("set default backend size", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackendsWithMetric(f, gb(2))
				})

				It("clean all metrics in group", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertClearMetrics(f)
				})
			})
		})

		Context("one etcd instance has quota-backend-bytes", func() {
			Context("all nodes have 24 Gb memory but quota-backend-bytes not default on etcd instance", func() {
				BeforeEach(func() {
					manifests := getManifests("control-plane-1", []testNode{
						{
							memory:   gb(24),
							etcQuota: 0,
							setTaint: true,
						},
						{
							memory:   gb(24),
							etcQuota: gb(1),
							setTaint: true,
						},
						{
							memory:   gb(24),
							etcQuota: 0,
							setTaint: true,
						},
					})

					JoinKubeResourcesAndSet(f, manifests...)

					f.RunHook()
				})

				It("set backend for 24Gb instance (3gb)", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackendsWithMetric(f, gb(3))
				})

				It("clean all metrics in group", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertClearMetrics(f)
				})
			})
		})

		Context("all etcd instances have quota-backend-bytes", func() {
			Context("all nodes have less than 24 Gb memory and quota-backend-bytes set to 2gb", func() {
				etcdQuota := gb(2)
				BeforeEach(func() {
					manifests := getManifests("control-plane-1", []testNode{
						{
							memory:   gb(16),
							etcQuota: etcdQuota,
							setTaint: true,
						},
						{
							memory:   gb(8),
							etcQuota: etcdQuota,
							setTaint: true,
						},
						{
							memory:   gb(16),
							etcQuota: etcdQuota,
							setTaint: true,
						},
					})

					JoinKubeResourcesAndSet(f, manifests...)

					f.RunHook()
				})

				It("does not change quota", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackendsWithMetric(f, etcdQuota)
				})

				It("clean all metrics in group", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertClearMetrics(f)
				})
			})

			Context("all nodes have 24 Gb memory and quota-backend-bytes set to 2gb", func() {
				etcdQuota := gb(2)
				BeforeEach(func() {
					manifests := getManifests("control-plane-1", []testNode{
						{
							memory:   gb(24),
							etcQuota: etcdQuota,
							setTaint: true,
						},
						{
							memory:   gb(24),
							etcQuota: etcdQuota,
							setTaint: true,
						},
						{
							memory:   gb(24),
							etcQuota: etcdQuota,
							setTaint: true,
						},
					})

					JoinKubeResourcesAndSet(f, manifests...)

					f.RunHook()
				})

				It("increases quota (set 3gb)", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackendsWithMetric(f, gb(3))
				})

				It("clean all metrics in group", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertClearMetrics(f)
				})
			})

			Context("one node have greater than another memory and quota-backend-bytes set to for another nodes", func() {
				etcdQuota := gb(3)
				BeforeEach(func() {
					manifests := getManifests("control-plane-1", []testNode{
						{
							memory:   gb(24),
							etcQuota: etcdQuota,
							setTaint: true,
						},
						{
							memory:   gb(32),
							etcQuota: etcdQuota,
							setTaint: true,
						},
						{
							memory:   gb(24),
							etcQuota: etcdQuota,
							setTaint: true,
						},
					})

					JoinKubeResourcesAndSet(f, manifests...)

					f.RunHook()
				})

				It("stay current quota backend", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackendsWithMetric(f, etcdQuota)
				})

				It("clean all metrics in group", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertClearMetrics(f)
				})
			})

			Context("all nodes less memory than need for apply current quota-backend-bytes (decrease node case)", func() {
				etcdQuota := gb(3)
				BeforeEach(func() {
					manifests := getManifests("control-plane-1", []testNode{
						{
							memory:   gb(8),
							etcQuota: etcdQuota,
							setTaint: true,
						},
						{
							memory:   gb(8),
							etcQuota: etcdQuota,
							setTaint: true,
						},
						{
							memory:   gb(8),
							etcQuota: etcdQuota,
							setTaint: true,
						},
					})

					JoinKubeResourcesAndSet(f, manifests...)

					f.RunHook()
				})

				It("do not decrease quota backends stay as is", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackendsWithMetric(f, etcdQuota)
				})

				It("add decrease error metric", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertAddErrorMetric(f)
				})
			})

			Context("etcd instances have non default quota, but have one not dedicated node", func() {
				etcdQuota := gb(3)
				BeforeEach(func() {
					manifests := getManifests("control-plane-1", []testNode{
						{
							memory:   gb(32),
							etcQuota: etcdQuota,
							setTaint: false,
						},
						{
							memory:   gb(32),
							etcQuota: etcdQuota,
							setTaint: true,
						},
						{
							memory:   gb(32),
							etcQuota: etcdQuota,
							setTaint: true,
						},
					})

					JoinKubeResourcesAndSet(f, manifests...)

					f.RunHook()
				})

				It("stay quota backends as is (from instances)", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackendsWithMetric(f, etcdQuota)
				})

				It("clean all metrics in group", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertClearMetrics(f)
				})
			})
		})
	})
})
