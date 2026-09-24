// Copyright 2026 Flant JSC
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

package licensing

import (
	"context"
	"fmt"
	"time"

	equality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/licensing"
)

// updateKeyStatuses writes the per-key verdict back and returns, for every
// record id, the key that carries it.
func (r *reconciler) updateKeyStatuses(
	ctx context.Context,
	items []v1alpha1.ClusterLicense,
	keys []licensing.KeyRecords,
	failures []error,
	res licensing.Result,
	now time.Time,
) (map[string]string, error) {
	// The verdicts are matched back to their key by record id rather than by
	// position, so that a change in how Compute orders its output cannot quietly
	// attribute one key's records to another. The same id may legitimately
	// appear in two keys, one of them marked Duplicate, so the verdicts of an id
	// are consumed in order.
	verdicts := make(map[string][]licensing.RecordStatus, len(res.Records))
	for _, rec := range res.Records {
		verdicts[rec.ID] = append(verdicts[rec.ID], rec)
	}

	byKey := make(map[string][]licensing.RecordStatus, len(keys))
	jtis := make(map[string]string, len(keys))
	customers := make(map[string]string, len(keys))
	for _, key := range keys {
		records := make([]licensing.RecordStatus, 0, len(key.Records))
		for _, rec := range key.Records {
			pending := verdicts[rec.ID]
			if len(pending) == 0 {
				return nil, fmt.Errorf("record %s of cluster license %s has no verdict", rec.ID, key.Key)
			}
			records = append(records, pending[0])
			verdicts[rec.ID] = pending[1:]
		}
		byKey[key.Key] = records
		jtis[key.Key] = key.JTI
		customers[key.Key] = key.CustomerName
	}

	owners := make(map[string]string, len(res.Records))
	for i := range items {
		item := &items[i]
		records := byKey[item.Name]
		for _, rec := range records {
			owners[rec.ID] = item.Name
		}

		status := keyStatus(item.Status, failures[i], jtis[item.Name], customers[item.Name], records, res.Superseded[item.Name], now)
		if equality.Semantic.DeepEqual(item.Status, status) {
			continue
		}
		item.Status = status
		if err := r.Status().Update(ctx, item); err != nil {
			return nil, fmt.Errorf("update status of cluster license %s: %w", item.Name, err)
		}
	}

	return owners, nil
}

func keyStatus(
	prev v1alpha1.ClusterLicenseStatus,
	failure error,
	jti string,
	customer string,
	records []licensing.RecordStatus,
	superseded bool,
	now time.Time,
) v1alpha1.ClusterLicenseStatus {
	status := v1alpha1.ClusterLicenseStatus{
		Superseded: superseded,
		Conditions: append([]metav1.Condition(nil), prev.Conditions...),
	}

	if failure != nil {
		status.Message = failure.Error()
		setValid(&status, false, status.Message, now)
		return status
	}

	status.PackageJti = jti
	status.CustomerName = customer
	status.Records = make([]v1alpha1.LicenseRecordStatus, 0, len(records))

	accepted := 0
	for _, rec := range records {
		if rec.Accepted {
			accepted++
		}
		status.Records = append(status.Records, recordStatus(rec))
	}
	status.Accepted = accepted > 0
	status.Message = fmt.Sprintf("%d of %d records are part of the policy", accepted, len(records))
	setValid(&status, true, status.Message, now)

	return status
}

// setValid publishes the one condition a single key carries: whether the token
// itself verified. Everything else about a key is in its records.
func setValid(status *v1alpha1.ClusterLicenseStatus, valid bool, message string, now time.Time) {
	reason := "Rejected"
	if valid {
		reason = "Verified"
	}
	set(&status.Conditions, now, conditionValid, valid, reason, message, message)
}

func recordStatus(rec licensing.RecordStatus) v1alpha1.LicenseRecordStatus {
	out := v1alpha1.LicenseRecordStatus{
		ID:           rec.ID,
		Type:         rec.Type,
		Origin:       rec.Origin,
		Accepted:     rec.Accepted,
		Reason:       v1alpha1.LicenseRecordReason(rec.Reason),
		Message:      rec.Message,
		SupersededBy: rec.RenewedBy,
		StartAt:      ptr.To(metav1.NewTime(rec.StartAt)),
		GraceDays:    rec.GraceDays,
	}
	if rec.ExpireAt != nil {
		out.ExpireAt = ptr.To(metav1.NewTime(*rec.ExpireAt))
	}
	if rec.Platform != nil && rec.Platform.ResourceLimits != nil {
		out.Grants = rec.Platform.ResourceLimits
	}
	return out
}

// getEffectiveLicense returns the singleton, creating the empty shell on first
// reconcile. The read bypasses the cache so that the object created a moment ago
// is not created twice.
func (r *reconciler) getEffectiveLicense(ctx context.Context) (*v1alpha1.EffectiveLicense, error) {
	key := types.NamespacedName{Name: v1alpha1.EffectiveLicenseName}

	effective := new(v1alpha1.EffectiveLicense)
	err := r.apiReader.Get(ctx, key, effective)
	if err == nil {
		return effective, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("get effective license: %w", err)
	}

	effective = &v1alpha1.EffectiveLicense{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name, Labels: objectLabels},
	}
	if err := r.Create(ctx, effective); err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("create effective license: %w", err)
	}
	if err := r.apiReader.Get(ctx, key, effective); err != nil {
		return nil, fmt.Errorf("get effective license: %w", err)
	}
	return effective, nil
}

func (r *reconciler) updateEffectiveLicense(
	ctx context.Context,
	effective *v1alpha1.EffectiveLicense,
	res licensing.Result,
	observed []nodeObservation,
	request string,
	now time.Time,
) error {
	status := effectiveStatus(effective.Status, res, observed, request, now)
	if equality.Semantic.DeepEqual(effective.Status, status) {
		return nil
	}

	effective.Status = status
	if err := r.Status().Update(ctx, effective); err != nil {
		return fmt.Errorf("update effective license status: %w", err)
	}
	return nil
}

func effectiveStatus(
	prev v1alpha1.EffectiveLicenseStatus,
	res licensing.Result,
	observed []nodeObservation,
	request string,
	now time.Time,
) v1alpha1.EffectiveLicenseStatus {
	alloc := res.Allocation
	status := v1alpha1.EffectiveLicenseStatus{
		Licensed: res.Licensed,
		Compliance: v1alpha1.LicenseCompliance{
			State:  v1alpha1.LicenseComplianceState(res.State),
			Reason: res.Reason,
			// A state that did not change did not start again.
			Since: ptr.To(metav1.NewTime(now)),
		},
		Key: keyInfo(res.Key),
		Limits: v1alpha1.LicenseLimits{
			Values:    res.Limits.Values,
			Unlimited: res.Unlimited,
			GrantedBy: res.GrantedBy,
		},
		Consumption: v1alpha1.LicenseConsumption{
			Servers:   res.Consumption[licensing.MetricServers],
			VCPU:      res.Consumption[licensing.MetricVCPU],
			Cores:     res.Consumption[licensing.MetricCores],
			FreeNodes: freeNodes(observed),
		},
		Allocation: v1alpha1.LicenseAllocation{
			Servers: metricAllocation(alloc.Servers),
			VCPU:    metricAllocation(alloc.VCPU),
			Cores:   metricAllocation(alloc.Cores),
			Unlicensed: v1alpha1.LicenseUnlicensedNodes{
				Nodes: int64(len(alloc.Unlicensed)),
				VCPU:  alloc.UnlicensedVCPU,
			},
		},
		Nodes:               nodeStatuses(res, observed),
		RegistrationRequest: request,
		Conditions:          append([]metav1.Condition(nil), prev.Conditions...),
	}
	if res.OverLimitSince != nil {
		status.OverLimitSince = ptr.To(metav1.NewTime(*res.OverLimitSince))
	}

	if prev.Compliance.Since != nil &&
		prev.Compliance.State == status.Compliance.State &&
		prev.Compliance.Reason == status.Compliance.Reason {
		status.Compliance.Since = prev.Compliance.Since
	}

	setConditions(&status.Conditions, res, now)

	return status
}

// metricAllocation publishes one metric. An unlimited metric carries the flag
// instead of a limit, so that "no limit" never has to be spelled as a number.
func metricAllocation(m licensing.MetricAllocation) v1alpha1.LicenseMetricAllocation {
	return v1alpha1.LicenseMetricAllocation{
		Limit:     m.Limit,
		Unlimited: m.Limit == nil,
		Used:      m.Used,
	}
}

func keyInfo(key *licensing.KeyInfo) *v1alpha1.LicenseKeyInfo {
	if key == nil {
		return nil
	}
	out := &v1alpha1.LicenseKeyInfo{
		Name:         key.Name,
		Jti:          key.JTI,
		CustomerName: key.CustomerName,
		Origin:       key.Origin,
		GraceDays:    key.GraceDays,
	}
	if key.ValidUntil != nil {
		out.ValidUntil = ptr.To(metav1.NewTime(*key.ValidUntil))
	}
	return out
}

// nodeStatuses publishes the full allocation: the licensable nodes in allocation
// order, then the free ones. The Console renders this list as is, it computes
// nothing itself, so a cluster and its Console can never disagree.
func nodeStatuses(res licensing.Result, observed []nodeObservation) []v1alpha1.LicenseNodeStatus {
	if len(observed) == 0 {
		return nil
	}

	sizes := make(map[string]int64, len(observed))
	free := make(map[string]string, len(observed))
	for _, node := range observed {
		sizes[node.Name] = node.VCPU
		if node.Free {
			free[node.Name] = node.Reason
		}
	}

	out := make([]v1alpha1.LicenseNodeStatus, 0, len(observed))
	groups := []struct {
		names   []string
		billing v1alpha1.LicenseNodeBilling
	}{
		{res.Allocation.Servers.Nodes, v1alpha1.LicenseNodeServer},
		{res.Allocation.VCPU.Nodes, v1alpha1.LicenseNodeVCPU},
		{res.Allocation.Cores.Nodes, v1alpha1.LicenseNodeCores},
		{res.Allocation.CoresVCPU, v1alpha1.LicenseNodeCoresVCPU},
		{res.Allocation.Unlicensed, v1alpha1.LicenseNodeUnlicensed},
	}
	for _, group := range groups {
		for _, name := range group.names {
			out = append(out, v1alpha1.LicenseNodeStatus{Name: name, VCPU: sizes[name], Billing: group.billing})
		}
	}
	// observed is sorted by name, so the free group is too.
	for _, node := range observed {
		if reason, isFree := free[node.Name]; isFree {
			out = append(out, v1alpha1.LicenseNodeStatus{
				Name: node.Name, VCPU: node.VCPU, Billing: v1alpha1.LicenseNodeFree, Reason: reason,
			})
		}
	}
	return out
}

// previousServers reads the previous allocation off the published status. It is
// the only state the tie-breaking rule of specification 7.4 needs.
func previousServers(status v1alpha1.EffectiveLicenseStatus) map[string]bool {
	out := make(map[string]bool, len(status.Nodes))
	for _, node := range status.Nodes {
		if node.Billing == v1alpha1.LicenseNodeServer {
			out[node.Name] = true
		}
	}
	return out
}

func previousOverLimitSince(status v1alpha1.EffectiveLicenseStatus) *time.Time {
	if status.OverLimitSince == nil {
		return nil
	}
	at := status.OverLimitSince.Time
	return &at
}
