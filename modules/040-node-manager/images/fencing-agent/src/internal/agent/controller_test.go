/*
Copyright 2024 Flant JSC

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

package agent_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"fencing-controller/internal/agent"
	"fencing-controller/internal/watchdog/fakedog"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const testNodeName = "test-node"

func wdEventsCount(wdEvents *[]byte) map[byte]int {
	m := make(map[byte]int)
	for _, v := range *wdEvents {
		m[v]++
	}
	return m
}

var _ = Describe("FencingAgent", func() {

	var (
		fakeKubeClient *fake.Clientset
		ctx            context.Context
		cancel         context.CancelFunc
		logger         *zap.Logger
	)

	BeforeEach(func() {
		fakeKubeClient = fake.NewSimpleClientset()
		logger, _ = zap.NewDevelopment()
		ctx, cancel = context.WithCancel(context.Background())
	})

	Describe("Run", func() {

		ginkgo.Context("Maintenance annotation on the node is absent", func() {
			ginkgo.It("should return no error", func() {
				var fakeWDEvents []byte
				wd := fakedog.NewWatchdog(&fakeWDEvents)
				fa := agent.NewFencingAgent(logger, agent.Config{NodeName: testNodeName, KubernetesAPICheckInterval: 2 * time.Second, HealthProbeBindAddress: ""}, fakeKubeClient, wd)
				node := v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:        strings.ToLower(testNodeName),
						Labels:      map[string]string{},
						Annotations: map[string]string{},
					},
				}

				_, err := fakeKubeClient.CoreV1().Nodes().Create(ctx, &node, metav1.CreateOptions{})
				if err != nil {
					return
				}

				go func() {
					time.Sleep(5 * time.Second)
					cancel()
				}()

				err = fa.Run(ctx)
				Expect(err).To(BeNil())

				By("First event should be 0 (feed watchdog)")
				Expect(fakeWDEvents[0]).To(BeEquivalentTo(0))

				By("Last event should be 1 (close watchdog)")
				Expect(fakeWDEvents[len(fakeWDEvents)-1]).To(BeEquivalentTo(1))

				By("There should only be one watchdog closing event")
				wdStat := wdEventsCount(&fakeWDEvents)
				Expect(wdStat[1]).To(BeEquivalentTo(1))
			})
		})

		Context("Maintenance annotation on the node is present", func() {
			It("should return no error", func() {
				var fakeWDEvents []byte
				wd := fakedog.NewWatchdog(&fakeWDEvents)
				fa := agent.NewFencingAgent(logger, agent.Config{NodeName: testNodeName, KubernetesAPICheckInterval: 2 * time.Second, HealthProbeBindAddress: ""}, fakeKubeClient, wd)
				node := v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:   strings.ToLower(testNodeName),
						Labels: map[string]string{},
						Annotations: map[string]string{
							"update.node.deckhouse.io/disruption-approved": "",
						},
					},
				}

				_, err := fakeKubeClient.CoreV1().Nodes().Create(ctx, &node, metav1.CreateOptions{})
				if err != nil {
					return
				}

				go func() {
					time.Sleep(5 * time.Second)

					cancel()
				}()

				err = fa.Run(ctx)
				Expect(err).To(BeNil())

				By("Watchdog feeding should not occur")
				Expect(len(fakeWDEvents)).To(BeEquivalentTo(0))
			})
		})

		Context("Maintenance annotation on the node added and removed in runtime", func() {
			It("should return no error", func() {
				var fakeWDEvents []byte
				wd := fakedog.NewWatchdog(&fakeWDEvents)
				fa := agent.NewFencingAgent(logger, agent.Config{NodeName: testNodeName, KubernetesAPICheckInterval: 2 * time.Second, HealthProbeBindAddress: ""}, fakeKubeClient, wd)
				node := v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:        strings.ToLower(testNodeName),
						Labels:      map[string]string{},
						Annotations: map[string]string{},
					},
				}

				_, err := fakeKubeClient.CoreV1().Nodes().Create(ctx, &node, metav1.CreateOptions{})
				if err != nil {
					return
				}

				go func() {
					time.Sleep(5 * time.Second)
					existingNode, _ := fakeKubeClient.CoreV1().Nodes().Get(ctx, testNodeName, metav1.GetOptions{})
					existingNode.Annotations["update.node.deckhouse.io/disruption-approved"] = ""
					_, err = fakeKubeClient.CoreV1().Nodes().Update(ctx, existingNode, metav1.UpdateOptions{})

					time.Sleep(5 * time.Second)
					delete(existingNode.Annotations, "update.node.deckhouse.io/disruption-approved")
					_, err = fakeKubeClient.CoreV1().Nodes().Update(ctx, existingNode, metav1.UpdateOptions{})

					time.Sleep(5 * time.Second)
					cancel()
				}()

				err = fa.Run(ctx)
				Expect(err).To(BeNil())

				By("First event should be 0 (feed watchdog)")
				Expect(fakeWDEvents[0]).To(BeEquivalentTo(0))

				By("Last event should be 1 (close watchdog)")
				Expect(fakeWDEvents[len(fakeWDEvents)-1]).To(BeEquivalentTo(1))

				// WD Armed --> WD Disarmed --> WD Armed --> WD Disarmed
				By("There should only be two watchdog closing event")
				wdStat := wdEventsCount(&fakeWDEvents)
				Expect(wdStat[1]).To(BeEquivalentTo(2))
			})
		})

	})
})

func TestFencingAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FencingAgent Suite")
}
