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
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

func TestDeadlineExceeded(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		startedAt string
		timeout   time.Duration
		want      bool
	}{
		{"well past deadline", "2026-06-21T10:00:00Z", time.Hour, true},
		{"not yet", "2026-06-21T11:45:00Z", time.Hour, false},
		{"exactly at deadline is not exceeded", "2026-06-21T11:00:00Z", time.Hour, false},
		{"unparseable startedAt -> fail-safe false", "not-a-time", time.Hour, false},
		{"empty startedAt -> fail-safe false", "", time.Hour, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deadlineExceeded(tc.startedAt, now, tc.timeout); got != tc.want {
				t.Errorf("deadlineExceeded(%q, now, %s) = %v, want %v", tc.startedAt, tc.timeout, got, tc.want)
			}
		})
	}
}

func TestResolveDerived(t *testing.T) {
	directLegacy := &LegacyConfig{Mode: "Direct", ImagesRepo: "r.io/p", Scheme: "HTTPS"}
	persistedProxy := &DerivedConfig{Present: true, Cache: DerivedCache{Enabled: true}}

	t.Run("legacy phase: never derive, no persisted -> nil", func(t *testing.T) {
		if got := resolveDerived(helpers.PhaseLegacy, directLegacy, nil); got != nil {
			t.Errorf("expected nil during Legacy, got %+v", got)
		}
	})
	t.Run("migrating + legacy present -> fresh derive", func(t *testing.T) {
		got := resolveDerived(helpers.PhaseTakingOver, directLegacy, nil)
		if got == nil || !got.Present || got.Upstream == nil || got.Upstream.Host != "r.io" {
			t.Errorf("expected fresh Direct derive, got %+v", got)
		}
	})
	t.Run("migrating + legacy absent -> reuse persisted", func(t *testing.T) {
		got := resolveDerived(helpers.PhaseNew, nil, persistedProxy)
		if got != persistedProxy {
			t.Errorf("expected reuse of persisted, got %+v", got)
		}
	})
	t.Run("migrating + Unmanaged legacy + persisted -> keep persisted (no blank)", func(t *testing.T) {
		got := resolveDerived(helpers.PhaseNew, &LegacyConfig{Mode: "Unmanaged"}, persistedProxy)
		if got != persistedProxy {
			t.Errorf("Unmanaged must not blank a prior derive, got %+v", got)
		}
	})
	t.Run("legacy phase but persisted exists -> keep persisted", func(t *testing.T) {
		got := resolveDerived(helpers.PhaseLegacy, directLegacy, persistedProxy)
		if got != persistedProxy {
			t.Errorf("expected persisted retained during Legacy, got %+v", got)
		}
	})
}

func TestResolveStartedAt(t *testing.T) {
	const now = "2026-06-21T12:00:00Z"
	cases := []struct {
		name, phase, prev, want string
	}{
		{"enter TakingOver stamps now", helpers.PhaseTakingOver, "", now},
		{"stay TakingOver preserves", helpers.PhaseTakingOver, "2026-06-21T11:00:00Z", "2026-06-21T11:00:00Z"},
		{"New clears", helpers.PhaseNew, "2026-06-21T11:00:00Z", ""},
		{"Legacy clears", helpers.PhaseLegacy, "2026-06-21T11:00:00Z", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveStartedAt(tc.phase, tc.prev, now); got != tc.want {
				t.Errorf("resolveStartedAt(%q,%q) = %q, want %q", tc.phase, tc.prev, got, tc.want)
			}
		})
	}
}

func TestCleanupReady(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		flippedAt string
		after     time.Duration
		want      bool
	}{
		{"well past window", "2026-06-10T12:00:00Z", 168 * time.Hour, true},
		{"within window", "2026-06-22T00:00:00Z", 168 * time.Hour, false},
		{"empty flippedAt -> not ready", "", 168 * time.Hour, false},
		{"unparseable -> not ready", "nope", 168 * time.Hour, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cleanupReady(tc.flippedAt, now, tc.after); got != tc.want {
				t.Errorf("cleanupReady(%q,%s) = %v, want %v", tc.flippedAt, tc.after, got, tc.want)
			}
		})
	}
}

// resolveFlippedAt is the pure flippedAt rule the hook applies: stamp now on the
// first entry into New, otherwise preserve the prior value.
func TestResolveFlippedAt(t *testing.T) {
	const now = "2026-06-21T00:00:00Z"

	cases := []struct {
		name  string
		phase string
		prev  string
		now   string
		want  string
	}{
		{"enter New stamps now", helpers.PhaseNew, "", now, now},
		{"already flipped is preserved", helpers.PhaseNew, "2026-01-01T00:00:00Z", now, "2026-01-01T00:00:00Z"},
		{"legacy never stamps", helpers.PhaseLegacy, "", now, ""},
		{"takingOver never stamps", helpers.PhaseTakingOver, "", now, ""},
		{"cleanupPending preserves existing", helpers.PhaseCleanupPending, "2026-01-01T00:00:00Z", now, "2026-01-01T00:00:00Z"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveFlippedAt(tc.phase, tc.prev, tc.now); got != tc.want {
				t.Errorf("resolveFlippedAt(%q,%q,%q) = %q, want %q", tc.phase, tc.prev, tc.now, got, tc.want)
			}
		})
	}
}
