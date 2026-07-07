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
	"fmt"
	"os"
	"path/filepath"
)

// DefaultTemplateBaseDirs mirrors machineclass_checksum_assign.go
// getChecksumTemplatePath: the ordered set of module roots searched for a
// provider's machine-class.checksum. MODULES_DIR cannot be used because it may
// contain several colon-joined paths in an unpredictable order.
var DefaultTemplateBaseDirs = []string{
	"/deckhouse/modules",
	"/deckhouse/ee/modules",
	"/deckhouse/ee/fe/modules",
	"/deckhouse/ee/se-plus/modules",
}

// FallbackTemplateBaseDir is the generated-path root used when the provider
// template is not found under any of the module roots
// (getChecksumTemplatePath's unchecked fallback).
const FallbackTemplateBaseDir = "/deckhouse/modules/040-node-manager/cloud-providers"

// Checksum template sub-paths within a 030-cloud-provider-<type> module. The two
// provisioning modes use distinct templates rendered by the same engine:
//   - MCM  MachineClass    → checksum/machine-class   (machineclass_checksum hooks)
//   - CAPI MachineTemplate → checksum/instance-class  (node-group _capi_machine_template.tpl)
const (
	MCMChecksumSubPath  = "cloud-instance-manager/machine-class.checksum"
	CAPIChecksumSubPath = "capi/instance-class.checksum"
)

// MCMMachineClassSubPath is the provider MachineClass manifest template
// (the get_crds node_group_machine_class define) within a 030-cloud-provider-<type>
// module. It is resolved with the same base-dir search as the checksum templates.
const MCMMachineClassSubPath = "cloud-instance-manager/machine-class.yaml"

// ResolveChecksumTemplatePath returns the path to the checksum template (subPath,
// e.g. MCMChecksumSubPath or CAPIChecksumSubPath) for cloudType, mirroring the
// assign hook: it returns the first path that exists under baseDirs, otherwise the
// generated fallback path (returned unchecked, exactly like the hook — a
// subsequent read surfaces the miss).
func ResolveChecksumTemplatePath(baseDirs []string, fallbackBaseDir, cloudType, subPath string) string {
	provider := fmt.Sprintf("030-cloud-provider-%s", cloudType)
	for _, dir := range baseDirs {
		p := filepath.Join(dir, provider, subPath)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join(fallbackBaseDir, cloudType, filepath.Base(subPath))
}

// ReadChecksumTemplate resolves and reads the subPath checksum template for
// cloudType. cloudType must be non-empty (it comes from the decoded cloud-provider
// secret's .type); an empty value means the cloud branch should not run at all.
func ReadChecksumTemplate(baseDirs []string, fallbackBaseDir, cloudType, subPath string) ([]byte, error) {
	if cloudType == "" {
		return nil, fmt.Errorf("cloud type not set")
	}
	path := ResolveChecksumTemplatePath(baseDirs, fallbackBaseDir, cloudType, subPath)
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read checksum template for cloud type %q: %w", cloudType, err)
	}
	return content, nil
}
