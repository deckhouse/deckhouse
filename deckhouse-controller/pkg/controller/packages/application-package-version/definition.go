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
	Name        string              `yaml:"name"`        // Package name (required)
	Description *PackageDescription `yaml:"description"` // Description for catalog/UI (required)
	// Package category for classification like "Databases", "Monitoring", etc... (required)
	Category string `yaml:"category"`
	// Maturity stage, like "Preview" (required)
	Stage string `yaml:"stage"`
	// Package type, must be one of: Package, ClusterApplication, Application (required)
	Type PackageType `yaml:"type"`
	// Package version (required, injected during build)
	Version string `yaml:"version"`
	// Environment requirements (optional)
	Requirements *PackageRequirements `yaml:"requirements"`
	// Package availability by editions (optional)
	Licensing *PackageLicensing `yaml:"licensing"`
	// Rules for upgrade and downgrade
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
	Editions map[string]PackageEdition `yaml:"editions"`
	// Only for modules, array of bundles, where module enabled by default
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
