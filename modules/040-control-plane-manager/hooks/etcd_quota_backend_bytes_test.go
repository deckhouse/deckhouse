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
				nodes: []go_hook.FilterResult{{
					memory:      gb(8),
					isDedicated: false,
				}},
				expectedNode: &etcdNode{
					memory:      gb(8),
					isDedicated: false,
				},
			},

			{
				title: "For all different nodes return with minimal memory",
				nodes: []go_hook.FilterResult{
					{
						memory:      gb(8),
						isDedicated: false,
					},
					{
						memory:      gb(12),
						isDedicated: false,
					},
				},

				expectedNode: &etcdNode{
					memory:      gb(8),
					isDedicated: false,
				},
			},

			{
				title: "For all different nodes return with minimal memory",
				nodes: []go_hook.FilterResult{
					{
						memory:      gb(16),
						isDedicated: false,
					},
					{
						memory:      gb(8),
						isDedicated: false,
					},
					{
						memory:      gb(12),
						isDedicated: false,
					},
				},

				expectedNode: &etcdNode{
					memory:      gb(8),
					isDedicated: false,
				},
			},

			{
				title: "For all different nodes return with minimal memory",
				nodes: []go_hook.FilterResult{
					{
						memory:      gb(16),
						isDedicated: false,
					},
					{
						memory:      gb(12),
						isDedicated: false,
					},
					{
						memory:      gb(8),
						isDedicated: false,
					},
				},

				expectedNode: &etcdNode{
					memory:      gb(8),
					isDedicated: false,
				},
			},

			{
				title: "For all different nodes return with minimal memory",
				nodes: []go_hook.FilterResult{
					{
						memory:      gb(8),
						isDedicated: false,
					},
					{
						memory:      gb(12),
						isDedicated: false,
					},
					{
						memory:      gb(16),
						isDedicated: false,
					},
				},

				expectedNode: &etcdNode{
					memory:      gb(8),
					isDedicated: false,
				},
			},

			{
				title: "If have dedicated node, return dedicated node",
				nodes: []go_hook.FilterResult{
					{
						memory:      gb(8),
						isDedicated: false,
					},
					{
						memory:      gb(12),
						isDedicated: true,
					},
					{
						memory:      gb(16),
						isDedicated: false,
					},
				},

				expectedNode: &etcdNode{
					memory:      gb(12),
					isDedicated: true,
				},
			},

			{
				title: "If have two dedicated nodes, return first dedicated node",
				nodes: []go_hook.FilterResult{
					{
						memory:      gb(8),
						isDedicated: false,
					},
					{
						memory:      gb(12),
						isDedicated: true,
					},
					{
						memory:      gb(16),
						isDedicated: true,
					},
				},

				expectedNode: &etcdNode{
					memory:      gb(12),
					isDedicated: true,
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

	_ = HookExecutionConfigInit(initValuesString, "")

	Context("Single master", func() {
		Context("etcd does not have quota-backend-bytes parameter", func() {
			Context("etcd db size is 0 bytes", func() {

			})

			Context("etcd has quota-backend-bytes parameter", func() {
				_ = func(data map[string]interface{}) string {
					data["maxDbSize"] = 4 * 1024 * 1024 * 1024
					return etcdPodManifest(data)
				}

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
