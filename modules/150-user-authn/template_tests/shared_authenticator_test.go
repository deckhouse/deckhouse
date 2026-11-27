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

package template_tests

import (
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: user-authn :: helm template :: shared dex authenticator", func() {
	hec := SetupHelmConfig("")

	const namespace = "d8-user-authn"

	instances := []string{"dex-authenticator-system", "dex-authenticator"}

	setValues := func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.33.1")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler", "operator-prometheus"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", "plainstring")

		hec.ValuesSetFromYaml("userAuthn.internal.sharedDexAuthenticator.redisPasswords", `
dex-authenticator-system: dh-test-secret-0123456789abcdef0
dex-authenticator: dh-test-secret-0123456789abcdef1
`)
	}

	// containerArgs returns the arguments of a container of the Deployment of an instance.
	containerArgs := func(instance, container string) []string {
		deployment := hec.KubernetesResource("Deployment", namespace, instance)
		Expect(deployment.Exists()).To(BeTrue(), instance)
		args := []string{}
		for _, c := range deployment.Field("spec.template.spec.containers").Array() {
			if c.Get("name").String() != container {
				continue
			}
			for _, arg := range c.Get("args").Array() {
				args = append(args, arg.String())
			}
		}

		return args
	}

	Context("without the password of the Redis of an instance", func() {
		for name, passwords := range map[string]string{
			"one password is missing": `{"dex-authenticator-system": "dh-test-secret-0123456789abcdef0"}`,
			"one password is empty":   `{"dex-authenticator-system": "dh-test-secret-0123456789abcdef0", "dex-authenticator": ""}`,
		} {
			Context(name, func() {
				BeforeEach(func() {
					setValues()
					hec.ValuesSetFromYaml("userAuthn.internal.sharedDexAuthenticator.redisPasswords", passwords)
					hec.HelmRender()
				})

				It("fails the render and names the value", func() {
					Expect(hec.RenderError).Should(HaveOccurred())
					Expect(hec.RenderError.Error()).To(ContainSubstring("userAuthn.internal.sharedDexAuthenticator.redisPasswords.dex-authenticator"))
				})
			})
		}
	})

	Context("before the hooks have filled the passwords", func() {
		BeforeEach(func() {
			setValues()
			hec.ValuesDelete("userAuthn.internal.sharedDexAuthenticator.redisPasswords")
			hec.HelmRender()
		})

		// addon-operator renders the whole chart for the ConversionWebhooks of the module ahead of
		// the release, with the values the module has before its hooks run: a render that fails
		// there stops the main queue.
		It("renders the chart", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			Expect(hec.KubernetesResource("Secret", namespace, "dex-authenticator-redis").Exists()).To(BeTrue())
		})
	})

	Context("with the Redis passwords of the instances", func() {
		BeforeEach(func() {
			setValues()
		})

		for _, highAvailability := range []bool{false, true} {
			Context("high availability "+map[bool]string{false: "off", true: "on"}[highAvailability], func() {
				BeforeEach(func() {
					hec.ValuesSet("global.highAvailability", highAvailability)
					hec.HelmRender()
				})

				It("runs two replicas of each instance on two nodes where there are two", func() {
					Expect(hec.RenderError).ShouldNot(HaveOccurred())

					for _, instance := range instances {
						deployment := hec.KubernetesResource("Deployment", namespace, instance)
						Expect(deployment.Exists()).To(BeTrue(), instance)
						Expect(deployment.Field("spec.replicas").Int()).To(BeEquivalentTo(2))

						Expect(deployment.Field("spec.template.spec.affinity.podAntiAffinity").Exists()).To(BeFalse(),
							"a required anti-affinity would not fit a cluster of one node, a preferred one does not spread")
						spread := deployment.Field("spec.template.spec.topologySpreadConstraints").Array()
						Expect(spread).To(HaveLen(1))
						Expect(spread[0].Get("topologyKey").String()).To(Equal("kubernetes.io/hostname"))
						Expect(spread[0].Get("maxSkew").Int()).To(BeEquivalentTo(1))
						Expect(spread[0].Get("whenUnsatisfiable").String()).To(Equal("DoNotSchedule"))
						Expect(spread[0].Get("nodeTaintsPolicy").String()).To(Equal("Honor"),
							"a cordoned node is no domain that the second replica has to go to")
						Expect(spread[0].Get("minDomains").Exists()).To(BeFalse(), "one node is enough")
						Expect(spread[0].Get("labelSelector.matchLabels.app").String()).To(Equal("shared-dex-authenticator"))
						Expect(spread[0].Get("labelSelector.matchLabels.instance").String()).To(Equal(instance))
						Expect(spread[0].Get("matchLabelKeys").Array()).To(HaveLen(1))
						Expect(spread[0].Get("matchLabelKeys.0").String()).To(Equal("pod-template-hash"),
							"each revision is spread on its own")

						pdb := hec.KubernetesResource("PodDisruptionBudget", namespace, instance)
						Expect(pdb.Exists()).To(BeTrue())
						Expect(pdb.Field("spec.minAvailable").Int()).To(BeEquivalentTo(1))
						Expect(pdb.Field("spec.selector.matchLabels.instance").String()).To(Equal(instance))
					}
				})
			})
		}

		Context("rendered", func() {
			BeforeEach(func() {
				hec.HelmRender()
			})

			It("gives each instance its own pods, Services and table", func() {
				Expect(hec.RenderError).ShouldNot(HaveOccurred())

				for _, instance := range instances {
					deployment := hec.KubernetesResource("Deployment", namespace, instance)
					labels := deployment.Field("spec.template.metadata.labels")
					Expect(labels.Get("app").String()).To(Equal("shared-dex-authenticator"))
					Expect(labels.Get("instance").String()).To(Equal(instance))
					Expect(deployment.Field("spec.selector.matchLabels.instance").String()).To(Equal(instance))

					service := hec.KubernetesResource("Service", namespace, instance)
					Expect(service.Field("spec.selector.instance").String()).To(Equal(instance))
					Expect(service.Field("spec.ports.0.port").Int()).To(BeEquivalentTo(443))

					// The neighbours are found through a headless Service that lists the pods that are
					// not ready too: a pod fills its Redis from a neighbour before it is ready.
					peers := hec.KubernetesResource("Service", namespace, instance+"-redis")
					Expect(peers.Field("spec.clusterIP").String()).To(Equal("None"))
					Expect(peers.Field("spec.publishNotReadyAddresses").Bool()).To(BeTrue())
					Expect(peers.Field("spec.selector.instance").String()).To(Equal(instance))

					// The table of clients is optional: the pods start before the controller writes it.
					var table string
					for _, volume := range deployment.Field("spec.template.spec.volumes").Array() {
						if volume.Get("name").String() == "client-table" {
							Expect(volume.Get("secret.optional").Bool()).To(BeTrue())
							table = volume.Get("secret.secretName").String()
						}
					}
					Expect(table).To(Equal(instance + "-clients"))
				}
			})

			It("starts the proxy with the table of clients and the store over the Redis of the replicas", func() {
				Expect(hec.RenderError).ShouldNot(HaveOccurred())

				for _, instance := range instances {
					args := containerArgs(instance, "dex-authenticator")
					Expect(args).To(ContainElements(
						"--client-table=/opt/dex-authenticator/client-table/clients.json",
						"--session-store-type=redis-peers",
						"--redis-connection-url=redis://127.0.0.1:6379/",
						"--redis-peers-dns-name="+instance+"-redis.d8-user-authn",
						"--redis-peers-self-address=$(POD_IP)",
						"--cookie-encrypt-refresh-token",
						"--skip-oidc-discovery",
						"--reverse-proxy",
						"--logging-format=json",
						"--scope=groups email openid profile offline_access",
					))
					for _, arg := range args {
						Expect(arg).NotTo(HavePrefix("--client-id"), "a client is set by the table, not by a flag")
						Expect(arg).NotTo(HavePrefix("--whitelist-domain"))
					}
				}
			})

			It("passes the proxy only the flags the proxy of the image has", func() {
				flags, err := proxyFlags()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(flags).To(HaveKey("cookie-refresh"), "the list of the upstream flags")
				Expect(flags).To(HaveKey("client-table"), "the flags of the patches")

				// Every branch of the template that adds a flag.
				variants := map[string]func(){
					"defaults": func() {},
					"the revocation feed, the sessions of dex and a custom logo": func() {
						hec.ValuesSet("userAuthn.internal.revocationFeedTLS.ca", "plainstring")
						hec.ValuesSet("userAuthn.sessions.enabled", true)
						hec.ValuesSet("userAuthn.internal.customLogo.enabled", true)
					},
					"without HTTPS": func() {
						hec.ValuesSet("global.modules.https.mode", "Disabled")
					},
				}
				seen := map[string]struct{}{}
				for variant, set := range variants {
					setValues()
					set()
					hec.HelmRender()
					Expect(hec.RenderError).ShouldNot(HaveOccurred(), variant)

					for _, instance := range instances {
						args := containerArgs(instance, "dex-authenticator")
						Expect(len(args)).To(BeNumerically(">", 20), "%s, %s", variant, instance)
						for _, arg := range args {
							name, ok := flagName(arg)
							Expect(ok).To(BeTrue(), "%s, %s: the argument %q is no flag", variant, instance, arg)
							Expect(flags).To(HaveKey(name), "%s, %s: the proxy has no flag --%s: no patch of the image defines it", variant, instance, name)
							seen[name] = struct{}{}
						}
					}
				}
				// The branches were really taken.
				for _, name := range []string{"cookie-encrypt-refresh-token", "revocation-feed-token-file", "provider-logout-url", "restart-login-without-csrf-cookie", "cookie-secure"} {
					Expect(seen).To(HaveKey(name))
				}
			})

			It("subscribes to the revocation feed once per pod, with a token issued for the feed only", func() {
				hec.ValuesSet("userAuthn.internal.revocationFeedTLS.ca", "plainstring")
				hec.HelmRender()
				Expect(hec.RenderError).ShouldNot(HaveOccurred())

				for _, instance := range instances {
					args := containerArgs(instance, "dex-authenticator")
					Expect(args).To(ContainElements(
						"--revocation-feed-url=https://revocation-feed.d8-user-authn/revocations",
						"--revocation-feed-ca-file=/opt/dex-authenticator/revocation-feed-ca/ca.crt",
						"--revocation-feed-token-file=/opt/dex-authenticator/revocation-feed-token/token",
					))
					Expect(args).NotTo(ContainElement("--revocation-feed-disable"))

					deployment := hec.KubernetesResource("Deployment", namespace, instance)
					Expect(deployment.Field("spec.template.spec.serviceAccountName").String()).To(Equal(instance))
					Expect(deployment.Field("spec.template.spec.automountServiceAccountToken").Bool()).To(BeFalse(),
						"the pod has no token for the API server")
					var audience string
					for _, volume := range deployment.Field("spec.template.spec.volumes").Array() {
						if volume.Get("name").String() == "revocation-feed-token" {
							audience = volume.Get("projected.sources.0.serviceAccountToken.audience").String()
						}
					}
					Expect(audience).To(Equal("revocation-feed.d8-user-authn"))
				}
			})

			It("runs without the feed while the feed has no CA", func() {
				Expect(hec.RenderError).ShouldNot(HaveOccurred())

				for _, instance := range instances {
					for _, arg := range containerArgs(instance, "dex-authenticator") {
						Expect(arg).NotTo(HavePrefix("--revocation-feed"))
					}
				}
			})

			It("signs out through dex only while the sessions of dex are on", func() {
				for _, instance := range instances {
					args := strings.Join(containerArgs(instance, "dex-authenticator"), " ")
					Expect(args).NotTo(ContainSubstring("--provider-logout-url"), instance)
					Expect(args).NotTo(ContainSubstring("--restart-login-without-csrf-cookie"), instance)
				}

				hec.ValuesSet("userAuthn.sessions.enabled", true)
				hec.HelmRender()
				Expect(hec.RenderError).ShouldNot(HaveOccurred())
				for _, instance := range instances {
					args := containerArgs(instance, "dex-authenticator")
					// The table of the instance names only the clients dex knows the address to return to of.
					Expect(args).To(ContainElement("--provider-logout-url=https://dex.example.com/logout"), instance)
					Expect(args).To(ContainElement("--restart-login-without-csrf-cookie"), instance)
				}
			})

			It("follows the ID token lifetime of the module with the refresh period", func() {
				hec.ValuesSet("userAuthn.idTokenTTL", "2m")
				hec.HelmRender()
				Expect(hec.RenderError).ShouldNot(HaveOccurred())

				for _, instance := range instances {
					Expect(containerArgs(instance, "dex-authenticator")).To(ContainElement("--cookie-refresh=2m"))
				}
			})

			It("keeps the Redis password out of the arguments of the containers", func() {
				Expect(hec.RenderError).ShouldNot(HaveOccurred())

				for i, instance := range instances {
					password := []string{"dh-test-secret-0123456789abcdef0", "dh-test-secret-0123456789abcdef1"}[i]

					secret := hec.KubernetesResource("Secret", namespace, instance+"-redis")
					Expect(secret.Exists()).To(BeTrue())
					Expect(secret.Field("data.password").String()).NotTo(BeEmpty())

					deployment := hec.KubernetesResource("Deployment", namespace, instance)
					Expect(deployment.ToYaml()).NotTo(ContainSubstring(password))
					for _, container := range []string{"dex-authenticator", "redis"} {
						for _, arg := range containerArgs(instance, container) {
							Expect(strings.Contains(arg, "password") || strings.Contains(arg, "requirepass")).To(BeFalse(), arg)
						}
					}
					Expect(containerArgs(instance, "redis")).To(Equal([]string{"/etc/redis/redis.conf"}))
				}
			})

			It("gives Redis the CPU it needs on a busy node and checks that it takes connections", func() {
				Expect(hec.RenderError).ShouldNot(HaveOccurred())

				for _, instance := range instances {
					deployment := hec.KubernetesResource("Deployment", namespace, instance)
					var redis gjson.Result
					for _, c := range deployment.Field("spec.template.spec.containers").Array() {
						if c.Get("name").String() == "redis" {
							redis = c
						}
					}
					Expect(redis.Exists()).To(BeTrue(), instance)
					Expect(redis.Get("resources.requests.cpu").String()).To(Equal("100m"))
					Expect(redis.Get("resources.requests.memory").String()).To(Equal("64Mi"))
					Expect(redis.Get("resources.limits").Exists()).To(BeFalse(),
						"a CPU limit throttles Redis under load, a memory limit kills it with every session")
					Expect(redis.Get("livenessProbe.tcpSocket.port").String()).To(Equal("redis"))
					Expect(redis.Get("livenessProbe.failureThreshold").Int()).To(BeEquivalentTo(6))
					Expect(redis.Get("readinessProbe").Exists()).To(BeFalse(),
						"the proxy serves through the Redis of its neighbour while its own one is away")

					vpa := hec.KubernetesResource("VerticalPodAutoscaler", namespace, instance)
					Expect(vpa.Exists()).To(BeTrue(), instance)
					var floor gjson.Result
					for _, policy := range vpa.Field("spec.resourcePolicy.containerPolicies").Array() {
						if policy.Get("containerName").String() == "redis" {
							floor = policy.Get("minAllowed")
						}
					}
					Expect(floor.Get("cpu").String()).To(Equal("100m"), "the VPA does not take the CPU of Redis below the request")
					Expect(floor.Get("memory").String()).To(Equal("64Mi"))
				}
			})

			It("lets only the pods of the instance reach its Redis", func() {
				Expect(hec.RenderError).ShouldNot(HaveOccurred())

				for _, instance := range instances {
					policy := hec.KubernetesResource("NetworkPolicy", namespace, instance)
					Expect(policy.Exists()).To(BeTrue())
					Expect(policy.Field("spec.podSelector.matchLabels.instance").String()).To(Equal(instance))

					redisRules := 0
					for _, rule := range policy.Field("spec.ingress").Array() {
						for _, port := range rule.Get("ports").Array() {
							if port.Get("port").Int() != 6379 {
								continue
							}
							redisRules++
							from := rule.Get("from").Array()
							Expect(from).To(HaveLen(1))
							Expect(from[0].Get("podSelector.matchLabels.instance").String()).To(Equal(instance))
							Expect(from[0].Get("namespaceSelector").Exists()).To(BeFalse(), "the pods of this namespace only")
						}
					}
					Expect(redisRules).To(Equal(1))
				}
			})

			It("gives VPA no ceiling of a single small application", func() {
				Expect(hec.RenderError).ShouldNot(HaveOccurred())

				for _, instance := range instances {
					vpa := hec.KubernetesResource("VerticalPodAutoscaler", namespace, instance)
					Expect(vpa.Exists()).To(BeTrue())
					for _, policy := range vpa.Field("spec.resourcePolicy.containerPolicies").Array() {
						Expect(policy.Get("maxAllowed.cpu").String()).NotTo(Equal("20m"))
						Expect(policy.Get("maxAllowed.memory").String()).NotTo(Equal("50Mi"))
					}
				}
			})
		})
	})
})
