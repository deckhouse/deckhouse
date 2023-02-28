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

package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

func Test_certificateManifest(t *testing.T) {
	manifest := certificateManifest("xyz", "big-xyz", "somens")

	// agentID, name, namespace string
	expected := `
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: "xyz"
    upmeter-group: control-plane
    upmeter-probe: cert-manager
  name: "big-xyz"
  namespace: "somens"
spec:
  certificateOwnerRef: true
  dnsNames:
  - nothing-xyz.example.com
  issuerRef:
    kind: ClusterIssuer
    name: selfsigned
  secretName: "big-xyz"
  secretTemplate:
    labels:
      heritage: upmeter
      upmeter-agent: "xyz"
      upmeter-group: control-plane
      upmeter-probe: cert-manager
`
	assert.Equal(t, expected, manifest)

	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(manifest), obj)
	assert.NoError(t, err, "YAML is expected to be valid")
}
