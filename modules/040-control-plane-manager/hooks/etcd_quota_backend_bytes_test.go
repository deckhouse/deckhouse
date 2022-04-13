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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = FDescribe("Modules :: controler-plane-manager :: hooks :: etcd-quota-backend-bytes ::", func() {
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
				nodeSize: gb(24),
				newQuota: gb(3),
			},

			{
				nodeSize: gb(32),
				newQuota: gb(4),
			},

			{
				nodeSize: gb(40),
				newQuota: gb(5),
			},

			{
				nodeSize: gb(48),
				newQuota: gb(6),
			},

			{
				nodeSize: gb(56),
				newQuota: gb(7),
			},

			{
				nodeSize: gb(64),
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
			c := c
			It(fmt.Sprintf("Node size %d", c.nodeSize/1024/1024/1024), func() {
				newQuota := calcNewQuota(c.nodeSize)

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
						memory:      gb(8),
						isDedicated: true,
					}},
				expectedNode: &etcdNode{
					memory:      gb(8),
					isDedicated: true,
				},
			},

			{
				title: "For all different nodes return with minimal memory",
				nodes: []go_hook.FilterResult{
					&etcdNode{
						memory:      gb(8),
						isDedicated: true,
					},
					&etcdNode{
						memory:      gb(12),
						isDedicated: true,
					},
				},

				expectedNode: &etcdNode{
					memory:      gb(8),
					isDedicated: true,
				},
			},

			{
				title: "For all different nodes return with minimal memory",
				nodes: []go_hook.FilterResult{
					&etcdNode{
						memory:      gb(16),
						isDedicated: true,
					},
					&etcdNode{
						memory:      gb(8),
						isDedicated: true,
					},
					&etcdNode{
						memory:      gb(12),
						isDedicated: true,
					},
				},

				expectedNode: &etcdNode{
					memory:      gb(8),
					isDedicated: true,
				},
			},

			{
				title: "For all different nodes return with minimal memory",
				nodes: []go_hook.FilterResult{
					&etcdNode{
						memory:      gb(16),
						isDedicated: true,
					},
					&etcdNode{
						memory:      gb(12),
						isDedicated: true,
					},
					&etcdNode{
						memory:      gb(8),
						isDedicated: true,
					},
				},

				expectedNode: &etcdNode{
					memory:      gb(8),
					isDedicated: true,
				},
			},

			{
				title: "For all different nodes return with minimal memory",
				nodes: []go_hook.FilterResult{
					&etcdNode{
						memory:      gb(8),
						isDedicated: true,
					},
					&etcdNode{
						memory:      gb(12),
						isDedicated: true,
					},
					&etcdNode{
						memory:      gb(16),
						isDedicated: true,
					},
				},

				expectedNode: &etcdNode{
					memory:      gb(8),
					isDedicated: true,
				},
			},

			{
				title: "If have dedicated node, return dedicated node",
				nodes: []go_hook.FilterResult{
					&etcdNode{
						memory:      gb(8),
						isDedicated: true,
					},
					&etcdNode{
						memory:      gb(12),
						isDedicated: false,
					},
					&etcdNode{
						memory:      gb(16),
						isDedicated: true,
					},
				},

				expectedNode: &etcdNode{
					memory:      gb(12),
					isDedicated: false,
				},
			},

			{
				title: "If have two dedicated nodes, return first dedicated node",
				nodes: []go_hook.FilterResult{
					&etcdNode{
						memory:      gb(8),
						isDedicated: true,
					},
					&etcdNode{
						memory:      gb(12),
						isDedicated: false,
					},
					&etcdNode{
						memory:      gb(16),
						isDedicated: false,
					},
				},

				expectedNode: &etcdNode{
					memory:      gb(12),
					isDedicated: false,
				},
			},
		}

		for _, c := range cases {
			c := c
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
    key: node-role.kubernetes.io/master
`
		}
		return fmt.Sprintf(`
apiVersion: v1
kind: Node
metadata:
  name: %s
  labels:
    node-role.kubernetes.io/master: ""
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

	assertNewQuotaBackends := func(f *HookExecutionConfig, newSize int64) {
		size := f.ValuesGet("controlPlaneManager.internal.etcdQuotaBackendBytes").Int()
		Expect(size).To(Equal(newSize))
	}

	assertClearMetrics := func(f *HookExecutionConfig) {
		metrics := f.MetricsCollector.CollectedMetrics()

		Expect(metrics).To(HaveLen(1))

		Expect(metrics[0].Action).To(Equal("expire"))
		Expect(metrics[0].Group).To(Equal(etcdBackendBytesGroup))
	}

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

					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest, nodeManifest)

					f.RunHook()
				})

				It("set default backend size as quota-backend-bytes", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackends(f, gb(2))
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

					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest, nodeManifest)

					f.RunHook()
				})

				It("set increase on 1 gb of default size as quota-backend-bytes", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackends(f, gb(3))
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

					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest, nodeManifest)

					f.RunHook()
				})

				It("set maximum size for quota-backend-bytes", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackends(f, gb(8))
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

					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest, nodeManifest)

					f.RunHook()
				})

				It("set default backend size as quota-backend-bytes", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackends(f, gb(2))
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

					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest, nodeManifest)

					f.RunHook()
				})

				It("does not change quota-backend-bytes", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackends(f, gb(2))
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

					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest, nodeManifest)

					f.RunHook()
				})

				It("increase on 1 gb of default size as quota-backend-bytes", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackends(f, gb(3))
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

					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest, nodeManifest)

					f.RunHook()
				})

				It("set maximum size for quota-backend-bytes", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackends(f, gb(8))
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

					JoinKubeResourcesAndSet(f, testETCDSecret, podManifest, nodeManifest)

					f.RunHook()
				})

				It("set current backend size", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertNewQuotaBackends(f, quotaBackend)
				})

				It("clean all metrics in group", func() {
					Expect(f).Should(ExecuteSuccessfully())

					assertClearMetrics(f)
				})
			})
		})

		Context("Multi-master", func() {
			_ = func(ips []string, namePrefix, nodePrefix string) []string {
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

			})
		})
	})
})
