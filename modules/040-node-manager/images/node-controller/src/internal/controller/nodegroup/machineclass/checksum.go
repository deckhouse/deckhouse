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

// Package machineclass ports the get_crds machineclass_checksum hooks
// (modules/040-node-manager/hooks/machineclass_checksum_{assign,collect}.go).
// It renders a cloud-provider's machine-class.checksum Helm template against a
// NodeGroup blob element to produce the checksum stored in the
// checksum/machine-class annotation of a MachineDeployment.
package machineclass

import (
	"bytes"
	"fmt"
	"text/template"
)

// BuildChecksumElement assembles the minimal NodeGroup blob element the checksum
// templates consume. Every provider machine-class.checksum / instance-class.checksum
// reads only .nodeGroup.instanceClass.* and .nodeGroup.manualRolloutID, so passing
// just these two fields yields a checksum byte-identical to rendering the full
// internal.nodeGroups blob element — the caller does not need the whole blob.
//
// instanceClass must be the resolved instance class spec (cloud defaults applied),
// i.e. the value of derived_status.Result.InstanceClass; manualRolloutID is the
// NodeGroup's manual-rollout-id annotation ("" when unset).
func BuildChecksumElement(instanceClass map[string]interface{}, manualRolloutID string) map[string]interface{} {
	return map[string]interface{}{
		"instanceClass":   instanceClass,
		"manualRolloutID": manualRolloutID,
	}
}

// RenderChecksum renders a provider machine-class.checksum template with the same
// engine the assign hook uses (RenderTemplate(content, {"nodeGroup": ng.Raw})),
// where blobElement is the whole internal.nodeGroups blob element built by
// BuildNodeGroupBlob. The rendered output is the checksum verbatim — the template
// ends in `| sha256sum` so the result is the 64-char hex digest with no trailing
// whitespace (the closing `-}}` trims it).
//
// Byte-parity requirement: the template, the FuncMap and the data shape must all
// match the hook, otherwise the checksum diverges and nodes roll.
func RenderChecksum(templateContent []byte, blobElement map[string]interface{}) (string, error) {
	t, err := template.New("machine-class.checksum").Funcs(FuncMap()).Parse(string(templateContent))
	if err != nil {
		return "", fmt.Errorf("parse checksum template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, map[string]interface{}{"nodeGroup": blobElement}); err != nil {
		return "", fmt.Errorf("render checksum template: %w", err)
	}

	checksum := buf.String()
	if checksum == "" {
		return "", fmt.Errorf("empty checksum")
	}
	return checksum, nil
}
