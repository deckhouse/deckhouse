package template_tests

import (
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func assertBashibleAPIServerTLSSecret(f *Config, namespace string) {
	secret := f.KubernetesResource("Secret", namespace, "bashible-api-server-tls")

	Expect(secret.Exists()).To(BeTrue())

	ca := getDecodedSecretValue(&secret, "ca\\.crt")
	crt := getDecodedSecretValue(&secret, "apiserver\\.crt")
	key := getDecodedSecretValue(&secret, "apiserver\\.key")

	Expect(ca).To(BeEquivalentTo(bashibleAPIServerCA))
	Expect(crt).To(BeEquivalentTo(bashibleAPIServerCrt))
	Expect(key).To(BeEquivalentTo(bashibleAPIServerKey))

}

func assertBashibleAPIServerCaBundle(f *Config) {
	apiService := f.KubernetesGlobalResource("APIService", "v1alpha1.bashible.deckhouse.io")

	caBundle := decodeK8sObjField(&apiService, "spec.caBundle")
	Expect(caBundle).To(BeEquivalentTo(bashibleAPIServerCA))
}

func assertBashibleAPIServerTLS(f *Config, namespace string) {
	assertBashibleAPIServerTLSSecret(f, namespace)
	assertBashibleAPIServerCaBundle(f)
}
