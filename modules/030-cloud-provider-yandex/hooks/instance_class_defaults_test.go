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

package hooks

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: instance_class_defaults ::", func() {
	const (
		initValuesString = `
global:
  discovery: {}
cloudProviderYandex:
  internal: {}
`
		imageIDValuesPath = "cloudProviderYandex.internal.instanceClassDefaults.imageID"

		pccWithMasterImage = `
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
nodeNetworkCIDR: 10.0.0.0/16
sshPublicKey: ssh-rsa AAAAAbbbb
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 4
    memory: 8192
    imageID: pcc-master-image
provider:
  cloudID: test
  folderID: test
  serviceAccountJSON: |-
    {
      "id": "test"
    }
`

		// A cluster whose masters this provider does not manage carries no masterNodeGroup.
		pccWithoutMasterNodeGroup = `
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
nodeNetworkCIDR: 10.0.0.0/16
sshPublicKey: ssh-rsa AAAAAbbbb
provider:
  cloudID: test
  folderID: test
  serviceAccountJSON: |-
    {
      "id": "test"
    }
`

		discoveryData = `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "YandexCloudDiscoveryData",
  "defaultLbTargetGroupNetworkId": "test",
  "internalNetworkIDs": ["test"],
  "region": "test",
  "routeTableID": "test",
  "shouldAssignPublicIPAddress": false,
  "zoneToSubnetIdMap": {"ru-central1-a": "test"},
  "zones": ["ru-central1-a"]
}
`
	)

	// The migration names the master class through BuildInstanceClassName; clusters set up
	// by hand may carry a plain "master" instead, and the hook accepts both.
	generatedMasterName := cpapi.BuildInstanceClassName("master")

	instanceClass := func(name, imageID string) string {
		spec := fmt.Sprintf(`
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: %s
spec:
  cores: 4
  memory: 8192
`, name)
		if imageID != "" {
			spec += fmt.Sprintf("  imageID: %s\n", imageID)
		}
		return spec
	}

	pccSecret := func(clusterConfiguration string) string {
		return fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`,
			base64.StdEncoding.EncodeToString([]byte(clusterConfiguration)),
			base64.StdEncoding.EncodeToString([]byte(discoveryData)),
		)
	}

	pccSecretWithoutClusterConfiguration := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(discoveryData)))

	newSuite := func() *HookExecutionConfig {
		suite := HookExecutionConfigInit(initValuesString, `{}`)
		suite.RegisterCRD("deckhouse.io", "v1", "YandexInstanceClass", false)
		return suite
	}

	// runWith wires one KubeState through the hook and asserts the resulting default.
	runWith := func(description, kubeState, wantImageID string) {
		Context(description, func() {
			f := newSuite()

			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(kubeState))
				f.RunHook()
			})

			It(fmt.Sprintf("resolves the default image to %q", wantImageID), func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(imageIDValuesPath).String()).To(Equal(wantImageID))
			})
		})
	}

	runWith(
		"Cluster has neither the master InstanceClass nor a PCC",
		``,
		"",
	)

	runWith(
		"Migration is done: only the generated master InstanceClass exists",
		instanceClass(generatedMasterName, "ic-master-image"),
		"ic-master-image",
	)

	runWith(
		"The master InstanceClass carries the plain name",
		instanceClass("master", "plain-master-image"),
		"plain-master-image",
	)

	runWith(
		"Migration is in progress: only the PCC exists",
		pccSecret(pccWithMasterImage),
		"pcc-master-image",
	)

	runWith(
		"Both the master InstanceClass and the PCC exist",
		instanceClass(generatedMasterName, "ic-master-image")+"\n---\n"+pccSecret(pccWithMasterImage),
		"ic-master-image",
	)

	runWith(
		"The master InstanceClass carries no image but the PCC does",
		instanceClass(generatedMasterName, "")+"\n---\n"+pccSecret(pccWithMasterImage),
		"pcc-master-image",
	)

	// Both accepted names resolve to a class, and only one of them carries an image: the
	// imageless one must not shadow it, whichever order the snapshots arrive in.
	runWith(
		"Both master names exist and only the generated one carries an image",
		instanceClass("master", "")+"\n---\n"+instanceClass(generatedMasterName, "ic-master-image"),
		"ic-master-image",
	)

	runWith(
		"Both master names exist and only the plain one carries an image",
		instanceClass("master", "plain-master-image")+"\n---\n"+instanceClass(generatedMasterName, ""),
		"plain-master-image",
	)

	runWith(
		"The PCC Secret carries discovery data only",
		pccSecretWithoutClusterConfiguration,
		"",
	)

	runWith(
		"The PCC has no masterNodeGroup",
		pccSecret(pccWithoutMasterNodeGroup),
		"",
	)

	runWith(
		"Only a non-master InstanceClass exists",
		instanceClass("worker-abcdef012345", "worker-image"),
		"",
	)

	Context("The master InstanceClass is deleted after the default was set", func() {
		f := newSuite()

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(instanceClass(generatedMasterName, "ic-master-image")))
			f.RunHook()
			Expect(f.ValuesGet(imageIDValuesPath).String()).To(Equal("ic-master-image"))

			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("drops the stale default", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(imageIDValuesPath).String()).To(BeEmpty())
		})
	})
})
