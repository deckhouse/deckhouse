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

package bootstrap

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// EC2 refuses an instance whose user data is longer than this, both as it is
// ("User data is limited to 16384 bytes") and base64-encoded by
// machine-controller-manager ("Encoded User data is limited to 25600 bytes").
// The first is the stricter one.
const (
	awsUserDataLimit        = 16384
	awsEncodedUserDataLimit = 25600
)

// mingetGzipSize is minget, as `pm install minget` installs it (2400 bytes),
// after gzip -9: what EncodeMinget base64-encodes. Measured on 2026-11-25.
const mingetGzipSize = 1559

// The whole of phase 1 travels in userData: the bash library it needs before it can
// fetch phase 2, minget, the provider's network script. That is why lib.sh.tpl
// keeps the comments of those parts as {{/* */}}, which are not rendered, and why
// minget goes in gzipped.
func TestAWSUserDataFitsTheEC2Limit(t *testing.T) {
	files := &Files{text: map[string]string{
		"lib.sh.tpl":                        liveTemplate(t, "candi/bashible/lib.sh.tpl"),
		"01-bootstrap-prerequisites.sh.tpl": liveTemplate(t, "candi/bashible/bootstrap/01-bootstrap-prerequisites.sh.tpl"),
		"bootstrap-networks-aws.sh.tpl":     liveTemplate(t, "modules/030-cloud-provider-aws/candi/bashible/bootstrap-networks.sh.tpl"),
	}}

	in := mcmInput(files)
	in.NodeGroup = map[string]any{"name": "worker", "nodeType": "CloudEphemeral", "cloudProviderType": "aws"}
	// Sizes of a real cluster, not of the goldens' stubs: three masters, sha256 digests,
	// a PEM cluster CA, and minget gzipped. Its bytes do not compress further, so
	// any of that length stand for it.
	in.APIServerEndpoints = []string{"10.0.0.1:6443", "10.0.0.2:6443", "10.0.0.3:6443"}
	in.ClusterMasterEndpoints = nil
	for _, address := range []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"} {
		in.ClusterMasterEndpoints = append(in.ClusterMasterEndpoints, map[string]any{
			"address": address, "kubeApiPort": int64(6443), "rppServerPort": int64(4219), "rppBootstrapServerPort": int64(4220),
		})
	}
	digest := "sha256:" + strings.Repeat("0", 64)
	in.Images = map[string]any{"registrypackages": map[string]any{
		"jq": digest, "d8Curl": digest, "tailLog": digest, "rppGet": digest, "ec2DescribeTagsV001Flant3": digest,
	}}
	in.MingetB64 = base64.StdEncoding.EncodeToString(make([]byte, mingetGzipSize))
	in.KubernetesCA = "-----BEGIN CERTIFICATE-----\n" + strings.Repeat(strings.Repeat("A", 64)+"\n", 17) + "-----END CERTIFICATE-----\n"

	userData, err := RenderCloudConfig(in)
	require.NoError(t, err)

	encoded := base64.StdEncoding.EncodedLen(len(userData))
	t.Logf("AWS userData: %d bytes of %d, %d encoded of %d", len(userData), awsUserDataLimit, encoded, awsEncodedUserDataLimit)
	const advice = "phase 1 no longer fits EC2 user data; keep what a node does not need before phase 2 out of it"
	require.LessOrEqual(t, len(userData), awsUserDataLimit, advice)
	require.LessOrEqual(t, encoded, awsEncodedUserDataLimit, advice)
}
