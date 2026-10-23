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
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const (
	dexCA            = "-----BEGIN CERTIFICATE-----\ndex-ca\n-----END CERTIFICATE-----\n"
	revocationFeedCA = "-----BEGIN CERTIFICATE-----\nrevocation-feed-ca\n-----END CERTIFICATE-----\n"
)

func authenticatorArgs(hec *Config, namespace, name string) []string {
	deployment := hec.KubernetesResource("Deployment", namespace, name)
	ExpectWithOffset(1, deployment.Exists()).To(BeTrue())
	args := []string{}
	for _, arg := range deployment.Field(`spec.template.spec.containers.#(name=="dex-authenticator").args`).Array() {
		args = append(args, arg.String())
	}
	return args
}

var _ = Describe("Module :: user-authn :: helm template :: DexAuthenticator oauth2-proxy", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.29.1")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")
		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", dexCA)
		hec.ValuesSet("global.discovery.gatewayAPIDefaultGateway.name", "shared-gateway")
		hec.ValuesSet("global.discovery.gatewayAPIDefaultGateway.namespace", "d8-alb")
		hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", `
- name: app
  encodedName: appEncoded
  namespace: tenant-ns
  credentials:
    appDexSecret: appSecret
    cookieSecret: cookieSecret
  spec:
    applications:
    - domain: app.example.com
      ingressClassName: nginx
      signOutURL: /logout
      gatewayAPI:
        httpRouteListenerSetName: app-listeners
    - domain: second.example.com
      ingressClassName: nginx
`)
		hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorNames", `
"app@tenant-ns":
  name: app-dex-authenticator
  secretName: dex-authenticator-app
  ingressNames:
    "0":
      name: app-dex-authenticator
    "1":
      name: app-second-dex-authenticator
  signOutIngressNames:
    "0":
      name: app-dex-authenticator-sign-out
`)
	})

	Context("By default", func() {
		BeforeEach(func() {
			hec.HelmRender(WithAPIVersions("gateway.networking.k8s.io/v1/HTTPRoute"))
			Expect(hec.RenderError).NotTo(HaveOccurred())
		})

		It("checks the certificate of dex against the CA of dex", func() {
			args := authenticatorArgs(hec, "tenant-ns", "app-dex-authenticator")
			Expect(args).NotTo(ContainElement(HavePrefix("--ssl-insecure-skip-verify")))
			Expect(args).To(ContainElement("--provider-ca-file=/opt/dex-authenticator/dex-ca/ca.crt"))
			Expect(args).To(ContainElement("--redeem-url=https://dex.d8-user-authn/token"))
			Expect(args).To(ContainElement("--oidc-jwks-url=https://dex.d8-user-authn/keys"))

			secret := hec.KubernetesResource("Secret", "tenant-ns", "dex-authenticator-app")
			Expect(secret.Field(`data.dex-ca\.crt`).String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(dexCA))))

			pod := hec.KubernetesResource("Deployment", "tenant-ns", "app-dex-authenticator").Field("spec.template")
			Expect(pod.Get(`spec.volumes.#(name=="dex-ca")`).String()).To(MatchJSON(`{
				"name": "dex-ca",
				"secret": {"secretName": "dex-authenticator-app", "items": [{"key": "dex-ca.crt", "path": "ca.crt"}]}
			}`))
			Expect(pod.Get(`spec.containers.#(name=="dex-authenticator").volumeMounts.#(name=="dex-ca")`).String()).To(MatchJSON(`{
				"name": "dex-ca", "mountPath": "/opt/dex-authenticator/dex-ca", "readOnly": true
			}`))
			// oauth2-proxy reads the CA once, so a new CA of dex restarts the pods.
			Expect(pod.Get(`metadata.annotations.checksum/dex-ca`).String()).
				To(Equal(fmt.Sprintf("%x", sha256.Sum256([]byte(dexCA)))))
		})

		It("revokes the refresh token in dex on sign-out", func() {
			Expect(authenticatorArgs(hec, "tenant-ns", "app-dex-authenticator")).
				To(ContainElement("--revocation-url=https://dex.d8-user-authn/token/revoke"))
		})

		It("runs without the revocation feed while its CA is unknown", func() {
			// oauth2-proxy does not start with an empty CA file.
			Expect(authenticatorArgs(hec, "tenant-ns", "app-dex-authenticator")).
				NotTo(ContainElement(HavePrefix("--revocation-feed-")))
			secret := hec.KubernetesResource("Secret", "tenant-ns", "dex-authenticator-app")
			Expect(secret.Field(`data.revocation-feed-ca\.crt`).String()).To(BeEmpty())
		})

		It("uses PKCE", func() {
			Expect(authenticatorArgs(hec, "tenant-ns", "app-dex-authenticator")).To(ContainElement("--code-challenge-method=S256"))
		})

		// Two tabs signing in at the same time shared one CSRF cookie, and each sign-in overwrote the
		// PKCE verifier of the other, so dex refused every code.
		It("keeps one short-lived CSRF cookie per sign-in flow", func() {
			args := authenticatorArgs(hec, "tenant-ns", "app-dex-authenticator")
			Expect(args).To(ContainElement("--cookie-csrf-per-request=true"))
			Expect(args).To(ContainElement("--cookie-csrf-per-request-limit=10"))
			Expect(args).To(ContainElement("--cookie-csrf-expire=10m"))
			Expect(args).NotTo(ContainElement(HavePrefix("--cookie-domain")))
		})

		It("sets no environment nobody reads", func() {
			env := hec.KubernetesResource("Deployment", "tenant-ns", "app-dex-authenticator").
				Field(`spec.template.spec.containers.#(name=="dex-authenticator").env.#.name`)
			Expect(env.String()).To(MatchJSON(`["OAUTH2_PROXY_CLIENT_SECRET", "OAUTH2_PROXY_COOKIE_SECRET"]`))
		})

		It("keeps the sessions in the Redis sidecar of the pod", func() {
			Expect(authenticatorArgs(hec, "tenant-ns", "app-dex-authenticator")).To(ContainElements(
				"--session-store-type=redis",
				"--redis-connection-url=redis://127.0.0.1/",
			))
			deployment := hec.KubernetesResource("Deployment", "tenant-ns", "app-dex-authenticator")
			Expect(deployment.Field(`spec.template.spec.containers.#.name`).String()).To(MatchJSON(`["dex-authenticator", "redis"]`))
			Expect(deployment.Field(`spec.template.spec.containers.#(name=="redis").args`).String()).
				To(MatchJSON(`["--save", "", "--appendonly", "no", "--bind", "127.0.0.1"]`))
			vpa := hec.KubernetesResource("VerticalPodAutoscaler", "tenant-ns", "app-dex-authenticator")
			Expect(vpa.Field(`spec.resourcePolicy.containerPolicies.#.containerName`).String()).
				To(MatchJSON(`["dex-authenticator", "redis"]`))
		})

		It("finds the dex client of the authenticator by labels", func() {
			client := hec.KubernetesResource("OAuth2Client", "d8-user-authn", "appEncoded")
			Expect(client.Exists()).To(BeTrue())
			Expect(client.Field("id").String()).To(Equal("dex-authenticator-app@tenant-ns"))
			Expect(client.Field("name").String()).To(Equal("dex-authenticator-app@tenant-ns"))
			Expect(client.Field("secret").String()).To(Equal("appSecret"))
			Expect(client.Field("redirectURIs").String()).To(MatchJSON(`[
				"https://app.example.com/dex-authenticator/callback",
				"https://second.example.com/dex-authenticator/callback"
			]`))
			Expect(client.Field("metadata.labels.app").String()).To(Equal("dex"))
			Expect(client.Field(`metadata.labels.deckhouse\.io/dex-authenticator-for`).String()).To(Equal("app"))
			Expect(client.Field(`metadata.labels.deckhouse\.io/dex-authenticator-namespace`).String()).To(Equal("tenant-ns"))
			Expect(client.Field(`metadata.labels.heritage`).String()).To(Equal("deckhouse"))
		})

		It("names the dex client on the objects that route to the authenticator", func() {
			Expect(authenticatorArgs(hec, "tenant-ns", "app-dex-authenticator")).
				To(ContainElement("--client-id=dex-authenticator-app@tenant-ns"))
			for _, object := range []struct{ kind, name string }{
				{"Service", "app-dex-authenticator"},
				{"Ingress", "app-dex-authenticator"},
				{"Ingress", "app-second-dex-authenticator"},
				{"Ingress", "app-dex-authenticator-sign-out"},
				{"HTTPRoute", "app-dex-authenticator"},
			} {
				resource := hec.KubernetesResource(object.kind, "tenant-ns", object.name)
				Expect(resource.Exists()).To(BeTrue(), object.kind+" "+object.name)
				Expect(resource.Field(`metadata.annotations.dexauthenticator\.deckhouse\.io/client-id`).String()).
					To(Equal("dex-authenticator-app@tenant-ns"), object.kind+" "+object.name)
			}
		})
	})

	Context("With the CA of the revocation feed", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.internal.revocationFeedTLS.ca", revocationFeedCA)
			hec.HelmRender()
			Expect(hec.RenderError).NotTo(HaveOccurred())
		})

		It("subscribes to the revocation feed and trusts only its CA for it", func() {
			Expect(authenticatorArgs(hec, "tenant-ns", "app-dex-authenticator")).To(ContainElements(
				"--revocation-feed-url=https://revocation-feed.d8-user-authn/revocations",
				"--revocation-feed-ca-file=/opt/dex-authenticator/revocation-feed-ca/ca.crt",
			))

			secret := hec.KubernetesResource("Secret", "tenant-ns", "dex-authenticator-app")
			Expect(secret.Field(`data.revocation-feed-ca\.crt`).String()).
				To(Equal(base64.StdEncoding.EncodeToString([]byte(revocationFeedCA))))
			Expect(secret.Field(`data.dex-ca\.crt`).String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(dexCA))))

			pod := hec.KubernetesResource("Deployment", "tenant-ns", "app-dex-authenticator").Field("spec.template")
			Expect(pod.Get(`spec.volumes.#(name=="revocation-feed-ca")`).String()).To(MatchJSON(`{
				"name": "revocation-feed-ca",
				"secret": {"secretName": "dex-authenticator-app", "items": [{"key": "revocation-feed-ca.crt", "path": "ca.crt"}]}
			}`))
			Expect(pod.Get(`spec.containers.#(name=="dex-authenticator").volumeMounts.#(name=="revocation-feed-ca")`).String()).To(MatchJSON(`{
				"name": "revocation-feed-ca", "mountPath": "/opt/dex-authenticator/revocation-feed-ca", "readOnly": true
			}`))
			// oauth2-proxy reads the CA once, so a new CA of the feed restarts the pods.
			Expect(pod.Get(`metadata.annotations.checksum/revocation-feed-ca`).String()).
				To(Equal(fmt.Sprintf("%x", sha256.Sum256([]byte(revocationFeedCA)))))
		})

		It("keeps the readiness of the authenticator independent of the feed", func() {
			probe := hec.KubernetesResource("Deployment", "tenant-ns", "app-dex-authenticator").
				Field(`spec.template.spec.containers.#(name=="dex-authenticator").readinessProbe`)
			Expect(probe.String()).To(MatchJSON(`{
				"httpGet": {"port": 8443, "scheme": "HTTPS", "path": "/ready"},
				"initialDelaySeconds": 5,
				"periodSeconds": 5
			}`))
		})
	})

	Context("With a Redis session store outside the pod", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.internal.dexAuthenticatorRedisURL", "redis://dex-authenticator-redis.d8-user-authn:6379/")
			hec.HelmRender()
			Expect(hec.RenderError).NotTo(HaveOccurred())
		})

		It("points the authenticator at it and runs no sidecar", func() {
			Expect(authenticatorArgs(hec, "tenant-ns", "app-dex-authenticator")).To(ContainElements(
				"--session-store-type=redis",
				"--redis-connection-url=redis://dex-authenticator-redis.d8-user-authn:6379/",
			))
			deployment := hec.KubernetesResource("Deployment", "tenant-ns", "app-dex-authenticator")
			Expect(deployment.Field(`spec.template.spec.containers.#.name`).String()).To(MatchJSON(`["dex-authenticator"]`))
			vpa := hec.KubernetesResource("VerticalPodAutoscaler", "tenant-ns", "app-dex-authenticator")
			Expect(vpa.Field(`spec.resourcePolicy.containerPolicies.#.containerName`).String()).
				To(MatchJSON(`["dex-authenticator"]`))
		})
	})

	Context("With a local Redis address on another port", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.internal.dexAuthenticatorRedisURL", "redis://localhost:6380/")
			hec.HelmRender()
			Expect(hec.RenderError).NotTo(HaveOccurred())
		})

		It("runs the sidecar on that port", func() {
			deployment := hec.KubernetesResource("Deployment", "tenant-ns", "app-dex-authenticator")
			Expect(deployment.Field(`spec.template.spec.containers.#.name`).String()).To(MatchJSON(`["dex-authenticator", "redis"]`))
			Expect(deployment.Field(`spec.template.spec.containers.#(name=="redis").args`).String()).
				To(MatchJSON(`["--save", "", "--appendonly", "no", "--bind", "127.0.0.1", "--port", "6380"]`))
		})
	})

	// The CRD admits keepUsersLoggedInFor in the format of idTokenTTL with a bounded number of digits, so
	// that every value it admits parses as a duration and renders: a value the template cannot parse fails
	// the release of the whole module.
	for _, tc := range []struct{ value, cookieExpire string }{
		{"", "168h"},
		{"45s", "10m1s"},
		{"1h0s", "1h0s"},
		{"2h30m10s", "2h30m10s"},
		{"99999h9999999m999999999s", "99999h9999999m999999999s"},
	} {
		tc := tc
		It("renders keepUsersLoggedInFor "+tc.value+" in the format of the CRD", func() {
			hec.ValuesSet("userAuthn.idTokenTTL", "10m")
			hec.ValuesSet("userAuthn.internal.dexAuthenticatorCRDs.0.spec.keepUsersLoggedInFor", tc.value)
			hec.HelmRender()
			Expect(hec.RenderError).NotTo(HaveOccurred())
			Expect(authenticatorArgs(hec, "tenant-ns", "app-dex-authenticator")).To(ContainElements(
				"--cookie-refresh=10m",
				"--cookie-expire="+tc.cookieExpire,
			))
		})
	}

	// spec.highAvailability of the DexAuthenticator overrides the high availability of the cluster. The rollout
	// strategy and the PodDisruptionBudget follow the replicas it gives: a single replica surges, so that a
	// rollout is not an outage, and may be evicted.
	for _, tc := range []struct {
		name            string
		clusterHA       bool
		authenticatorHA *bool
		replicas        int64
		maxSurge        int64
		maxUnavailable  int64
		minAvailable    int64
	}{
		{name: "in a cluster without high availability", clusterHA: false, replicas: 1, maxSurge: 1, maxUnavailable: 0, minAvailable: 0},
		{name: "in a cluster with high availability", clusterHA: true, replicas: 2, maxSurge: 0, maxUnavailable: 1, minAvailable: 1},
		{name: "without high availability in a cluster with it", clusterHA: true, authenticatorHA: ptr.To(false), replicas: 1, maxSurge: 1, maxUnavailable: 0, minAvailable: 0},
		{name: "with high availability in a cluster without it", clusterHA: false, authenticatorHA: ptr.To(true), replicas: 2, maxSurge: 0, maxUnavailable: 1, minAvailable: 1},
	} {
		tc := tc
		It("rolls out the authenticator "+tc.name+" without an outage", func() {
			hec.ValuesSet("global.highAvailability", tc.clusterHA)
			if tc.authenticatorHA != nil {
				hec.ValuesSet("userAuthn.internal.dexAuthenticatorCRDs.0.spec.highAvailability", *tc.authenticatorHA)
			}
			hec.HelmRender()
			Expect(hec.RenderError).NotTo(HaveOccurred())

			deployment := hec.KubernetesResource("Deployment", "tenant-ns", "app-dex-authenticator")
			Expect(deployment.Field("spec.replicas").Int()).To(Equal(tc.replicas))
			Expect(deployment.Field("spec.strategy.type").String()).To(Equal("RollingUpdate"))
			Expect(deployment.Field("spec.strategy.rollingUpdate.maxSurge").Int()).To(Equal(tc.maxSurge))
			Expect(deployment.Field("spec.strategy.rollingUpdate.maxUnavailable").Int()).To(Equal(tc.maxUnavailable))
			Expect(tc.replicas+tc.maxSurge-tc.maxUnavailable).To(BeNumerically(">=", 1),
				"a replica keeps serving during a rollout")

			pdb := hec.KubernetesResource("PodDisruptionBudget", "tenant-ns", "app-dex-authenticator")
			Expect(pdb.Field("spec.minAvailable").Int()).To(Equal(tc.minAvailable))
			Expect(tc.minAvailable).To(BeNumerically("<", tc.replicas), "the budget does not block the drain of a node")
		})
	}
})
