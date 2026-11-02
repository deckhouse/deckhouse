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

package ledger

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"

	"controller/apis/deckhouse.io/v1alpha3"
)

// Results of a decision, the values of the result label of the requests metric.
const (
	ResultAllowed          = "allowed"
	ResultExceeded         = "exceeded"
	ResultInconsistent     = "inconsistent"
	ResultUnexpected       = "unexpected"
	ResultDryRun           = "dry_run"
	ResultStale            = "stale"
	ResultLagging          = "lagging"
	ResultUnknownNamespace = "unknown_namespace"
	ResultNoProject        = "no_project"
	ResultNoProjectQuota   = "no_project_quota"
	ResultBusyInflight     = "busy_inflight"
	ResultBusy             = "busy"
	ResultOverload         = "overload"
	ResultNotLeader        = "not_leader"
	ResultFenced           = "fenced"
	ResultSyncing          = "syncing"
	ResultBadRequest       = "bad_request"
)

// APIServerUser is the user the quota plugin of the API server writes the status of a quota as.
const APIServerUser = "system:apiserver"

// inflightShare is the share of a limit the approvals the informer has not confirmed may take over the
// writes of a project; it bounds what a write saved later than the fence window can put over the
// limit.
const inflightShare = 0.1

// Input is the write of the status of a quota the webhook is asked about.
type Input struct {
	Namespace string
	Name      string
	User      string
	DryRun    bool
	// OldUID, OldRV and OldUsed are the uid, the version and the usage of the quota the write replaces.
	OldUID  types.UID
	OldRV   uint64
	OldUsed corev1.ResourceList
	// NewUsed is the usage the write puts.
	NewUsed corev1.ResourceList
}

// Decision is the answer to a write.
type Decision struct {
	Allowed bool
	// Code is the HTTP status of a refusal: 403 for the quota, 503 for a temporary state.
	Code    int32
	Result  string
	Message string
	// Retry says that the state may change within a few milliseconds and the decision may be taken
	// again.
	Retry bool
}

func allow(result string) Decision {
	return Decision{Allowed: true, Result: result}
}

func unavailable(result, cause string, retry bool) Decision {
	return Decision{Code: http.StatusServiceUnavailable, Result: result, Retry: retry,
		Message: fmt.Sprintf("project quota ledger unavailable (%s), retry", cause)}
}

// Decide answers a write of the status of a d8-project-quota. claim is called under the lock of the
// ledger right before an approval is recorded; when it returns false the answer has already been
// given by the deadline, and nothing is recorded.
func (s *State) Decide(in Input, now time.Time, claim func() bool) Decision {
	if in.User != APIServerUser || in.Name != v1alpha3.ProjectQuotaName {
		return Decision{Code: http.StatusForbidden, Result: ResultUnexpected, Message: "unexpected request"}
	}
	if in.DryRun {
		return allow(ResultDryRun)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	q, ok := s.quotas[in.Namespace]
	if !ok {
		return unavailable(ResultUnknownNamespace, "unknown namespace", true)
	}
	// The uid of a quota is known only to who can read it. A write that replaces another object than
	// the one the informer shows is refused before its version is looked at, so a made-up request gets
	// the same answer whatever version it names, and the answers do not lead to the version and the
	// usage the next checks compare. The informer that has not shown a quota made anew yet catches up
	// within the retries.
	if in.OldUID != q.uid {
		return Decision{Code: http.StatusForbidden, Result: ResultInconsistent, Message: "inconsistent request", Retry: true}
	}
	projectName, ok := s.namespaces[in.Namespace]
	if !ok {
		return unavailable(ResultNoProject, "the namespace belongs to no project", true)
	}
	p, ok := s.projects[projectName]
	if !ok {
		return unavailable(ResultNoProjectQuota, "the project has no ProjectQuota", true)
	}
	switch {
	case in.OldRV < q.rv:
		// the write replaces a version the informer has seen replaced: the storage refuses it
		return allow(ResultStale)
	case in.OldRV > q.rv:
		return unavailable(ResultLagging, "the ledger lags behind", true)
	}
	if !equality.Semantic.DeepEqual(nonZero(in.OldUsed), nonZero(q.used)) || aboveOwnHard(in.OldUsed, in.NewUsed, q.hard) {
		return Decision{Code: http.StatusForbidden, Result: ResultInconsistent, Message: "inconsistent request"}
	}

	var keys []corev1.ResourceName
	for key := range p.hard {
		if newValue, oldValue := in.NewUsed[key], in.OldUsed[key]; newValue.Cmp(oldValue) > 0 {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return allow(ResultAllowed)
	}
	slices.Sort(keys)

	var over []corev1.ResourceName
	for _, key := range keys {
		projected := in.NewUsed[key].DeepCopy()
		projected.Add(p.sum[key])
		projected.Sub(q.eff[key])
		if limit := p.hard[key]; projected.Cmp(limit) > 0 {
			over = append(over, key)
		}
	}
	if len(over) > 0 {
		return Decision{Code: http.StatusForbidden, Result: ResultExceeded,
			Message: s.exceededMessageLocked(projectName, p, q, in, over)}
	}

	for _, key := range keys {
		inflight := s.inflightLocked(projectName, key)
		if inflight.Sign() == 0 {
			continue
		}
		increase := in.NewUsed[key].DeepCopy()
		increase.Sub(q.eff[key])
		if increase.Sign() <= 0 {
			continue
		}
		inflight.Add(increase)
		if limit := p.hard[key]; inflight.AsApproximateFloat64() > inflightShare*limit.AsApproximateFloat64() {
			return unavailable(ResultBusyInflight, "too many approvals of the project are not confirmed yet", true)
		}
	}

	if !claim() {
		return unavailable(ResultBusy, "busy", false)
	}
	q.pending = append(q.pending, pendingEntry{rvOld: in.OldRV, used: in.NewUsed.DeepCopy(), at: now})
	s.recomputeLocked(in.Namespace)
	return allow(ResultAllowed)
}

// exceededMessageLocked is the refusal of a write that takes the project over its quota. It keeps the
// "exceeded quota:" form of the refusals of the quota plugin, by which clients tell a quota refusal
// from other errors.
func (s *State) exceededMessageLocked(projectName string, p *projectState, q *namespaceQuota, in Input, over []corev1.ResourceName) string {
	requested := make([]string, 0, len(over))
	used := make([]string, 0, len(over))
	limited := make([]string, 0, len(over))
	for _, key := range over {
		delta := in.NewUsed[key].DeepCopy()
		delta.Sub(in.OldUsed[key])
		current := p.sum[key].DeepCopy()
		current.Sub(q.eff[key])
		current.Add(in.OldUsed[key])
		requested = append(requested, fmt.Sprintf("%s=%s", key, delta.String()))
		used = append(used, fmt.Sprintf("%s=%s", key, current.String()))
		limit := p.hard[key]
		limited = append(limited, fmt.Sprintf("%s=%s", key, limit.String()))
	}
	return fmt.Sprintf("exceeded quota: %s, project %s: requested: %s, used: %s, limited: %s (%d namespaces)",
		v1alpha3.ProjectQuotaName, projectName, strings.Join(requested, ","), strings.Join(used, ","), strings.Join(limited, ","),
		len(s.members[projectName]))
}

// nonZero drops the zero values, so that a key the plugin writes as 0 compares equal to a missing one.
func nonZero(list corev1.ResourceList) corev1.ResourceList {
	out := make(corev1.ResourceList, len(list))
	for key, value := range list {
		if !value.IsZero() {
			out[key] = value
		}
	}
	return out
}

// aboveOwnHard reports a write that raises a key above status.hard of the quota itself as the informer
// shows it, which the quota plugin never writes. The limit of the request is not taken: a made-up
// request would raise it along with the usage. A key the write leaves as it was may stay above the
// limit: the quota plugin checks only the keys a request adds to, and a lowered limit or a recount
// leaves the others above it.
func aboveOwnHard(oldUsed, newUsed, hard corev1.ResourceList) bool {
	for key, value := range newUsed {
		limit, ok := hard[key]
		if !ok || value.Cmp(limit) <= 0 {
			continue
		}
		if previous, had := oldUsed[key]; !had || value.Cmp(previous) > 0 {
			return true
		}
	}
	return false
}
