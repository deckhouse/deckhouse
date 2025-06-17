package types

import "strings"

const (
	BundleMinimal = "minimal"
	BundleManaged = "managed"
	BundleDefault = "default"
)

type Bundle string

func (b Bundle) String() string {
	return string(b)
}

func (b Bundle) IsValid() bool {
	switch strings.ToLower(b.String()) {
	case BundleMinimal, BundleManaged, BundleDefault:
		return true
	default:
		return false
	}
}

// named types if we want to use custom logic like validation
type FeatureFlag string

type EeNetworking struct {
	Available    bool          `json:"available,omitempty" yaml:"available,omitempty"`
	Enabled      bool          `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	FeatureFlags []FeatureFlag `json:"featureFlags,omitempty" yaml:"featureFlags,omitempty"`
}

type Batches struct {
	EeNetworking EeNetworking `json:"ee-networking,omitempty" yaml:"ee-networking,omitempty"`
}

type Default struct {
	Available        bool          `json:"available,omitempty" yaml:"available,omitempty"`
	EnabledInBundles []FeatureFlag `json:"enabledInBundles,omitempty" yaml:"enabledInBundles,omitempty"`
}

type Ee struct {
	Available        bool          `json:"available,omitempty" yaml:"available,omitempty"`
	EnabledInBundles []Bundle      `json:"enabledInBundles,omitempty" yaml:"enabledInBundles,omitempty"`
	FeatureFlags     []FeatureFlag `json:"featureFlags,omitempty" yaml:"featureFlags,omitempty"`
}

type Se struct {
	Available        bool          `json:"available,omitempty" yaml:"available,omitempty"`
	EnabledInBundles []Bundle      `json:"enabledInBundles,omitempty" yaml:"enabledInBundles,omitempty"`
	FeatureFlags     []FeatureFlag `json:"featureFlags,omitempty" yaml:"featureFlags,omitempty"`
}

type Be struct {
	Available bool `json:"available,omitempty" yaml:"available,omitempty"`
}

type Editions struct {
	Default Default `json:"_default,omitempty" yaml:"_default,omitempty"`
	Ee      Ee      `json:"ee,omitempty" yaml:"ee,omitempty"`
	Se      Se      `json:"se,omitempty" yaml:"se,omitempty"`
	Be      Be      `json:"be,omitempty" yaml:"be,omitempty"`
}

type Accessibility struct {
	// TODO: change Batches name to the new one.
	Batches  Batches  `json:"batches,omitempty" yaml:"batches,omitempty"`
	Editions Editions `json:"editions,omitempty" yaml:"editions,omitempty"`
}
