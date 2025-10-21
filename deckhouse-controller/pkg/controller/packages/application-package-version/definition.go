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

package applicationpackageversion

type PackageType string

var (
	PackageTypeModule             PackageType = "Package"
	PackageTypeClusterApplication PackageType = "ClusterApplication"
	PackageTypeApplication        PackageType = "Application"
)

// Definition of package.yaml file
type PackageDefinition struct {
	Name        string              `yaml:"name"`
	Description *PackageDescription `yaml:"description"`
	// Package category for classification like "Databases", "Monitoring", etc...
	Category string `yaml:"category"`
	// Maturity stage, like "Preview"
	Stage string `yaml:"stage"`
	// Package type, must be one of: Package, ClusterApplication, Application
	Type    PackageType `yaml:"type"`
	Version string      `yaml:"version"`
	// Environment requirements (+optional)
	// TODO: this implemet is incorrect, fix
	// requirements:                         # environment requirements (+optional)
	//   deckhouse: ">= 1.70"
	//   kubernetes: ">= 1.31"
	//   modules:
	//     cert-manager: ">= 1.0.0"
	Requirements map[string]string `yaml:"requirements"`
	// Package availability by editions
	Licensing PackageLicensing `yaml:"licensing"`
	// Rules for upgrade and downgrade
	VersionCompatibilityRules VersionCompatibilityRules `yaml:"versionCompatibilityRules"`
}

type PackageDescription struct {
	Ru string `yaml:"ru"`
	En string `yaml:"en"`
}

type PackageLicensing struct {
	Editions map[string]PackageEdition `yaml:"editions"`
	// Only for modules, array of bundles, where module enabled by deafult
	EnabledInBundles []string `yaml:"enabledInBundles"`
}

type PackageEdition struct {
	Available bool `yaml:"available"`
	// EnabledInBundles []string `yaml:"enabledInBundles"`
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
