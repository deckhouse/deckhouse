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
	"encoding/base64"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/flant/kube-client/manifest/releaseutil"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/testing/library"
	libhelm "github.com/deckhouse/deckhouse/testing/library/helm"
)

const goldenGlobal = `
global:
  enabledModules: ["vertical-pod-autoscaler"]
  modules:
    placement: {}
  internal:
    modules:
      kubeRBACProxyCA:
        cert: test
        key: test
  discovery:
    d8SpecificNodeCountByRole:
      master: 3
    clusterUUID: deadbeef-dead-beef-dead-beefdeadbeef
    kubernetesVersion: 1.32.8
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: vSphere
    clusterDomain: cluster.local
    clusterType: Cloud
    defaultCRI: Containerd
    kind: ClusterConfiguration
    kubernetesVersion: "1.32"
    podSubnetCIDR: 10.111.0.0/16
    podSubnetNodeCIDRPrefix: "24"
    serviceSubnetCIDR: 10.222.0.0/16
  modulesImages:
    registry:
      base: registry.example.com
      dockercfg: Y2ZnCg==
      address: registry.deckhouse.io
      path: /deckhouse/fe
      CA: CACACA
      scheme: https
    digests:
      registrypackages:
        jq171: sha256:jq
        d8Curl891: sha256:curl
        tailLog: sha256:tail
        rppGet: sha256:rpp
        ec2DescribeTagsV001Flant3: sha256:ec2
`

const goldenNodeManagerCommon = `
nodeManager:
  mcmEmergencyBrake: false
  internal:
    capiControllerManagerWebhookCert:
      ca: string
      key: string
      crt: string
    capsControllerManagerWebhookCert:
      ca: string
      key: string
      crt: string
    nodeControllerWebhookCert:
      ca: string
      key: string
      crt: string
    bashibleApiServerCA: meapiserverca
    bashibleApiServerCrt: meapiservercrt
    bashibleApiServerKey: meapiserverprivkey
    instancePrefix: myprefix
    clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"]
    clusterMasterEndpoints:
    - address: 10.0.0.1
      kubeApiPort: 6443
      rppServerPort: 4219
      rppBootstrapServerPort: 4220
    kubernetesCA: myclusterca
    packagesProxy:
      token: mytoken
    bootstrapTokens:
      worker: myworker
`

const goldenStaticInstances = `
    nodeGroups:
    - name: worker
      nodeType: Static
      staticInstances:
        labelSelector:
          matchLabels:
            node-group: worker
      kubernetesVersion: "1.32"
      cri:
        type: "Containerd"
`

const goldenCloudPermanent = `
    nodeGroups:
    - name: worker
      nodeType: CloudPermanent
      kubernetesVersion: "1.32"
      cri:
        type: "Containerd"
`

const goldenCAPIYandex = `
    cloudProvider:
      type: yandex
      capiClusterKind: YandexCluster
      capiClusterName: sandbox
      sshPublicKey: ssh-rsa AAAA
      yandex:
        instanceClassDefaults: {}
        serviceAccountJSON: '{"my":"svcacc"}'
        region: myreg
        folderID: myfolder
        sshKey: cert-authority,principals="test" ssh-rsa AAAAB...==
        sshUser: mysshuser
        nameservers: ["4.2.2.2"]
        dns:
          search: ["qwe"]
          nameservers: ["1.2.3.4","3.4.5.6"]
        zoneToSubnetIdMap:
          zonea: subneta
    nodeGroups:
    - name: worker
      instanceClass:
        platformID: myplaid
        cores: 42
        memory: 42
        imageID: myimageid
      nodeType: CloudEphemeral
      kubernetesVersion: "1.32"
      cri:
        type: "Containerd"
      cloudInstances:
        classReference:
          kind: YandexInstanceClass
          name: worker
        maxPerZone: 5
        minPerZone: 2
        zones:
        - zonea
`

const goldenMCMAWS = `
    cloudProvider:
      type: aws
      machineClassKind: AWSInstanceClass
      aws:
        providerAccessKeyId: myprovaccesskeyid
        providerSecretAccessKey: myprovsecretaccesskey
        region: myregion
        loadBalancerSecurityGroupID: mylbsecuritygroupid
        keyName: mykeyname
        instances:
          iamProfileName: myiamprofilename
          additionalSecurityGroups: ["mysecgroupid1", "mysecgroupid2"]
          extraTags: ["extratag1", "extratag2"]
        internal:
          zoneToSubnetIdMap:
            zonea: mysubnetida
    nodeGroups:
    - name: worker
      instanceClass:
        ami: myami
        diskSizeGb: 50
        diskType: gp2
        iops: 42
        instanceType: t2.medium
      nodeType: CloudEphemeral
      kubernetesVersion: "1.32"
      cri:
        type: "Containerd"
      cloudInstances:
        classReference:
          kind: AWSInstanceClass
          name: worker
        maxPerZone: 5
        minPerZone: 2
        zones:
        - zonea
    machineControllerManagerEnabled: true
`

const goldenMCMAzure = `
    cloudProvider:
      type: azure
      machineClassKind: AzureMachineClass
      azure:
        sshPublicKey: ssh-rsa AAAAB...==
        clientId: clientId
        clientSecret: clientSecret
        subscriptionId: subscriptionId
        tenantId: tenantId
        location: location
        resourceGroupName: resourceGroupName
        vnetName: vnetName
        subnetName: subnetName
        urn: urn
        diskType: diskType
        additionalTags: []
    nodeGroups:
    - name: worker
      instanceClass:
        machineSize: mymachinesize
        urn: myurn
        diskSizeGb: 42
      nodeType: CloudEphemeral
      kubernetesVersion: "1.32"
      cri:
        type: "Containerd"
      cloudInstances:
        classReference:
          kind: AzureInstanceClass
          name: worker
        maxPerZone: 5
        minPerZone: 2
        zones:
        - zonea
    machineControllerManagerEnabled: true
`

// TestGenerateGoldens renders the node-group templates with helm and writes the
// bootstrap payloads node-controller must reproduce byte for byte
// (internal/bootstrap/testdata/golden, guarded by TestRenderMatchesHelmGoldens).
// Never regenerate a golden from node-controller's own output: a golden taken
// from the renderer under test guards nothing but its own drift.
//
// Three files a checkout does not have must be staged first, and reverted
// after. Every one of them is present in the built image, so leaving them out
// would freeze a render that production never performs:
//
//	# 1. modules/040-node-manager/candi is an absolute symlink to /deckhouse/candi,
//	#    i.e. to a different checkout. Point it at this one.
//	ln -sfn ../../candi modules/040-node-manager/candi
//
//	# 2. The minget binary is produced by the build. Its base64 goes into the
//	#    script, so a stub with known content keeps the golden readable.
//	printf 'minget' > candi/bashible/bootstrap/minget
//
//	# 3. candi/cloud-providers is assembled by tools/build_includes/candi-cloud-providers-*.yaml.
//	#    The bootstrap script inlines the provider's network setup, so without
//	#    these two the aws and yandex goldens differ only by an ssh block.
//	for p in aws yandex; do
//	  mkdir -p candi/cloud-providers/$p/bashible
//	  cp modules/030-cloud-provider-$p/candi/bashible/bootstrap-networks.sh.tpl \
//	     candi/cloud-providers/$p/bashible/
//	done
//
//	cd modules/040-node-manager/template_tests
//	GENERATE_GOLDENS=1 go test -run TestGenerateGoldens -v .
//
//	# revert
//	git checkout modules/040-node-manager/candi
//	rm -rf candi/cloud-providers candi/bashible/bootstrap/minget
//
// The values below must stay in step with the Input fixtures in
// internal/bootstrap/render_test.go: the goldens only prove a byte match while
// both sides describe the same cluster.
func TestGenerateGoldens(t *testing.T) {
	if os.Getenv("GENERATE_GOLDENS") != "1" {
		t.Skip("golden generator, run with GENERATE_GOLDENS=1 (see the doc comment)")
	}

	modulePath, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(modulePath, "images", "node-controller", "src", "internal", "bootstrap", "testdata", "golden")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		values string
		keys   map[string]string
	}{
		{"static-instances", goldenStaticInstances, map[string]string{"cloud-config": "cloud-config", "bootstrap.sh": "bootstrap-sh"}},
		{"cloud-permanent", goldenCloudPermanent, map[string]string{"bootstrap.sh": "bootstrap-sh"}},
		{"capi-yandex", goldenCAPIYandex, map[string]string{"value": "value"}},
		{"mcm-aws", goldenMCMAWS, map[string]string{"userData": "userData"}},
		{"azure", goldenMCMAzure, map[string]string{"userData": "userData"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			values, err := library.InitValues(modulePath, []byte(goldenGlobal+goldenNodeManagerCommon+tc.values))
			if err != nil {
				t.Fatal(err)
			}
			valuesYAML, err := yaml.Marshal(values)
			if err != nil {
				t.Fatal(err)
			}

			files, err := libhelm.Renderer{}.RenderChartFromDir(modulePath, string(valuesYAML))
			if err != nil {
				t.Fatal(err)
			}

			data := map[string]string{}
			for path, manifests := range files {
				if !strings.Contains(path, "node-group/node-group.yaml") {
					continue
				}
				for _, doc := range releaseutil.SplitManifests(manifests) {
					var obj map[string]any
					if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
						t.Fatal(err)
					}
					secretData, ok := obj["data"].(map[string]any)
					if !ok {
						continue
					}
					for k, v := range secretData {
						s, ok := v.(string)
						if !ok {
							continue
						}
						data[k] = s
					}
					t.Logf("%s: secret %v keys %v", tc.name, obj["metadata"], slices.Sorted(maps.Keys(secretData)))
				}
			}

			for key, suffix := range tc.keys {
				encoded, ok := data[key]
				if !ok {
					t.Fatalf("key %q not found, have %v", key, slices.Sorted(maps.Keys(data)))
				}
				decoded, err := base64.StdEncoding.DecodeString(encoded)
				if err != nil {
					t.Fatal(err)
				}
				out := filepath.Join(outDir, fmt.Sprintf("%s-%s.txt", tc.name, suffix))
				if err := os.WriteFile(out, decoded, 0o644); err != nil {
					t.Fatal(err)
				}
				t.Logf("wrote %s (%d bytes)", out, len(decoded))
			}
		})
	}
}
