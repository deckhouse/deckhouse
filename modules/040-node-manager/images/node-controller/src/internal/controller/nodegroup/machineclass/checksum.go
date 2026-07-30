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

package machineclass

import (
	"bytes"
	"fmt"
	"text/template"
)

func buildChecksumElement(instanceClass interface{}, manualRolloutID string) map[string]interface{} {
	return map[string]interface{}{
		"instanceClass":   instanceClass,
		"manualRolloutID": manualRolloutID,
	}
}

// RenderChecksumForInstanceClass renders the checksum from the only two inputs the checksum
// contract allows: the instance class and the manualRolloutID. Production code must use this
// instead of passing the whole resolved NodeGroup map — a provider template reaching for any
// other field (updateEpoch, kubernetesVersion, zones) then fails to render instead of silently
// changing the checksum on unrelated edits, which renames the immutable MachineClass /
// MachineTemplate and rolls every node in the group.
func RenderChecksumForInstanceClass(templateContent []byte, instanceClass interface{}, manualRolloutID string, cloudProvider map[string]interface{}) (string, error) {
	return RenderChecksum(templateContent, buildChecksumElement(instanceClass, manualRolloutID), cloudProvider)
}

// RenderChecksum renders a provider instance-class checksum template. Every template reads
// .nodeGroup; some (vcd CAPI) additionally require
// .Values.nodeManager.internal.cloudProvider.<type> and fail rendering without it, so the
// cloud-provider tree is always part of the context — callers cannot omit it.
func RenderChecksum(templateContent []byte, nodeGroupValues, cloudProvider map[string]interface{}) (string, error) {
	ctx := map[string]interface{}{
		"nodeGroup": nodeGroupValues,
		"Values": map[string]interface{}{
			"nodeManager": map[string]interface{}{
				"internal": map[string]interface{}{
					"cloudProvider": cloudProvider,
				},
			},
		},
	}

	t, err := template.New("machine-class.checksum").Funcs(FuncMap()).Parse(string(templateContent))
	if err != nil {
		return "", fmt.Errorf("parse checksum template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("render checksum template: %w", err)
	}

	checksum := buf.String()
	if checksum == "" {
		return "", fmt.Errorf("empty checksum")
	}
	return checksum, nil
}
