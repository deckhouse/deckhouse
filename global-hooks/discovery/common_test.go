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
	"encoding/base64"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

// ClusterConfiguration fixtures shared by cluster_configuration_test.go (which drives the
// ClusterConfiguration discovery hook) and target_kubernetes_version_test.go (which drives the
// Kubernetes version hook). Both hooks watch the same Secret, so both suites need these.
const (
	ccStateAClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
cloud:
  provider: OpenStack
  prefix: kube
podSubnetCIDR: 10.111.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.33"
clusterDomain: "test.local"
`

	ccStateBClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: AWS
  prefix: lube
podSubnetCIDR: 10.122.0.0/16
podSubnetNodeCIDRPrefix: "26"
serviceSubnetCIDR: 10.213.0.0/16
kubernetesVersion: "1.33"
clusterDomain: "test.local"
`

	ccStateCClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: AWS
  prefix: lube
podSubnetCIDR: 10.122.0.0/16
podSubnetNodeCIDRPrefix: "26"
serviceSubnetCIDR: 10.213.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "test.local"
`
)

// clusterConfigurationSecret wraps a ClusterConfiguration document into the Secret the hooks watch.
func clusterConfigurationSecret(doc string) string {
	return `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(doc))
}

// moduleConfigYAML renders ModuleConfig/control-plane-manager; an empty version leaves settings out
// entirely, which is the "operator has not migrated yet" state.
func moduleConfigYAML(version string) string {
	settings := ""
	if version != "" {
		settings = fmt.Sprintf("\n  settings:\n    kubernetesVersion: %q", version)
	}
	return fmt.Sprintf(`
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  enabled: true
  version: 1%s
`, settings)
}
