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

package controlplaneoperation

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/checksum"
	"control-plane-manager/internal/constants"
)

func TestRecordChangedFiles(t *testing.T) {
	t.Parallel()

	manifest := constants.ManifestsPath + "/kube-apiserver.yaml"
	extraFile := constants.ExtraFilesPath + "/authorization-config.yaml"
	sensitiveFile := constants.ExtraFilesPath + "/secret-encryption-config.yaml"
	state := controlplanev1alpha1.NewOperationState(&controlplanev1alpha1.ControlPlaneOperation{})

	recordChangedFiles(state, []fileWriteResult{
		{Path: manifest, Changed: true, Diff: "-old\n+new\n"},
		{Path: extraFile, Changed: true, Diff: "+authorizers: []\n"},
		{Path: sensitiveFile, Changed: true, Diff: "+secret: key\n"},
		{Path: constants.ExtraFilesPath + "/scheduler-config.yaml", Changed: false},
	})
	require.Equal(t, []controlplanev1alpha1.ChangedFile{
		{Path: manifest, Diff: "-old\n+new\n"},
		{Path: extraFile, Diff: "+authorizers: []\n"},
		{Path: sensitiveFile},
	}, state.Raw().Status.ChangedFiles)

	// A later step replaces the entry with the same path and cuts an oversized diff at a line boundary.
	line := strings.Repeat("x", 99) + "\n"
	recordChangedFiles(state, []fileWriteResult{
		{Path: manifest, Changed: true, Diff: strings.Repeat(line, maxPublishedDiffBytes/len(line)+1)},
	})
	files := state.Raw().Status.ChangedFiles
	require.Len(t, files, 3)
	require.LessOrEqual(t, len(files[0].Diff), maxPublishedDiffBytes)
	require.True(t, strings.HasSuffix(files[0].Diff, "\n"+diffTruncatedNote))
}

// TestExtraFilesClassified makes a new extra-file fail until it is classified:
// published, or kept on the node because it may hold sensitive data.
func TestExtraFilesClassified(t *testing.T) {
	t.Parallel()

	sensitiveExtraFiles := map[string]bool{
		// The secret encryption key.
		"secret-encryption-config.yaml": true,
		// Rules from the kube-system/audit-policy Secret, may name real users.
		"audit-policy.yaml": true,
		// The webhook URL from ModuleConfig is a free string and may carry credentials.
		"authn-webhook-config.yaml": true,
		"webhook-config.yaml":       true,
		// The OIDC CA from ModuleConfig is a free string: kube-apiserver skips a private key pasted along with it.
		"authentication-config.yaml": true,
		"oidc-ca.crt":                true,
		// The extender path and CA come from KubeSchedulerWebhookConfiguration as free strings.
		"scheduler-config.yaml": true,
		// The URL and CA come from a ConfigMap of the runtime-audit-engine module, outside this repository.
		"audit-webhook-config.yaml": true,
	}
	for _, podComponent := range controlplanev1alpha1.ComponentRegistry() {
		for _, key := range checksum.ExtraFileKeysForPodComponent(podComponent) {
			name := strings.TrimPrefix(key, "extra-file-")
			require.Truef(t, publishedExtraFiles[name] != sensitiveExtraFiles[name],
				"extra-file %s: add it to publishedExtraFiles, or to sensitiveExtraFiles if it may hold sensitive data", name)
		}
	}
}

func TestComputeUnifiedDiffNoPhantomLastLine(t *testing.T) {
	t.Parallel()

	require.Equal(t, "--- f\n+++ f (new)\n@@ -1,2 +1,3 @@\n a\n b\n+status: {}\n",
		computeUnifiedDiff("a\nb\n", "a\nb\nstatus: {}\n", "f"))
	require.Equal(t, "--- f\n+++ f (new)\n@@ -0,0 +1 @@\n+a\n",
		computeUnifiedDiff("", "a\n", "f"))
	require.Equal(t, "--- f\n+++ f (new)\n@@ -1 +0,0 @@\n-a\n",
		computeUnifiedDiff("a", "", "f"))
}
