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

var DefaultTemplateBaseDirs = []string{
	"/deckhouse/modules",
	"/deckhouse/ee/modules",
	"/deckhouse/ee/fe/modules",
	"/deckhouse/ee/se-plus/modules",
}

const FallbackTemplateBaseDir = "/deckhouse/modules/040-node-manager/cloud-providers"

const (
	MCMChecksumSubPath  = "cloud-instance-manager/machine-class.checksum"
	CAPIChecksumSubPath = "capi/instance-class.checksum"
)

const MCMMachineClassSubPath = "cloud-instance-manager/machine-class.yaml"

const CAPIMachineTemplateSubPath = "capi/machine-template.yaml"

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
