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

import "time"

// AgentDSStatus is the relevant status of the registry-agent DaemonSet.
type AgentDSStatus struct {
	NumberReady            int
	DesiredNumberScheduled int
}

// CacheSTSStatus is the relevant status of the registry-cache StatefulSet.
type CacheSTSStatus struct {
	ReadyReplicas int
	Replicas      int
}

// CacheLeaseStatus is the relevant content of the registry-cache-leader Lease.
type CacheLeaseStatus struct {
	Holder               string
	RenewTime            time.Time
	LeaseDurationSeconds int
}

// isAirgap reports whether the derived config represents an air-gap Local
// migration (legacy Local has no upstream proxy — Upstream is nil).
func isAirgap(derived *DerivedConfig) bool {
	return derived != nil && derived.Present && derived.Upstream == nil
}

// computeVerifyReady reports whether the new stack is proven ready to take over.
// Agent readiness is always required; cache readiness is required only when a
// cache is expected (a Direct-mode takeover has no cache StatefulSet).
// For air-gap Local migrations, storeSynced must also be true before New is
// allowed — the cache must be pre-populated from the legacy store first.
func computeVerifyReady(ds *AgentDSStatus, sts *CacheSTSStatus, lease *CacheLeaseStatus, cacheExpected, airgap, storeSynced bool, now time.Time) bool {
	if ds == nil || ds.DesiredNumberScheduled == 0 || ds.NumberReady != ds.DesiredNumberScheduled {
		return false
	}
	if !cacheExpected {
		return true
	}
	if sts == nil || sts.Replicas == 0 || sts.ReadyReplicas != sts.Replicas {
		return false
	}
	if lease == nil || lease.Holder == "" {
		return false
	}
	dur := lease.LeaseDurationSeconds
	if dur <= 0 {
		dur = 15
	}
	// A live leader renews well within the lease duration; allow 2x as slack.
	if now.Sub(lease.RenewTime) > time.Duration(2*dur)*time.Second {
		return false
	}
	if airgap && !storeSynced {
		return false
	}
	return true
}

// cacheExpected reports whether the desired config runs the on-master cache:
// the takeover-derived cache flag when migrating, else the operator's cache.
func cacheExpected(derived *DerivedConfig, operatorCacheEnabled bool) bool {
	if derived != nil && derived.Present {
		return derived.Cache.Enabled
	}
	return operatorCacheEnabled
}
