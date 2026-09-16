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
	"log/slog"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/licensing"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// systemTaintKeys mark a node as reserved for platform components. A node
// carrying one of them is presumed not to run user workload.
var systemTaintKeys = []string{
	"dedicated.deckhouse.io",
	"node-role.kubernetes.io/control-plane",
	"node-role.kubernetes.io/master",
}

// systemNamespacePrefixes are the namespaces whose pods are platform, not user,
// workload.
var systemNamespacePrefixes = []string{"d8-", "kube-"}

// countNodes applies the membership rule of the consumption metrics. Both
// metrics are derived from the same node set, so they cannot drift apart.
//
// A node counts unless it is reserved by a system taint, and a reserved node
// counts anyway once it actually runs user workload: a control plane opened up
// for user pods is cluster capacity like any other node. NotReady nodes count,
// the state is temporary and it is the capacity that is licensed.
//
// A node whose pods cannot be listed aborts the whole observation: a sample that
// silently leaves such a node out would enter the seven day average as a real
// drop in consumption, and no later reconcile would ever correct it.
func countNodes(nodes []corev1.Node, hasUserPods func(nodeName string) (bool, error)) (vcpu, count int64, err error) {
	for _, node := range nodes {
		if isSystemTainted(node) {
			used, err := hasUserPods(node.Name)
			if err != nil {
				return 0, 0, err
			}
			if !used {
				continue
			}
		}
		count++
		vcpu += wholeCores(node)
	}
	return vcpu, count, nil
}

func isSystemTainted(node corev1.Node) bool {
	for _, taint := range node.Spec.Taints {
		for _, key := range systemTaintKeys {
			if taint.Key == key {
				return true
			}
		}
	}
	return false
}

// wholeCores rounds the capacity up: a node advertising 3500m is licensed as
// four cores, never as three.
func wholeCores(node corev1.Node) int64 {
	milli := node.Status.Capacity.Cpu().MilliValue()
	if milli <= 0 {
		return 0
	}
	return (milli + 999) / 1000
}

// sampleConsumption takes one observation of the consumption metrics.
func (r *reconciler) sampleConsumption(ctx context.Context) (map[string]float64, error) {
	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes); err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	// ponytail: one pod list per reserved node, once an hour. That is a handful
	// of calls on any real cluster; if a fleet ever shows up with hundreds of
	// system nodes, replace it with a single cluster wide pod list.
	vcpu, count, err := countNodes(nodes.Items, func(name string) (bool, error) {
		return r.hasUserPods(ctx, name)
	})
	if err != nil {
		return nil, err
	}

	return map[string]float64{
		metricVCPU:  float64(vcpu),
		metricNodes: float64(count),
	}, nil
}

// hasUserPods reports whether a reserved node runs at least one pod that is
// neither platform workload nor a per-node agent. The list goes through the
// uncached reader on purpose: the manager caches only a narrow slice of pods,
// and this question is asked once an hour.
func (r *reconciler) hasUserPods(ctx context.Context, nodeName string) (bool, error) {
	var pods corev1.PodList
	err := r.apiReader.List(ctx, &pods, client.MatchingFields{"spec.nodeName": nodeName})
	if err != nil {
		// No sample at all beats a wrong one: the reconcile fails, the
		// controller backs off and the observation is taken when the API
		// answers again.
		r.logger.Warn("list pods of a reserved node", slog.String("node", nodeName), log.Err(err))
		return false, fmt.Errorf("list pods of node %s: %w", nodeName, err)
	}

	for _, pod := range pods.Items {
		if isUserPod(pod) {
			return true, nil
		}
	}
	return false, nil
}

func isUserPod(pod corev1.Pod) bool {
	// A finished Job leaves its pod object behind for hours. It is not running
	// workload, so it must not keep a reserved node inside the metrics.
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return false
	}
	for _, prefix := range systemNamespacePrefixes {
		if strings.HasPrefix(pod.Namespace, prefix) {
			return false
		}
	}
	// A DaemonSet lands on every node it tolerates, so its presence says nothing
	// about the node being opened up for user workload.
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "DaemonSet" {
			return false
		}
	}
	return true
}

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

// publishMetrics exports the policy. Everything goes into one group that is
// expired first, so a series for a resource or a record that left the policy
// stops being exported instead of freezing at its last value.
func (r *reconciler) publishMetrics(
	res licensing.Result,
	values map[string]licensing.MetricValue,
	owners map[string]string,
	now time.Time,
) {
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
	group.GaugeSet(metrics.LicensingGroup, metrics.D8LicenseWithinLimits, boolGauge(res.WithinLimits), map[string]string{})

	for name, limit := range res.Effective {
		// An unlimited resource exports no series at all: the alerts compare
		// consumption against the limit, and a missing limit must not compare.
		if limit == nil {
			continue
		}
		group.GaugeSet(metrics.LicensingGroup, metrics.D8LicenseEffectiveLimit, float64(*limit), map[string]string{
			metrics.LabelResource: name,
		})
	}

	for name, value := range values {
		for kind, v := range map[string]float64{
			"instant":      value.Instant,
			"avg_7d":       value.Avg7d,
			"extrapolated": value.Extrapolated,
		} {
			group.GaugeSet(metrics.LicensingGroup, metrics.D8LicenseConsumption, v, map[string]string{
				metrics.LabelResource: name,
				metrics.LabelKind:     kind,
			})
		}
	}

	if res.NextReduction != nil {
		group.GaugeSet(metrics.LicensingGroup, metrics.D8LicenseNextReductionSeconds,
			res.NextReduction.At.Sub(now).Seconds(), map[string]string{})
		for name, limit := range res.NextReduction.To {
			if limit == nil {
				continue
			}
			group.GaugeSet(metrics.LicensingGroup, metrics.D8LicenseLimitAfterReduction, float64(*limit), map[string]string{
				metrics.LabelResource: name,
			})
		}
	}

	counts := make(map[[2]string]float64, len(res.Records))
	for _, rec := range res.Records {
		status := "accepted"
		if !rec.Accepted {
			status = rec.Reason
		}
		counts[[2]string{rec.Type, status}]++

		// Only records that are actually carrying the policy can expire out of
		// it; a renewed or superseded one is already gone.
		if !licensing.Active(rec, now) || rec.ExpireAt == nil {
			continue
		}
		group.GaugeSet(metrics.LicensingGroup, metrics.D8LicenseRecordExpiresInSeconds,
			rec.ExpireAt.Sub(now).Seconds(), map[string]string{
				metrics.LabelRecordID: rec.ID,
				metrics.LabelLicense:  owners[rec.ID],
			})
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
