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

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	valuesPath        = "registry.internal.takeover"
	migrateConfigPath = "registry.migrate"
	queue             = "/modules/registry/takeover"

	// Order 5 runs before the legacy orchestrator (Order 10), so the phase is
	// set before the orchestrator decides whether to yield — even on the very
	// first reconcile of a fresh install.
	hookOrder = 5

	takeoverSnap     = "takeover"
	oldStateSnap     = "old-state"
	legacyConfigSnap = "legacy-config"

	agentDSSnap    = "agent-ds"
	cacheSTSSnap   = "cache-sts"
	cacheLeaseSnap = "cache-lease"
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: hookOrder},
		Queue:        queue,
		Kubernetes:   KubernetesConfigs(takeoverSnap, oldStateSnap, legacyConfigSnap, agentDSSnap, cacheSTSSnap, cacheLeaseSnap),
	},
	handle,
)

func handle(_ context.Context, input *go_hook.HookInput) error {
	values := helpers.NewValuesAccessor[Values](input, valuesPath)
	prev := values.Get()

	in := Inputs{PrevPhase: prev.Phase}
	prevFlippedAt := prev.FlippedAt

	var storedDerived *DerivedConfig
	var storedStartedAt string
	if stored, err := helpers.SnapshotToSingle[Values](input, takeoverSnap); err == nil {
		in.StoredPhase = stored.Phase
		if prevFlippedAt == "" {
			prevFlippedAt = stored.FlippedAt
		}
		storedDerived = stored.Derived
		storedStartedAt = stored.TakingOverStartedAt
	} else if !errors.Is(err, helpers.ErrNoSnapshot) {
		return fmt.Errorf("get registry-takeover snapshot: %w", err)
	}

	// A list snapshot of an absent secret yields an empty list (not ErrNoSnapshot),
	// so any error here is genuine — unlike the single-snapshot read above.
	names, err := helpers.SnapshotToList[string](input, oldStateSnap)
	if err != nil {
		return fmt.Errorf("get registry-state snapshot: %w", err)
	}
	in.OldStatePresent = len(names) > 0

	// migrate is the explicit operator opt-in. A read error (unset key) leaves
	// Migrate=false — the no-surprise-flip fail-safe.
	if migrate, err := helpers.GetValue[bool](input, migrateConfigPath); err == nil {
		in.Migrate = migrate
	}

	// Resolve the persisted derived config (values store first, then the
	// durable secret snapshot). Must happen before Transition so that
	// cacheExpected can be computed correctly.
	persistedDerived := prev.Derived
	if persistedDerived == nil {
		persistedDerived = storedDerived
	}

	// Read the legacy desired-state config, if present.
	var legacy *LegacyConfig
	if lc, err := helpers.SnapshotToSingle[LegacyConfig](input, legacyConfigSnap); err == nil {
		legacy = &lc
	} else if !errors.Is(err, helpers.ErrNoSnapshot) {
		return fmt.Errorf("get registry-config snapshot: %w", err)
	}

	// Operator cache flag (used only when no derived config is present).
	operatorCacheEnabled, _ := helpers.GetValue[bool](input, "registry.cache.enabled")

	var dsStatus *AgentDSStatus
	if v, err := helpers.SnapshotToSingle[AgentDSStatus](input, agentDSSnap); err == nil {
		dsStatus = &v
	} else if !errors.Is(err, helpers.ErrNoSnapshot) {
		return fmt.Errorf("get agent DaemonSet snapshot: %w", err)
	}
	var stsStatus *CacheSTSStatus
	if v, err := helpers.SnapshotToSingle[CacheSTSStatus](input, cacheSTSSnap); err == nil {
		stsStatus = &v
	} else if !errors.Is(err, helpers.ErrNoSnapshot) {
		return fmt.Errorf("get cache StatefulSet snapshot: %w", err)
	}
	var leaseStatus *CacheLeaseStatus
	if v, err := helpers.SnapshotToSingle[CacheLeaseStatus](input, cacheLeaseSnap); err == nil {
		leaseStatus = &v
	} else if !errors.Is(err, helpers.ErrNoSnapshot) {
		return fmt.Errorf("get cache Lease snapshot: %w", err)
	}

	now := time.Now().UTC()
	// Resolve derived against the previous phase so cacheExpected is accurate
	// before we know the new phase. After Transition, re-resolve with the new
	// phase so the persisted value reflects the correct post-flip state.
	derived := resolveDerived(in.PrevPhase, legacy, persistedDerived)

	// storeSynced is set by the Order-3 store-sync hook; default false on error.
	storeSynced, _ := helpers.GetValue[bool](input, "registry.internal.takeover.storeSynced")
	airgap := isAirgap(derived)

	in.VerifyReady = computeVerifyReady(dsStatus, stsStatus, leaseStatus, cacheExpected(derived, operatorCacheEnabled), airgap, storeSynced, now)

	// Resolve the prior startedAt: values store first (in-memory, fastest),
	// then fall back to the durable secret snapshot.
	prevStartedAt := prev.TakingOverStartedAt
	if prevStartedAt == "" {
		prevStartedAt = storedStartedAt
	}

	// Compute DeadlineExceeded BEFORE Transition so the edge is visible to it.
	timeout := parseMigrateTimeout(input)
	in.DeadlineExceeded = resolveCurrent(in) == helpers.PhaseTakingOver &&
		prevStartedAt != "" &&
		deadlineExceeded(prevStartedAt, now, timeout)

	// Compute CleanupReady BEFORE Transition so the New -> CleanupPending edge
	// is visible to it.
	in.CleanupReady = resolveCurrent(in) == helpers.PhaseNew &&
		prevFlippedAt != "" &&
		cleanupReady(prevFlippedAt, now, parseCleanupAfter(input))

	phase := Transition(in)
	flippedAt := resolveFlippedAt(phase, prevFlippedAt, now.Format(time.RFC3339))
	startedAt := resolveStartedAt(phase, prevStartedAt, now.Format(time.RFC3339))

	derived = resolveDerived(phase, legacy, persistedDerived)

	values.Set(Values{
		Phase:               phase,
		FlippedAt:           flippedAt,
		TakingOverStartedAt: startedAt,
		Derived:             derived,
	})
	return nil
}

// resolveStartedAt stamps now on entry to TakingOver, preserves it while in
// TakingOver, and clears it in any other phase.
func resolveStartedAt(phase, prev, now string) string {
	if phase != helpers.PhaseTakingOver {
		return ""
	}
	if prev == "" {
		return now
	}
	return prev
}

// deadlineExceeded reports whether now is more than timeout after the RFC3339
// startedAt. An unparseable startedAt is treated as not-exceeded (fail-safe).
func deadlineExceeded(startedAt string, now time.Time, timeout time.Duration) bool {
	t, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return false
	}
	return now.Sub(t) > timeout
}

// parseMigrateTimeout reads registry.migrateTimeout as a Go duration, defaulting
// to 30m on absence or parse error.
func parseMigrateTimeout(input *go_hook.HookInput) time.Duration {
	const def = 30 * time.Minute
	raw, err := helpers.GetValue[string](input, "registry.migrateTimeout")
	if err != nil || raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return def
	}
	return d
}

// resolveDerived returns the derived new-arch config: a fresh derivation from
// the live legacy config while migrating (phase != Legacy) when there is
// something to translate, otherwise the persisted value (never blanked).
func resolveDerived(phase string, legacy *LegacyConfig, persisted *DerivedConfig) *DerivedConfig {
	if phase != helpers.PhaseLegacy && legacy != nil {
		if d := deriveFromLegacy(*legacy); d.Present {
			return &d
		}
	}
	return persisted
}

// resolveFlippedAt stamps now the first time the cluster enters New and
// otherwise preserves the prior value. It never stamps for non-New phases.
func resolveFlippedAt(phase, prev, now string) string {
	if phase == helpers.PhaseNew && prev == "" {
		return now
	}
	return prev
}

// cleanupReady reports whether the cluster has been in New longer than `after`
// (measured from flippedAt). An empty/unparseable flippedAt is not ready.
func cleanupReady(flippedAt string, now time.Time, after time.Duration) bool {
	t, err := time.Parse(time.RFC3339, flippedAt)
	if err != nil {
		return false
	}
	return now.Sub(t) > after
}

func parseCleanupAfter(input *go_hook.HookInput) time.Duration {
	const def = 168 * time.Hour
	raw, err := helpers.GetValue[string](input, "registry.cleanupAfter")
	if err != nil || raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return def
	}
	return d
}
