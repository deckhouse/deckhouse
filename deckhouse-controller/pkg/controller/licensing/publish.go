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
	"log/slog"
	"math"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/licensing"
)

// stateCodes encode the compliance state for Prometheus. The numbers are the
// contract of the licensing alerts, they are not an implementation detail.
var stateCodes = map[string]float64{
	licensing.StateValid:     0,
	licensing.StateWarning:   1,
	licensing.StateGrace:     2,
	licensing.StateViolation: 3,
	// Not computed yet, but the code is reserved here so that the day it is,
	// the alerts do not have to be renumbered.
	string(v1alpha1.LicenseComplianceNoUpdateRight): 4,
}

// publishMetrics exports the policy (specification 12.1). Everything goes into
// one group that is expired first, so a series for a metric or a record that
// left the policy stops being exported instead of freezing at its last value.
func (r *reconciler) publishMetrics(res licensing.Result, owners map[string]string, now time.Time) {
	group := r.metricStorage.Grouped()
	group.ExpireGroupMetrics(metrics.LicensingGroup)

	// An unknown state has no code, and publishing 0 for it would read as Valid
	// on every dashboard and silence every compliance alert.
	if code, ok := stateCodes[res.State]; ok {
		group.GaugeSet(metrics.LicensingGroup, metrics.D8LicenseComplianceState, code, map[string]string{
			// The reason lets an alert exclude a state that is expected, such
			// as a cluster that was simply never registered.
			metrics.LabelReason: res.Reason,
		})
	} else {
		r.logger.Warn("compliance state has no metric code, not published", slog.String("state", res.State))
	}
	group.GaugeSet(metrics.LicensingGroup, metrics.D8LicenseLicensed, boolGauge(res.Licensed), map[string]string{})

	// An unlimited metric is published as +Inf rather than left out: the
	// allocation already covers every node in that case, and a comparison
	// against +Inf is silent in exactly the same way an absent series is.
	for name, limit := range res.Limits.Values {
		value := math.Inf(1)
		if limit != nil {
			value = float64(*limit)
		}
		group.GaugeSet(metrics.LicensingGroup, metrics.D8LicenseLimit, value, map[string]string{
			metrics.LabelResource: name,
		})
	}

	for name, value := range res.Consumption {
		group.GaugeSet(metrics.LicensingGroup, metrics.D8LicenseConsumption, float64(value), map[string]string{
			metrics.LabelResource: name,
		})
	}

	for billing, count := range map[string]int{
		licensing.BillingServer:     len(res.Allocation.Servers),
		licensing.BillingPool:       len(res.Allocation.Pool),
		licensing.BillingUnlicensed: len(res.Allocation.Unlicensed),
	} {
		group.GaugeSet(metrics.LicensingGroup, metrics.D8LicenseNodes, float64(count), map[string]string{
			metrics.LabelBilling: billing,
		})
	}
	group.GaugeSet(metrics.LicensingGroup, metrics.D8LicenseUnlicensedVCPU,
		float64(res.Allocation.UnlicensedVCPU), map[string]string{})

	overLimit := float64(0)
	if res.OverLimitSince != nil {
		overLimit = now.Sub(*res.OverLimitSince).Seconds()
	}
	group.GaugeSet(metrics.LicensingGroup, metrics.D8LicenseOverLimitSeconds, overLimit, map[string]string{})

	// A key that never expires exports no series at all: an expiry alert must
	// not have to know what "never" looks like as a number.
	if res.Key != nil && res.Key.ValidUntil != nil {
		group.GaugeSet(metrics.LicensingGroup, metrics.D8LicenseKeyExpiresInSeconds,
			res.Key.ValidUntil.Sub(now).Seconds(), map[string]string{})
	}

	r.publishRecordMetrics(res, owners, now)
}

func (r *reconciler) publishRecordMetrics(res licensing.Result, owners map[string]string, now time.Time) {
	group := r.metricStorage.Grouped()

	counts := make(map[[2]string]float64, len(res.Records))
	for _, rec := range res.Records {
		status := "accepted"
		if !rec.Accepted {
			status = rec.Reason
		}
		counts[[2]string{rec.Type, status}]++

		labels := map[string]string{
			metrics.LabelRecordID: rec.ID,
			metrics.LabelLicense:  owners[rec.ID],
		}
		active := licensing.Active(rec, now)
		group.GaugeSet(metrics.LicensingGroup, metrics.D8LicenseRecordActive, boolGauge(active), labels)
		group.GaugeSet(metrics.LicensingGroup, metrics.D8LicenseRecordAccepted, boolGauge(rec.Accepted), labels)

		if rec.Platform != nil && rec.Platform.DKP != nil {
			for name, grant := range rec.Platform.DKP.ResourceLimits {
				value := math.Inf(1)
				if grant != nil {
					value = float64(*grant)
				}
				group.GaugeSet(metrics.LicensingGroup, metrics.D8LicenseRecordGrant, value, map[string]string{
					metrics.LabelRecordID: rec.ID,
					metrics.LabelLicense:  owners[rec.ID],
					metrics.LabelResource: name,
				})
			}
		}

		// Only a record that actually carries the policy can expire out of it;
		// an extinguished one is already gone.
		if !active || rec.ExpireAt == nil {
			continue
		}
		group.GaugeSet(metrics.LicensingGroup, metrics.D8LicenseRecordExpiresInSeconds,
			rec.ExpireAt.Sub(now).Seconds(), labels)
	}

	for key, count := range counts {
		group.GaugeSet(metrics.LicensingGroup, metrics.D8LicenseRecords, count, map[string]string{
			metrics.LabelType:   key[0],
			metrics.LabelStatus: key[1],
		})
	}
}

func boolGauge(v bool) float64 {
	if v {
		return 1
	}
	return 0
}
