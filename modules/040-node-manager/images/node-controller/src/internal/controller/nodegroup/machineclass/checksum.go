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

func BuildChecksumElement(instanceClass map[string]interface{}, manualRolloutID string) map[string]interface{} {
	return map[string]interface{}{
		"instanceClass":   instanceClass,
		"manualRolloutID": manualRolloutID,
	}
}

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
