// Copyright 2021 Flant JSC
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
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: kubernetes_version ::", func() {
	versionHTTPClient = dependency.TestDC.GetHTTPClient()
	const (
		initValuesString           = `{"global": {"enabledModules": ["control-plane-manager"],"modulesImages": {}, "discovery":{}}}`
		globalValuesWithoutCPMYaml = `
modulesImages: {}
discovery: {}
`

		initConfigValuesString = `{}`

		initialVersion = "1.19.10"
		verToChange    = "1.20.1"
	)

	var (
		endpointsOne  = []string{"192.168.128.190"}
		endpointsMul  = []string{"192.168.128.190", "192.168.128.191", "192.168.128.192"}
		apiServerPods = []struct {
			title  string
			name   string
			labels map[string]string
		}{
			{
				title:  "Api-server k8s-app labeled",
				name:   "api-server-k8s",
				labels: apiServerK8sAppLabels(),
			},

			{
				title:  "Api-server control plane labeled",
				name:   "api-server-cp",
				labels: apiServerControlPlaneLabels(),
			},
		}
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	stateEndpoints := func(ips []string) string {
		var ipsStr string
		for _, ip := range ips {
			ipsStr = fmt.Sprintf("%s\n  - ip: %s", ipsStr, ip)
		}
		return fmt.Sprintf(`
---
apiVersion: v1
kind: Endpoints
metadata:
  labels:
    endpointslice.kubernetes.io/skip-mirror: "true"
  name: kubernetes
  namespace: default
subsets:
- addresses: %s
  ports:
  - name: https
    port: 6443
    protocol: TCP

`, ipsStr)
	}

	indexOf := func(t string, ss []string) int {
		for i, s := range ss {
			if s == t {
				return i
			}
		}

		return -1
	}

	versionsResponse := func(version string) *http.Response {
		b := fmt.Sprintf(`{
  "major": "1",
  "minor": "19",
  "gitVersion": "v%s",
  "gitCommit": "0000000000000000000000000000000000000000",
  "gitTreeState": "archive",
  "buildDate": "2021-07-21T15:25:03Z",
  "goVersion": "go1.15.3",
  "compiler": "gc",
  "platform": "linux/amd64"
}`, version)
		buf := bytes.NewBufferString(b)
		rc := io.NopCloser(buf)
		return &http.Response{
			Header:     map[string][]string{"Content-Type": {"application/json"}},
			StatusCode: http.StatusOK,
			Body:       rc,
		}
	}

	createAPIServerPod := func(name string, labels map[string]string) string {
		obj := &corev1.Pod{
			TypeMeta: v1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name:      name,
				Namespace: apiServerNs,
				Labels:    labels,
			},
		}

		str, err := yaml.Marshal(obj)
		if err != nil {
			panic(err)
		}

		return fmt.Sprintf("\n---%s\n", string(str))
	}

	createAPIServerPodsMultiple := func(count int, name string, labels map[string]string) (string, []string) {
		podsStateSlice := make([]string, 0)
		for i := 0; i < count; i++ {
			name := fmt.Sprintf("%s-%v", name, i)
			podsStateSlice = append(podsStateSlice, createAPIServerPod(name, labels))
		}
		podsState := strings.Join(podsStateSlice, "\n")

		return podsState, podsStateSlice
	}

	assertValues := func(k8sVer string, allVersions []string) {
		Expect(f.ValuesGet("global.discovery.kubernetesVersion").String()).To(Equal(k8sVer))
		Expect(f.ValuesGet("global.discovery.kubernetesVersions").AsStringSlice()).To(Equal(allVersions))
	}

	assertNoValues := func() {
		Expect(f.ValuesGet("global.discovery.kubernetesVersion").Exists()).To(BeFalse())
		Expect(f.ValuesGet("global.discovery.kubernetesVersions").Exists()).To(BeFalse())
	}

	assertNoFile := func() {
		_, err := os.ReadFile(kubeVersionFileName)
		Expect(os.IsNotExist(err)).To(BeTrue())
	}

	assertVersionInFile := func(k8sVer string) {
		content, err := os.ReadFile(kubeVersionFileName)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal(k8sVer))
	}

	AfterEach(func() {
		err := os.Remove(kubeVersionFileName)
		if err == nil || os.IsNotExist(err) {
			return
		}
		panic(err)
	})

	Context("Cluster is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		for _, s := range apiServerPods {
			podState := createAPIServerPod(s.name, s.labels)

			Context(fmt.Sprintf("%s created", s.title), func() {
				BeforeEach(func() {
					dependency.TestDC.HTTPClient.DoMock.
						Set(func(_ *http.Request) (rp1 *http.Response, err error) {
							return versionsResponse(initialVersion), nil
						})
					f.BindingContexts.Set(f.KubeStateSet(podState))
					f.RunHook()
				})

				It("does not set k8s version with versions array with one version into values", func() {
					Expect(f).ToNot(ExecuteSuccessfully())
					assertNoValues()
				})

				It("does not write k8s version into file", func() {
					Expect(f).ToNot(ExecuteSuccessfully())
					assertNoFile()
				})

			})
		}

		Context("Endpoint were created", func() {
			const initialVersion = "1.19.2"
			BeforeEach(func() {
				dependency.TestDC.HTTPClient.DoMock.
					Set(func(_ *http.Request) (rp1 *http.Response, err error) {
						return versionsResponse(initialVersion), nil
					})
				f.BindingContexts.Set(f.KubeStateSet(stateEndpoints(endpointsOne)))
				f.RunHook()
			})

			It("does not set k8s version with versions array with one version into values", func() {
				Expect(f).NotTo(ExecuteSuccessfully())
				assertNoValues()
			})

			It("does not write k8s version into file", func() {
				Expect(f).NotTo(ExecuteSuccessfully())
				assertNoFile()
			})

			Context("control plane manager is disabled", func() {
				BeforeEach(func() {
					f.ValuesSetFromYaml("global", []byte(globalValuesWithoutCPMYaml))

					f.RunHook()
				})

				It("sets k8s version with versions array with one version into values", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertValues(initialVersion, []string{initialVersion})
				})

				It("sets k8s version into file", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertVersionInFile(initialVersion)
				})
			})

		})

		state := `
---
apiVersion: v1
kind: Endpoints
metadata:
  labels:
    endpointslice.kubernetes.io/skip-mirror: "true"
  name: kubernetes
  namespace: default
subsets:
- addresses:
  - ip: 192.168.128.190
  ports:
  - name: https
    port: 6443
    protocol: TCP
---
apiVersion: v1
kind: Service
metadata:
  creationTimestamp: "2023-04-24T03:03:05Z"
  labels:
    component: apiserver
    provider: kubernetes
  name: kubernetes
  namespace: default
  resourceVersion: "190"
  uid: 96574aff-f522-4f99-bc77-69ec111051b5
spec:
  clusterIP: 10.245.0.1
  clusterIPs:
  - 10.245.0.1
  internalTrafficPolicy: Cluster
  ipFamilies:
  - IPv4
  ipFamilyPolicy: SingleStack
  ports:
  - name: https
    port: 443
    protocol: TCP
    targetPort: 443
  sessionAffinity: None
  type: ClusterIP
status:
  loadBalancer: {}
`

		Context("Endpoint unavailable", func() {
			const initialVersion = "1.19.2"
			BeforeEach(func() {
				dependency.TestDC.HTTPClient.DoMock.
					Set(func(req *http.Request) (rp1 *http.Response, err error) {
						switch req.Host {
						case "10.245.0.1":
							return versionsResponse(initialVersion), nil
						case "192.168.128.190:6443":
							return nil, errors.New("endpoint unavailable")
						}

						return nil, errors.New("not found")
					})
				f.BindingContexts.Set(f.KubeStateSet(state))
				f.RunHook()
			})

			It("does not set k8s version with versions array with one version into values", func() {
				Expect(f).NotTo(ExecuteSuccessfully())
				assertNoValues()
			})

			It("does not write k8s version into file", func() {
				Expect(f).NotTo(ExecuteSuccessfully())
				assertNoFile()
			})

			Context("control plane manager is disabled", func() {
				BeforeEach(func() {
					f.ValuesSetFromYaml("global", []byte(globalValuesWithoutCPMYaml))

					f.RunHook()
				})

				It("sets k8s version with versions array with one version into values", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertValues(initialVersion, []string{initialVersion})
				})

				It("sets k8s version into file", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertVersionInFile(initialVersion)
				})
			})
		})
	})

	Context("Endpoinds in cluster", func() {
		endpointsState := stateEndpoints(endpointsOne)
		BeforeEach(func() {
			dependency.TestDC.HTTPClient.DoMock.
				Set(func(_ *http.Request) (rp1 *http.Response, err error) {
					return versionsResponse(initialVersion), nil
				})
			f.BindingContexts.Set(f.KubeStateSet(endpointsState))
			f.RunHook()
		})

		It("does not set k8s version with versions array with one version into values", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			assertNoValues()
		})

		It("does not write k8s version into file", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			assertNoFile()
		})

		Context("control plane manager is disabled", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", []byte(globalValuesWithoutCPMYaml))

				f.RunHook()
			})

			It("sets k8s version with versions array with one version into values", func() {
				Expect(f).To(ExecuteSuccessfully())
				assertValues(initialVersion, []string{initialVersion})
			})

			It("sets k8s version into file", func() {
				Expect(f).To(ExecuteSuccessfully())
				assertVersionInFile(initialVersion)
			})

			Context("Change version", func() {
				BeforeEach(func() {
					dependency.TestDC.HTTPClient.DoMock.
						Set(func(_ *http.Request) (rp1 *http.Response, err error) {
							return versionsResponse(verToChange), nil
						})
					f.RunHook()
				})

				It("changes k8s version with versions array with one version into values", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertValues(verToChange, []string{verToChange})
				})

				It("changes k8s version into file", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertVersionInFile(verToChange)
				})
			})
		})

		for _, s := range apiServerPods {
			podState := createAPIServerPod(s.name, s.labels)

			Context(fmt.Sprintf("%s pod created", s.title), func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(endpointsState + podState))
					f.RunHook()
				})

				It("sets k8s version with versions array with one version into values", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertValues(initialVersion, []string{initialVersion})
				})

				It("sets k8s version into file", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertVersionInFile(initialVersion)
				})

				Context("Change version", func() {
					BeforeEach(func() {
						dependency.TestDC.HTTPClient.DoMock.
							Set(func(_ *http.Request) (rp1 *http.Response, err error) {
								return versionsResponse(verToChange), nil
							})
						f.RunHook()
					})

					It("changes k8s version with versions array with one version into values", func() {
						Expect(f).To(ExecuteSuccessfully())
						assertValues(verToChange, []string{verToChange})
					})

					It("changes k8s version into file", func() {
						Expect(f).To(ExecuteSuccessfully())
						assertVersionInFile(verToChange)
					})
				})
			})
		}

		Context("Api-server simple pod created", func() {
			podStatus := createAPIServerPod("simple", map[string]string{
				"simple": "simple",
			})

			BeforeEach(func() {
				dependency.TestDC.HTTPClient.DoMock.
					Set(func(_ *http.Request) (rp1 *http.Response, err error) {
						return versionsResponse(initialVersion), nil
					})

				f.BindingContexts.Set(f.KubeStateSet(endpointsState + podStatus))
				f.RunHook()
			})

			It("does not change k8s version with versions array with one version into values", func() {
				Expect(f).To(ExecuteSuccessfully())
				assertValues(initialVersion, []string{initialVersion})
			})

			It("does not change k8s version into file", func() {
				Expect(f).To(ExecuteSuccessfully())
				assertVersionInFile(initialVersion)
			})
		})
	})

	Context("Endpoint with multiple IP's in cluster", func() {
		endpoindsState := stateEndpoints(endpointsMul)
		initVers := []string{"1.19.5", "1.19.2", "1.20.4"}
		k8sVer := initVers[1]
		BeforeEach(func() {
			dependency.TestDC.HTTPClient.DoMock.
				Set(func(req *http.Request) (rp1 *http.Response, err error) {
					host := strings.Split(req.Host, ":")[0]
					ver := initVers[indexOf(host, endpointsMul)]
					return versionsResponse(ver), nil
				})
			f.BindingContexts.Set(f.KubeStateSet(endpoindsState))
			f.RunHook()
		})

		It("does not set k8s version with versions array with one version into values", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			assertNoValues()
		})

		It("does not write k8s version into file", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			assertNoFile()
		})

		Context("control plane manager is disabled", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", []byte(globalValuesWithoutCPMYaml))

				f.RunHook()
			})

			It("sets k8s version with versions array with one version into values", func() {
				Expect(f).To(ExecuteSuccessfully())
				assertValues(k8sVer, initVers)
			})

			It("sets k8s version into file", func() {
				Expect(f).To(ExecuteSuccessfully())
				assertVersionInFile(k8sVer)
			})

			Context("Change version", func() {
				changeVers := []string{"1.21.20", "1.19.4", "1.20.2"}
				k8sVer := changeVers[1]
				BeforeEach(func() {
					dependency.TestDC.HTTPClient.DoMock.
						Set(func(req *http.Request) (rp1 *http.Response, err error) {
							host := strings.Split(req.Host, ":")[0]
							ver := changeVers[indexOf(host, endpointsMul)]
							return versionsResponse(ver), nil
						})
					f.RunHook()
				})

				It("changes k8s version with versions array with one version into values", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertValues(k8sVer, changeVers)
				})

				It("changes k8s version into file", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertVersionInFile(k8sVer)
				})
			})
		})

		for _, s := range apiServerPods {
			podsState, _ := createAPIServerPodsMultiple(len(initVers), s.name, s.labels)

			Context(fmt.Sprintf("%s pod created", s.title), func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(endpoindsState + podsState))
					f.RunHook()
				})

				It("sets k8s version with versions array with one version into values", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertValues(k8sVer, initVers)
				})

				It("sets k8s version into file", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertVersionInFile(k8sVer)
				})

				Context("Change version", func() {
					changeVers := []string{"1.21.20", "1.19.4", "1.20.2"}
					k8sVer := changeVers[1]
					BeforeEach(func() {
						dependency.TestDC.HTTPClient.DoMock.
							Set(func(req *http.Request) (rp1 *http.Response, err error) {
								host := strings.Split(req.Host, ":")[0]
								ver := changeVers[indexOf(host, endpointsMul)]
								return versionsResponse(ver), nil
							})
						f.RunHook()
					})

					It("changes k8s version with versions array with one version into values", func() {
						Expect(f).To(ExecuteSuccessfully())
						assertValues(k8sVer, changeVers)
					})

					It("changes k8s version into file", func() {
						Expect(f).To(ExecuteSuccessfully())
						assertVersionInFile(k8sVer)
					})
				})
			})
		}

		Context("Api-server simple pod created", func() {
			podsState, _ := createAPIServerPodsMultiple(len(initVers), "simple", map[string]string{
				"simple": "simple",
			})

			BeforeEach(func() {
				Skip("Current test framework does not implement labels selector")

				f.BindingContexts.Set(f.KubeStateSet(endpoindsState + podsState))
				f.RunHook()
			})

			It("does not set k8s version with versions array with one version into values", func() {
				Expect(f).To(ExecuteSuccessfully())
				assertNoValues()
			})

			It("does not write k8s version into file", func() {
				Expect(f).To(ExecuteSuccessfully())
				assertNoFile()
			})
		})
	})

	Context("Remove objects", func() {
		initVers := []string{"1.21.20", "1.20.2", "1.19.4"}
		k8sVer := initVers[2]

		endpointsState := stateEndpoints(endpointsMul)

		BeforeEach(func() {
			dependency.TestDC.HTTPClient.DoMock.
				Set(func(req *http.Request) (rp1 *http.Response, err error) {
					host := strings.Split(req.Host, ":")[0]
					ver := initVers[indexOf(host, endpointsMul)]
					return versionsResponse(ver), nil
				})
		})

		Context("Remove endpoints", func() {
			Context("Remove all", func() {
				BeforeEach(func() {
					podsState, _ := createAPIServerPodsMultiple(len(initVers), "k8s-app", apiServerK8sAppLabels())
					f.BindingContexts.Set(f.KubeStateSet(endpointsState + podsState))
					f.RunHook()

					f.BindingContexts.Set(f.KubeStateSet(podsState))
					f.RunHook()
				})

				It("does not change k8s version with versions array with one version into values", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertValues(k8sVer, initVers)
				})

				It("does not change k8s version into file", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertVersionInFile(k8sVer)
				})
			})

			Context("Remove not all", func() {
				BeforeEach(func() {
					podsState, _ := createAPIServerPodsMultiple(len(initVers), "k8s-app", apiServerK8sAppLabels())
					f.BindingContexts.Set(f.KubeStateSet(endpointsState + podsState))
					f.RunHook()

					f.BindingContexts.Set(f.KubeStateSet(stateEndpoints(endpointsOne) + podsState))
					f.RunHook()
				})

				It("does not change k8s version with versions array with one version into values", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertValues(k8sVer, initVers)
				})

				It("does not change k8s version into file", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertVersionInFile(k8sVer)
				})
			})
		})

		for _, s := range apiServerPods {
			Context(fmt.Sprintf("Remove %s pod", s.title), func() {
				BeforeEach(func() {
					podsState, _ := createAPIServerPodsMultiple(len(initVers), s.name, s.labels)
					f.BindingContexts.Set(f.KubeStateSet(podsState + endpointsState))
					f.RunHook()

					podsState, _ = createAPIServerPodsMultiple(len(initVers)-1, s.name, s.labels)
					f.BindingContexts.Set(f.KubeStateSet(podsState + endpointsState))
					f.RunHook()
				})

				It("does not change k8s version with versions array with one version into values", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertValues(k8sVer, initVers)
				})

				It("does not change k8s version into file", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertVersionInFile(k8sVer)
				})
			})
		}

		Context("Api-server simple pod", func() {
			BeforeEach(func() {
				podsState, _ := createAPIServerPodsMultiple(len(initVers), "simple", map[string]string{
					"simple": "simple",
				})
				f.BindingContexts.Set(f.KubeStateSet(podsState + endpointsState))
				f.RunHook()
			})

			Context("Removing", func() {
				BeforeEach(func() {
					podsState, _ := createAPIServerPodsMultiple(len(initVers)-1, "simple", map[string]string{
						"simple": "simple",
					})
					f.BindingContexts.Set(f.KubeStateSet(podsState + endpointsState))
					f.RunHook()
				})

				It("does not set k8s version with versions array with one version into values", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertNoValues()
				})

				It("does not write k8s version into file", func() {
					Expect(f).To(ExecuteSuccessfully())
					assertNoFile()
				})
			})
		})
	})
})
