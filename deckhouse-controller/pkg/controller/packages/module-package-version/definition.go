// Copyright 2025 Flant JSC
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

package modulepackageversion

// PackageDefinition represents the parsed content of package.yaml from v2 module packages.
type PackageDefinition struct {
	Name         string               `yaml:"name"`
	Description  *PackageDescription  `yaml:"description"`
	Category     string               `yaml:"category"`
	Stage        string               `yaml:"stage"`
	Type         string               `yaml:"type"`
	Version      string               `yaml:"version"`
	Requirements *PackageRequirements `yaml:"requirements"`
	Licensing    *PackageLicensing    `yaml:"licensing"`

	VersionCompatibilityRules *VersionCompatibilityRules `yaml:"versionCompatibilityRules"`
}

type PackageDescription struct {
	Ru string `yaml:"ru"`
	En string `yaml:"en"`
}

type PackageRequirements struct {
	Deckhouse  string            `yaml:"deckhouse"`
	Kubernetes string            `yaml:"kubernetes"`
	Modules    map[string]string `yaml:"modules"`
}

type PackageLicensing struct {
	Editions         map[string]PackageEdition `yaml:"editions"`
	EnabledInBundles []string                  `yaml:"enabledInBundles"`
}

type PackageEdition struct {
	Available bool `yaml:"available"`
}

type VersionCompatibilityRules struct {
	Upgrade   UpgradeRules   `yaml:"upgrade"`
	Downgrade DowngradeRules `yaml:"downgrade"`
}

type UpgradeRules struct {
	From             string `yaml:"from"`
	AllowSkipPatches uint   `yaml:"allowSkipPatches"`
	AllowSkipMinor   uint   `yaml:"allowSkipMinor"`
	AllowSkipMajor   uint   `yaml:"allowSkipMajor"`
}

type DowngradeRules struct {
	To               string `yaml:"to"`
	AllowSkipPatches uint   `yaml:"allowSkipPatches"`
	AllowSkipMinor   uint   `yaml:"allowSkipMinor"`
	AllowSkipMajor   uint   `yaml:"allowSkipMajor"`
	MaxRollback      uint   `yaml:"maxRollback"`
}
