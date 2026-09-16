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
	packages []*licensing.Package,
	res licensing.Result,
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
	}

	owners := make(map[string]string, len(res.Records))
	for i := range items {
		item := &items[i]
		records := byKey[item.Name]
		for _, rec := range records {
			owners[rec.ID] = item.Name
		}

		status := keyStatus(failures[i], packages[i], records)
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

func keyStatus(failure error, pkg *licensing.Package, records []licensing.RecordStatus) v1alpha1.ClusterLicenseStatus {
	if failure != nil {
		return v1alpha1.ClusterLicenseStatus{Accepted: false, Message: failure.Error()}
	}

	status := v1alpha1.ClusterLicenseStatus{
		PackageJti:   pkg.JTI,
		CustomerName: pkg.CustomerName,
		Records:      make([]v1alpha1.LicenseRecordStatus, 0, len(records)),
	}

	accepted := 0
	for _, rec := range records {
		if rec.Accepted {
			accepted++
		}
		status.Records = append(status.Records, recordStatus(rec))
	}
	status.Accepted = accepted > 0
	status.Message = fmt.Sprintf("%d of %d records are part of the policy", accepted, len(records))

	return status
}

func recordStatus(rec licensing.RecordStatus) v1alpha1.LicenseRecordStatus {
	out := v1alpha1.LicenseRecordStatus{
		ID:        rec.ID,
		Type:      rec.Type,
		Origin:    rec.Origin,
		Accepted:  rec.Accepted,
		Reason:    v1alpha1.LicenseRecordReason(rec.Reason),
		Message:   rec.Message,
		RenewedBy: rec.RenewedBy,
		StartAt:   ptr.To(metav1.NewTime(rec.StartAt)),
		GraceDays: rec.GraceDays,
	}
	if rec.ExpireAt != nil {
		out.ExpireAt = ptr.To(metav1.NewTime(*rec.ExpireAt))
	}
	if rec.Workload != nil && rec.Workload.DKP != nil && rec.Workload.DKP.ResourceLimits != nil {
		out.Grants = rec.Workload.DKP.ResourceLimits
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
	if err := r.Create(ctx, effective); err != nil {
		return nil, fmt.Errorf("create effective license: %w", err)
	}
	return effective, nil
}

func (r *reconciler) updateEffectiveLicense(
	ctx context.Context,
	effective *v1alpha1.EffectiveLicense,
	res licensing.Result,
	values map[string]licensing.MetricValue,
	request string,
	now time.Time,
) error {
	status := effectiveStatus(effective.Status, res, values, request, now)
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
	values map[string]licensing.MetricValue,
	request string,
	now time.Time,
) v1alpha1.EffectiveLicenseStatus {
	status := v1alpha1.EffectiveLicenseStatus{
		Compliance: v1alpha1.LicenseCompliance{
			State:  v1alpha1.LicenseComplianceState(res.State),
			Reason: res.Reason,
			// A state that did not change did not start again.
			Since: ptr.To(metav1.NewTime(now)),
		},
		Counts: v1alpha1.LicenseCounts{
			Packages: res.Counts.Packages,
			Records:  res.Counts.Records,
			Accepted: res.Counts.Accepted,
			Rejected: res.Counts.Rejected,
		},
		Effective: v1alpha1.LicenseEffective{
			Limits:    res.Effective,
			Unlimited: res.Unlimited,
			GrantedBy: res.GrantedBy,
		},
		Metrics:             make(map[string]v1alpha1.LicenseMetricValue, len(values)),
		WithinLimits:        res.WithinLimits,
		NextReduction:       reductionStatus(res.NextReduction, now),
		Timeline:            timelineStatus(res.Timeline),
		RegistrationRequest: request,
		Conditions:          append([]metav1.Condition(nil), prev.Conditions...),
	}

	if prev.Compliance.Since != nil &&
		prev.Compliance.State == status.Compliance.State &&
		prev.Compliance.Reason == status.Compliance.Reason {
		status.Compliance.Since = prev.Compliance.Since
	}

	for name, value := range values {
		status.Metrics[name] = v1alpha1.LicenseMetricValue{
			Instant:      value.Instant,
			Avg7d:        value.Avg7d,
			Extrapolated: value.Extrapolated,
		}
	}

	setConditions(&status.Conditions, res, now)

	return status
}

func timelineStatus(segments []licensing.Segment) []v1alpha1.LicenseTimelineSegment {
	if len(segments) == 0 {
		return nil
	}
	out := make([]v1alpha1.LicenseTimelineSegment, 0, len(segments))
	for _, seg := range segments {
		item := v1alpha1.LicenseTimelineSegment{
			From:   metav1.NewTime(seg.From),
			State:  v1alpha1.LicenseComplianceState(seg.State),
			Limits: seg.Limits,
		}
		if seg.To != nil {
			item.To = ptr.To(metav1.NewTime(*seg.To))
		}
		out = append(out, item)
	}
	return out
}

func reductionStatus(reduction *licensing.Reduction, now time.Time) *v1alpha1.LicenseReduction {
	if reduction == nil {
		return nil
	}

	names := make(map[string]bool, len(reduction.From)+len(reduction.To))
	for name := range reduction.From {
		names[name] = true
	}
	for name := range reduction.To {
		names[name] = true
	}

	limits := make(map[string]v1alpha1.LicenseLimitChange, len(names))
	for name := range names {
		from, hadFrom := reduction.From[name]
		to, hadTo := reduction.To[name]
		if hadFrom && hadTo && samePtr(from, to) {
			continue
		}
		limits[name] = v1alpha1.LicenseLimitChange{From: from, To: to}
	}

	return &v1alpha1.LicenseReduction{
		At:     metav1.NewTime(reduction.At),
		In:     humanDuration(reduction.At.Sub(now)),
		Limits: limits,
	}
}

func samePtr(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// humanDuration renders a coarse "time left", the way a person reads a licence:
// days while there are days, then hours, then minutes.
func humanDuration(d time.Duration) string {
	switch {
	case d <= 0:
		return "0m"
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd", int64(d/(24*time.Hour)))
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int64(d/time.Hour))
	default:
		return fmt.Sprintf("%dm", int64(d/time.Minute))
	}
}
