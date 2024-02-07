/*
Copyright 2021 Flant JSC

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

func assertBashibleAPIServerTLS(f *Config) {
	assertBashibleAPIServerTLSSecret(f, nodeManagerNamespace)
	assertBashibleAPIServerCaBundle(f)
}
