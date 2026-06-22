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

package takeover

// Values is the persisted shape of the takeover state, both in the
// registry-takeover secret (data.phase / data.flippedAt / data.derived) and in
// registry.internal.takeover.
type Values struct {
	Phase string `json:"phase"`
	// FlippedAt is the RFC3339 timestamp at which the cluster entered the New
	// phase. Empty until the flip completes; set once and then preserved.
	FlippedAt string `json:"flippedAt,omitempty"`
	// TakingOverStartedAt is the RFC3339 time the cluster entered TakingOver,
	// used to enforce migrateTimeout. Empty outside TakingOver.
	TakingOverStartedAt string `json:"takingOverStartedAt,omitempty"`
	// Derived is the new-arch config auto-translated from the legacy
	// registry-config while migrating. Persisted so it survives the cleanup
	// release deleting registry-config. nil until the first derivation.
	Derived *DerivedConfig `json:"derived,omitempty"`
}

// LegacyConfig is the flat desired-state the legacy orchestrator reads from the
// registry-config secret (d8-system).
type LegacyConfig struct {
	Mode       string
	ImagesRepo string
	Scheme     string
	CA         string
	Username   string
	Password   string
	TTL        string
}

// DerivedConfig is the new-arch desired state auto-translated from LegacyConfig.
// Present is false when there is nothing to translate (Unmanaged / unknown), in
// which case the operator's mc/registry config flows through unchanged.
type DerivedConfig struct {
	Present  bool             `json:"present"`
	Upstream *DerivedUpstream `json:"upstream,omitempty"`
	Cache    DerivedCache     `json:"cache"`
}

// DerivedUpstream mirrors the RegistryConfig CR upstream shape. Absent (nil)
// means air-gap (legacy Local mode): served from the on-master cache only.
type DerivedUpstream struct {
	Host        string              `json:"host"`
	Path        string              `json:"path,omitempty"`
	Scheme      string              `json:"scheme,omitempty"`
	CA          string              `json:"ca,omitempty"`
	Credentials *DerivedCredentials `json:"credentials,omitempty"`
}

type DerivedCredentials struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type DerivedCache struct {
	Enabled bool   `json:"enabled"`
	TTL     string `json:"ttl,omitempty"`
}

// Inputs is everything Transition needs to resolve the next phase.
type Inputs struct {
	// StoredPhase is the phase read from the registry-takeover secret; empty if
	// the secret does not exist yet.
	StoredPhase string
	// PrevPhase is the phase from the values store on the previous reconcile;
	// empty on the first run before the secret round-trips.
	PrevPhase string
	// OldStatePresent is true when the legacy orchestrator's registry-state
	// secret exists (an upgraded cluster that ran the old arch).
	OldStatePresent bool
	// Migrate is the operator's explicit opt-in (mc/registry settings.migrate).
	// It advances Legacy -> TakingOver and, while false during TakingOver,
	// rolls the flip back to Legacy.
	Migrate bool
	// VerifyReady reports that the new stack (cache + agents) is proven ready to
	// take over. Gates TakingOver -> New. Computed by computeVerifyReady from the
	// agent DaemonSet and (when caching) the cache StatefulSet + leader Lease.
	VerifyReady bool
	// DeadlineExceeded is true when the cluster has been in TakingOver longer
	// than migrateTimeout without reaching VerifyReady; it rolls the flip back.
	DeadlineExceeded bool
	// CleanupReady reports that the cluster has been in New longer than
	// cleanupAfter; it advances New -> CleanupPending (legacy teardown).
	CleanupReady bool
}
