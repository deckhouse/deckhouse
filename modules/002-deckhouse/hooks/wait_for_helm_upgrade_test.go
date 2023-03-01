/*
Copyright 2023 Flant JSC

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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: deckhouse :: hooks :: wait for deckhouse update ::", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)
	ts := time.Now().UTC()
	pollTimeout = 1 * time.Second

	Context("Deckhouse release is upgrading", func() {
		BeforeEach(func() {
			createHelmReleaseSecret("v1", "superseded", ts.Add(1*time.Second))
			createHelmReleaseSecret("v2", "deployed", ts.Add(2*time.Second))
			createHelmReleaseSecret("v3", "pending-upgrade", ts.Add(3*time.Second))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})
		It("Should block the main queue", func() {
			Expect(f.GoHookError.Error()).To(Equal("timeout waiting for deckhouse release to be deployed. last error: <nil>"))
		})
	})

	Context("Deckhouse release is deployed", func() {
		BeforeEach(func() {
			createHelmReleaseSecret("v4", "superseded", ts.Add(4*time.Second))
			createHelmReleaseSecret("v5", "superseded", ts.Add(5*time.Second))
			createHelmReleaseSecret("v6", "deployed", ts.Add(6*time.Second))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})
		It("Should continue", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

})

func createHelmReleaseSecret(version, status string, ts time.Time) {
	sec := v1.Secret{
		Type: "helm.sh/release.v1",
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: metav1.Time{Time: ts},
			Namespace:         "d8-system",
			Name:              fmt.Sprintf("sh.helm.release.v1.deckhouse.%s", version),
			Labels: map[string]string{
				"name":   "deckhouse",
				"status": status,
			},
		},
	}

	_, err := dependency.TestDC.K8sClient.CoreV1().Secrets("d8-system").Create(context.Background(), &sec, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}
