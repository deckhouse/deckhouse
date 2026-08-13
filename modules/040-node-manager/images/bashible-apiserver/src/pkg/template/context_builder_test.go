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

package template

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"sigs.k8s.io/yaml"
)

const testRPPBootstrapServerPort = 4282

func TestClusterUUIDIsPreservedInTemplateContexts(t *testing.T) {
	const clusterUUID = "ce64db27-f724-4b50-bb86-e4ac57a1d49d"

	var input inputData
	if err := yaml.Unmarshal([]byte("clusterUUID: "+clusterUUID+"\n"), &input); err != nil {
		t.Fatalf("unmarshal inputData: %v", err)
	}
	if input.ClusterUUID != clusterUUID {
		t.Fatalf("inputData.ClusterUUID = %q, want %q", input.ClusterUUID, clusterUUID)
	}

	common := tplContextCommon{ClusterUUID: input.ClusterUUID}
	bundle := bundleNGContext{tplContextCommon: &common}
	bundleData, err := yaml.Marshal(bundle)
	if err != nil {
		t.Fatalf("marshal bundleNGContext: %v", err)
	}
	bundleMap := make(map[string]interface{})
	if err := yaml.Unmarshal(bundleData, &bundleMap); err != nil {
		t.Fatalf("unmarshal bundleNGContext: %v", err)
	}
	if got := bundleMap["clusterUUID"]; got != clusterUUID {
		t.Fatalf("bundleNGContext clusterUUID = %v, want %q", got, clusterUUID)
	}

	bc := bashibleContext{ClusterUUID: input.ClusterUUID}
	bcData, err := yaml.Marshal(bc)
	if err != nil {
		t.Fatalf("marshal bashibleContext: %v", err)
	}
	bcMap := make(map[string]interface{})
	if err := yaml.Unmarshal(bcData, &bcMap); err != nil {
		t.Fatalf("unmarshal bashibleContext: %v", err)
	}
	if got := bcMap["clusterUUID"]; got != clusterUUID {
		t.Fatalf("bashibleContext clusterUUID = %v, want %q", got, clusterUUID)
	}
}

func TestClusterMasterEndpointAddresses(t *testing.T) {
	kubeAPIEndpoints, rppAddresses, rppBootstrapAddresses := clusterMasterEndpointAddresses([]clusterMasterEndpoint{
		{
			Address:                "10.0.0.1",
			KubeAPIPort:            6443,
			RPPServerPort:          4219,
			RPPBootstrapServerPort: testRPPBootstrapServerPort,
		},
		{
			Address:                "10.0.0.2",
			RPPServerPort:          4219,
			RPPBootstrapServerPort: testRPPBootstrapServerPort,
		},
	})

	if got, want := fmt.Sprint(kubeAPIEndpoints), "[10.0.0.1:6443]"; got != want {
		t.Fatalf("kubeAPIEndpoints = %s, want %s", got, want)
	}
	if got, want := fmt.Sprint(rppAddresses), "[10.0.0.1:4219 10.0.0.2:4219]"; got != want {
		t.Fatalf("rppAddresses = %s, want %s", got, want)
	}
	if got, want := fmt.Sprint(rppBootstrapAddresses), fmt.Sprintf("[10.0.0.1:%d 10.0.0.2:%d]", testRPPBootstrapServerPort, testRPPBootstrapServerPort); got != want {
		t.Fatalf("rppBootstrapAddresses = %s, want %s", got, want)
	}
}

func TestBashibleChecksum(t *testing.T) {
	hash := func(t *testing.T, bc *bashibleContext) string {
		h := sha256.New()

		bcDataExpected, err := yaml.Marshal(bc)
		if err != nil {
			t.Fatal("cannot marshal bashible context")
		}

		err = bc.AddToChecksum(h)
		if err != nil {
			t.Fatalf("Add to checksum error: %v", err)
		}

		bcDataAfter, err := yaml.Marshal(bc)
		if err != nil {
			t.Fatal("cannot marshal bashible context")
		}

		if string(bcDataExpected) != string(bcDataAfter) {
			t.Fatal("AddToChecksum should not change object")
		}

		return fmt.Sprintf("%x", h.Sum(nil))
	}

	clusterMasterAddresses := []string{
		"10.0.0.1",
	}

	const ngYaml = `
cloudInstances:
  classReference:
    kind: OpenStackInstanceClass
    name: pico
  maxPerZone: 0
  maxSurgePerZone: 0
  maxUnavailablePerZone: 0
  minPerZone: 0
  zones:
  - nova
cri:
  type: Docker
disruptions:
  approvalMode: Manual
instanceClass:
  flavorName: nm1.small
  imageName: ubuntu-18-04-cloud-amd64
  mainNetwork: sandbox
kubelet:
  containerLogMaxFiles: 4
  containerLogMaxSize: 50Mi
  maxPods: 13
kubernetesVersion: "1.32"
manualRolloutID: ""
name: stage
nodeTemplate:
  labels:
    node-role.aaaaa.io/staging: ""
nodeType: CloudEphemeral
updateEpoch: "1680009541"
`
	ng := make(map[string]interface{})

	err := yaml.Unmarshal([]byte(ngYaml), &ng)
	if err != nil {
		t.Errorf("unmarshal error: %v", err)
		return
	}

	bc := bashibleContext{
		KubernetesVersion: "1.26",
		Normal: map[string]interface{}{
			"apiserverEndpoints": clusterMasterAddresses,
			"clusterMasterEndpoints": []map[string]interface{}{
				{
					"address":                "10.0.0.1",
					"kubeApiPort":            6443,
					"rppServerPort":          4219,
					"rppBootstrapServerPort": testRPPBootstrapServerPort,
				},
			},
		},
		NodeGroup: ng,
		RunType:   "Normal",

		Images: map[string]map[string]string{
			"common": {
				"pause": "c5120536ab49040dbbff34be987469227fd9c241a6fd73da694c13c1-1654517843943",
			},
		},

		Registry: map[string]interface{}{
			"registryModuleEnable": true,
			"mode":                 "unmanaged",
			"version":              "unknown",
			"imagesBase":           "registry.d8-system.svc/deckhouse/system",
			"proxyEndpoints":       []interface{}{"192.168.1.1"},
			"hosts": map[string]interface{}{
				"registry.d8-system.svc": map[string]interface{}{
					"mirrors": []interface{}{
						map[string]interface{}{
							"host":   "r.example.com",
							"scheme": "https",
							"ca":     "==exampleCA==",
							"auth": map[string]interface{}{
								"username": "user",
								"password": "password",
								"auth":     "auth",
							},
							"rewrites": []interface{}{
								map[string]interface{}{
									"from": "^deckhouse/system",
									"to":   "deckhouse/ce",
								},
							},
						},
					},
				},
			},
		},

		Proxy: map[string]interface{}{
			"httpProxy": "proxy.example.com:444",
		},
	}

	expectedHash := hash(t, &bc)

	t.Run("changing counter in cloudInstances object does not affect checksum", func(t *testing.T) {
		bc.NodeGroup["cloudInstances"].(map[string]interface{})["maxPerZone"] = 2
		bc.NodeGroup["cloudInstances"].(map[string]interface{})["maxSurgePerZone"] = 1
		bc.NodeGroup["cloudInstances"].(map[string]interface{})["maxUnavailablePerZone"] = 1
		bc.NodeGroup["cloudInstances"].(map[string]interface{})["minPerZone"] = 1
		bc.NodeGroup["cloudInstances"].(map[string]interface{})["zones"] = []string{"aaaaaa"}

		newHash := hash(t, &bc)

		if expectedHash != newHash {
			t.Errorf("%s != %s", expectedHash, newHash)
			return
		}
	})

	t.Run("changing kubelet seccompDefault affects checksum", func(t *testing.T) {
		bc.NodeGroup["kubelet"].(map[string]interface{})["seccompDefault"] = true

		newHash := hash(t, &bc)

		if expectedHash == newHash {
			t.Fatalf("expected checksum to change when seccompDefault changes")
		}
	})
}

func TestGetCloudProvider(t *testing.T) {
	aws := cloudProvider{"type": "aws", "region": "eu-central-1"}
	yandex := cloudProvider{"type": "yandex"}

	t.Run("a writer that publishes no list still answers with the deprecated field", func(t *testing.T) {
		input := inputData{CloudProvider: aws}

		if got := input.getCloudProvider("aws"); !reflect.DeepEqual(got, aws) {
			t.Fatalf("getCloudProvider(aws) = %v, want the deprecated registration %v", got, aws)
		}
		if got := input.getCloudProvider(""); !reflect.DeepEqual(got, aws) {
			t.Fatalf("getCloudProvider(\"\") = %v, want the deprecated registration %v", got, aws)
		}
	})

	t.Run("a NodeGroup picks the registration of its own type", func(t *testing.T) {
		input := inputData{CloudProviders: []cloudProvider{aws, yandex}}

		if got := input.getCloudProvider("aws"); !reflect.DeepEqual(got, aws) {
			t.Fatalf("getCloudProvider(aws) = %v, want %v", got, aws)
		}
		if got := input.getCloudProvider("yandex"); !reflect.DeepEqual(got, yandex) {
			t.Fatalf("getCloudProvider(yandex) = %v, want %v", got, yandex)
		}
	})

	t.Run("a NodeGroup that names no provider gets none once the deprecated field is gone", func(t *testing.T) {
		input := inputData{CloudProviders: []cloudProvider{aws}}

		if got := input.getCloudProvider(""); got != nil {
			t.Fatalf("getCloudProvider(\"\") = %v, want nil", got)
		}
	})

	t.Run("a NodeGroup that names no provider still gets the deprecated one while it is published", func(t *testing.T) {
		input := inputData{CloudProvider: aws, CloudProviders: []cloudProvider{aws}}

		if got := input.getCloudProvider(""); !reflect.DeepEqual(got, aws) {
			t.Fatalf("getCloudProvider(\"\") = %v, want the deprecated registration %v", got, aws)
		}
	})

	t.Run("a type nobody registered falls back instead of guessing", func(t *testing.T) {
		input := inputData{CloudProvider: aws, CloudProviders: []cloudProvider{aws}}

		if got := input.getCloudProvider("gcp"); !reflect.DeepEqual(got, aws) {
			t.Fatalf("getCloudProvider(gcp) = %v, want the deprecated registration %v", got, aws)
		}
	})
}

func TestCloudProviderIsPerNodeGroupInBuiltContexts(t *testing.T) {
	aws := cloudProvider{"type": "aws", "region": "eu-central-1"}
	yandex := cloudProvider{"type": "yandex"}

	root := t.TempDir()
	writeStepTemplate(t, root, "bashible/common-steps/all/000_common.sh.tpl", "echo common")
	writeStepTemplate(t, root, "cloud-providers/aws/bashible/common-steps/all/010_aws.sh.tpl", "echo aws")
	writeStepTemplate(t, root, "cloud-providers/yandex/bashible/common-steps/all/010_yandex.sh.tpl", "echo yandex")

	t.Run("every group renders the steps of the provider it names", func(t *testing.T) {
		built, steps := buildContexts(t, root, inputData{
			CloudProviders: []cloudProvider{aws, yandex},
			NodeGroups: []nodeGroup{
				{"name": "worker-aws", "nodeType": "CloudEphemeral", "cloudProviderType": "aws"},
				{"name": "worker-yandex", "nodeType": "CloudEphemeral", "cloudProviderType": "yandex"},
				{"name": "static-ng", "nodeType": "Static"},
			},
		})

		if got := bashibleContextOf(t, built, "worker-aws").CloudProviderType; got != "aws" {
			t.Fatalf("worker-aws cloudProviderType = %q, want %q", got, "aws")
		}
		if got := bashibleContextOf(t, built, "worker-yandex").CloudProviderType; got != "yandex" {
			t.Fatalf("worker-yandex cloudProviderType = %q, want %q", got, "yandex")
		}
		if got := bashibleContextOf(t, built, "static-ng").CloudProviderType; got != "" {
			t.Fatalf("static-ng cloudProviderType = %q, want it empty", got)
		}

		if got, want := keysOf(steps["worker-aws"]), []string{"000_common.sh", "010_aws.sh"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("worker-aws steps = %v, want %v", got, want)
		}
		if got, want := keysOf(steps["worker-yandex"]), []string{"000_common.sh", "010_yandex.sh"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("worker-yandex steps = %v, want %v", got, want)
		}
		if got, want := keysOf(steps["static-ng"]), []string{"000_common.sh"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("static-ng steps = %v, want %v", got, want)
		}
	})

	t.Run("the registration reaches the bundle context of its own group only", func(t *testing.T) {
		built, _ := buildContexts(t, root, inputData{
			CloudProviders: []cloudProvider{aws},
			NodeGroups: []nodeGroup{
				{"name": "worker-aws", "nodeType": "CloudEphemeral", "cloudProviderType": "aws"},
				{"name": "static-ng", "nodeType": "Static"},
			},
		})

		if got := bundleContextOf(t, built, "worker-aws").CloudProvider; !reflect.DeepEqual(got, aws) {
			t.Fatalf("worker-aws bundle cloudProvider = %v, want %v", got, aws)
		}
		if got := bundleContextOf(t, built, "static-ng").CloudProvider; got != nil {
			t.Fatalf("static-ng bundle cloudProvider = %v, want nil", got)
		}
	})

	t.Run("a writer from before this contract keeps every group on the cluster provider", func(t *testing.T) {
		built, steps := buildContexts(t, root, inputData{
			CloudProvider: aws,
			NodeGroups: []nodeGroup{
				{"name": "worker-aws", "nodeType": "CloudEphemeral"},
				{"name": "static-ng", "nodeType": "Static"},
			},
		})

		for _, ng := range []string{"worker-aws", "static-ng"} {
			if got := bashibleContextOf(t, built, ng).CloudProviderType; got != "aws" {
				t.Fatalf("%s cloudProviderType = %q, want %q", ng, got, "aws")
			}
			if got, want := keysOf(steps[ng]), []string{"000_common.sh", "010_aws.sh"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("%s steps = %v, want %v", ng, got, want)
			}
		}
	})
}

// buildContexts builds over rootDir and collects the steps rendered per NodeGroup.
func buildContexts(t *testing.T, rootDir string, input inputData) (BashibleContextData, map[string]map[string]string) {
	t.Helper()

	steps := make(map[string]map[string]string)

	cb := NewContextBuilder(context.Background(), NewStepsStorage(context.Background(), rootDir, nil))
	cb.emitStepsOutput = func(ng string, rendered map[string]string) {
		if steps[ng] == nil {
			steps[ng] = make(map[string]string)
		}
		for name, content := range rendered {
			steps[ng][name] = content
		}
	}
	cb.SetInputData(input)

	data, _, errs := cb.Build()
	if len(errs) > 0 {
		t.Fatalf("build errors: %v", errs)
	}

	return data, steps
}

func bashibleContextOf(t *testing.T, data BashibleContextData, ng string) bashibleContext {
	t.Helper()

	bc, ok := data.bashibleContexts[fmt.Sprintf("bashible-%s", ng)].(bashibleContext)
	if !ok {
		t.Fatalf("no bashible context for NodeGroup %q", ng)
	}

	return bc
}

func bundleContextOf(t *testing.T, data BashibleContextData, ng string) bundleNGContext {
	t.Helper()

	bc, ok := data.bashibleContexts[fmt.Sprintf("bundle-%s", ng)].(bundleNGContext)
	if !ok {
		t.Fatalf("no bundle context for NodeGroup %q", ng)
	}

	return bc
}

func writeStepTemplate(t *testing.T, rootDir, path, content string) {
	t.Helper()

	full := filepath.Join(rootDir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func keysOf(m map[string]string) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}
