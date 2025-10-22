/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-openstack :: hooks :: cloud_provider_discovery_data ::", func() {
	initValues := `
cloudProviderOpenstack:
  internal:
    connection:
      authURL: https://test.tests.com:5000/v3/
      username: jamie
      password: nein
      domainName: default
      tenantName: default
      tenantID: "123"
      region: HetznerFinland
`

	storageClasses := `---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: default
  uid: 0c09c147-d4c8-4d48-b014-cb34d508eac5
  resourceVersion: '45632997'
  creationTimestamp: '2023-06-01T06:09:25Z'
  labels:
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: cloud-provider-openstack
  annotations:
    meta.helm.sh/release-name: cloud-provider-openstack
    meta.helm.sh/release-namespace: d8-system
  selfLink: /apis/storage.k8s.io/v1/storageclasses/default
provisioner: cinder.csi.openstack.org
parameters:
  type: __DEFAULT__
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
---
allowVolumeExpansion: true
allowedTopologies:
- matchLabelExpressions:
  - key: node.deckhouse.io/group
    values:
    - system
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  annotations:
    meta.helm.sh/release-name: local-path-provisioner
    meta.helm.sh/release-namespace: d8-system
  creationTimestamp: "2022-11-24T16:33:07Z"
  labels:
    app: local-path-provisioner
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: local-path-provisioner
  name: localpath-system
  resourceVersion: "106273"
  uid: 86533f89-b78a-4cd2-a7b7-5c2a4a77a163
provisioner: deckhouse.io/localpath-system
reclaimPolicy: Retain
volumeBindingMode: WaitForFirstConsumer
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ceph-ssd
  uid: 6daab1cc-6aa7-433b-8788-d905adb0e9cb
  resourceVersion: '45632996'
  creationTimestamp: '2023-06-01T06:09:25Z'
  labels:
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: cloud-provider-openstack
  annotations:
    meta.helm.sh/release-name: cloud-provider-openstack
    meta.helm.sh/release-namespace: d8-system
    storageclass.kubernetes.io/is-default-class: 'true'
  selfLink: /apis/storage.k8s.io/v1/storageclasses/ceph-ssd
provisioner: cinder.csi.openstack.org
parameters:
  type: ceph-ssd
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
`

	manualStorageClasses := `---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: manual-default
provisioner: cinder.csi.openstack.org
parameters:
  type: manual-default
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: manual-ceph-ssd
  annotations:
    storageclass.kubernetes.io/is-default-class: 'true'
provisioner: cinder.csi.openstack.org
parameters:
  type: manual-ceph-ssd
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
`

	//nolint:misspell
	discoveryData := `
{
  "apiVersion": "deckhouse.io/v1alpha1",
  "kind": "OpenStackCloudProviderDiscoveryData",
  "flavors": [
    "m1.medium-50g",
    "m1.xlarge",
    "m1.large-cpu-host-passthrough",
    "m2.large-50g",
    "c8m16d100",
    "m1.large",
    "c8m16d50",
    "m1.medium",
    "m1.large-cpu-host-model",
    "m1.tiny",
    "m1.large-50g",
    "m1.xsmall",
    "m1.small"
  ],
  "additionalNetworks": [
    "public",
    "dev",
    "shared"
  ],
  "additionalSecurityGroups": [
    "default",
    "dev",
    "dev-frontend"
  ],
  "defaultImageName": "ubuntu-22-04-cloud-amd64",
  "images": [
    "centos-9-x86_64-cloud",
    "alse-vanilla-1.7.3-cloud-adv-mg9.1.2",
    "alse-vanilla-1.7.3-cloud-max-mg9.1.2",
    "almalinux-8.7.x86_64",
    "almalinux-9.1.x86_64",
    "redos-standard-7.3.2",
    "fedora-coreos-36.20221014.3.1-openstack.x86_64",
    "alse-vanilla-1.7.2-cloud-mg7.2.0",
    "ubuntu-22-04-cloud-amd64",
    "debian-11-cloud-amd64",
    "orel-vanilla-2.12.43-cloud-mg6.5.0",
    "centos-8-cloud-amd64",
    "deepin-desktop-community-20.4-amd64-iso",
    "alse-vanilla-1.7.1-cloud-mg6.4.0",
    "debian-9-cloud-amd64",
    "orel-vanilla-2.12.43-cloud-mg6",
    "debian-10-cloud-amd64",
    "orel-vanilla-2.12.43-cloud",
    "ubuntu-20-04-cloud-amd64",
    "centos-7-x86_64-cloud-2003",
    "ubuntu-18-04-cloud-amd64-kubernetes-1.15.3",
    "ubuntu-16-04-cloud-amd64",
    "ubuntu-18-04-cloud-amd64"
  ],
  "mainNetwork": "dev",
  "zones": [
    "nova",
	"zz-zzz-1z",
	"xx-xxx-1x",
	"cc-ccc-1c"
  ],
  "volumeTypes": [
    {
      "id": "cc728a7f-787d-4a3d-ae3a-bf7611cda23e",
      "name": "__DEFAULT__",
      "description": "Default Volume Type",
      "isPublic": true
    },
    {
      "id": "b5637549-2e1a-4cf5-adb9-d854f52a9865",
      "name": "ceph-ssd",
      "isPublic": true
    },
    {
      "id": "11637549-2e1a-4cf5-adb9-d854f52a9865",
      "name": "other-bar"
    },
    {
      "id": "12637549-2e1a-4cf5-adb9-d854f52a9865",
      "name": "some-foo"
    },
    {
      "id": "13637549-2e1a-4cf5-adb9-d854f52a9865",
      "name": "SSD R1"
    },
    {
      "id": "14637549-2e1a-4cf5-adb9-d854f52a9865",
      "name": "-Xx__$()? -foo-"
    },
    {
      "id": "15637549-2e1a-4cf5-adb9-d854f52a9865",
      "name": "  YY fast SSD-foo."
    }
  ]
}`

	state := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cloud-provider-discovery-data
  namespace: kube-system
data:
  "discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(discoveryData)))

	a := HookExecutionConfigInit(initValues, `{}`)
	Context("Cluster has empty state", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(``))
			a.RunHook()
		})

		It("Hook should not fail with errors", func() {
			Expect(a).To(ExecuteSuccessfully())
			Expect(a.GoHookError).Should(BeNil())
		})
	})

	b := HookExecutionConfigInit(initValues, `{}`)
	Context("Cluster has only storage classes", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(storageClasses))
			b.RunHook()
		})

		It("Should discover all volumeTypes only for storage classes where deployed by cloud-provider-openstack module and no default", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderOpenstack.internal.storageClasses").String()).To(MatchJSON(`
[
  {
	"name": "ceph-ssd",
	"type": "ceph-ssd"
  },
  {
	"name": "default",
	"type": "__DEFAULT__"
  }
]
`))
		})
	})

	c := HookExecutionConfigInit(initValues, `{}`)
	Context("Cluster has only manual storage classes", func() {
		BeforeEach(func() {
			c.BindingContexts.Set(c.KubeStateSet(manualStorageClasses))
			c.RunHook()
		})

		It("Should not discover manual volumeTypes", func() {
			Expect(c).To(ExecuteSuccessfully())
			Expect(c.ValuesGet("cloudProviderOpenstack.internal.storageClasses").String()).To(BeEmpty())
		})
	})

	d := HookExecutionConfigInit(initValues, `{}`)
	Context("Cluster has deckhouse managed storage classes and manual storage classes", func() {
		BeforeEach(func() {
			d.BindingContexts.Set(d.KubeStateSet(storageClasses + manualStorageClasses))
			d.RunHook()
		})

		It("Should discover all deckhouse managed volumeTypes and no default", func() {
			Expect(d).To(ExecuteSuccessfully())
			Expect(d.ValuesGet("cloudProviderOpenstack.internal.storageClasses").String()).To(MatchJSON(`
[
  {
	"name": "ceph-ssd",
	"type": "ceph-ssd"
  },
  {
	"name": "default",
	"type": "__DEFAULT__"
  }
]
`))
		})
	})

	e := HookExecutionConfigInit(initValues, `{}`)
	Context("Provider data is successfully discovered", func() {
		BeforeEach(func() {
			e.BindingContexts.Set(e.KubeStateSet(state))
			e.RunHook()
		})

		It("Zones values should be gathered from discovered data", func() {
			Expect(e).To(ExecuteSuccessfully())

			Expect(e.ValuesGet("cloudProviderOpenstack.internal.discoveryData.zones").String()).To(MatchJSON(`["nova", "zz-zzz-1z", "xx-xxx-1x", "cc-ccc-1c"]`))
		})

		It("Should discover all volumeTypes and no default", func() {
			Expect(e).To(ExecuteSuccessfully())
			Expect(e.ValuesGet("cloudProviderOpenstack.internal.storageClasses").String()).To(MatchJSON(`
[
  {
	"name": "ceph-ssd",
	"type": "ceph-ssd"
  },
  {
	"name": "default",
	"type": "__DEFAULT__"
  },
  {
	"name": "other-bar",
	"type": "other-bar"
  },
  {
	"name": "some-foo",
	"type": "some-foo"
  },
  {
	"name": "ssd-r1",
	"type": "SSD R1"
  },
  {
	"name": "xx--foo",
	"type": "-Xx__$()? -foo-"
  },
  {
	"name": "yy-fast-ssd-foo",
	"type": "  YY fast SSD-foo."
  }
]
`))
		})
	})
})
