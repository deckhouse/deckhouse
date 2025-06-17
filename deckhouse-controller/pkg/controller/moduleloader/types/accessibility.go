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

func (f FeatureFlag) String() string {
	return string(f)
}

type Batch struct {
	Available    bool          `json:"available,omitempty" yaml:"available,omitempty"`
	Enabled      bool          `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	FeatureFlags []FeatureFlag `json:"featureFlags,omitempty" yaml:"featureFlags,omitempty"`
}

type Batches map[string]Batch

type Edition struct {
	Available        bool          `json:"available,omitempty" yaml:"available,omitempty"`
	EnabledInBundles []Bundle      `json:"enabledInBundles,omitempty" yaml:"enabledInBundles,omitempty"`
	FeatureFlags     []FeatureFlag `json:"featureFlags,omitempty" yaml:"featureFlags,omitempty"`
}

type Editions struct {
	Default *Edition `json:"_default,omitempty" yaml:"_default,omitempty"`
	Ee      *Edition `json:"ee,omitempty" yaml:"ee,omitempty"`
	Se      *Edition `json:"se,omitempty" yaml:"se,omitempty"`
	Be      *Edition `json:"be,omitempty" yaml:"be,omitempty"`
}

type Accessibility struct {
	// TODO: change Batches name to the new one.
	Batches  *Batches  `json:"batches,omitempty" yaml:"batches,omitempty"`
	Editions *Editions `json:"editions,omitempty" yaml:"editions,omitempty"`
}

func (a Accessibility) HasAccess() string {
	res := &strings.Builder{}

	editions := make(map[string]*Edition, 1)

	if a.Editions.Default != nil {
		editions["ee"] = a.Editions.Default
		editions["se"] = a.Editions.Default
		editions["be"] = a.Editions.Default
	}

	if a.Editions.Ee != nil {
		editions["ee"] = a.Editions.Ee
	}

	if a.Editions.Se != nil {
		editions["se"] = a.Editions.Se
	}

	if a.Editions.Be != nil {
		editions["be"] = a.Editions.Be
	}

	for edition, e := range editions {
		if e != nil && e.Available {
			res.WriteString("available in " + edition + " edition, ")
		}

		bundles := make([]string, len(e.EnabledInBundles))
		for _, ff := range e.EnabledInBundles {
			if ff != "" {
				bundles = append(bundles, ff.String())
			}
		}

		if len(bundles) > 0 {
			res.WriteString("enabled in " + strings.Join(bundles, ",") + " bundles, ")
		}

		ff := make([]string, len(e.FeatureFlags))
		for _, f := range e.FeatureFlags {
			if f != "" {
				ff = append(ff, f.String())
			}
		}

		if len(ff) > 0 {
			res.WriteString("requires feature flags: " + strings.Join(ff, ",") + ".\n")
		}
	}

	return res.String()
}
