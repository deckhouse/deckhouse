/*
Copyright 2025 Flant JSC

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

package osversion

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

const nodeGroupLabel = "node.deckhouse.io/group"

var (
	osImageUbuntuRegex = regexp.MustCompile(`^Ubuntu ([0-9.]+)( )?(LTS)?$`)
	osImageDebianRegex = regexp.MustCompile(`^Debian GNU\/Linux ([0-9.]+)( )?(.*)?$`)

	minOSVersionGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "d8_node_manager",
			Name:      "nodes_minimal_os_version",
			Help:      "Minimal OS version among all nodes in a NodeGroup, labeled by OS family.",
		},
		[]string{"os", "version"},
	)
)

func init() {
	dynr.RegisterReconciler(rcname.MetricsOSVersion, &corev1.Node{}, &Reconciler{})

	metrics.Registry.MustRegister(minOSVersionGauge)
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler computes the minimum OS version across all nodes that belong to a NodeGroup
// and exports the result as a Prometheus gauge metric.
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.NewPredicateFuncs(func(obj client.Object) bool {
		_, ok := obj.GetLabels()[nodeGroupLabel]
		return ok
	})}
}

func (r *Reconciler) SetupWatches(_ dynr.Watcher) {}

func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// List all nodes that have the node group label.
	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList, client.HasLabels{nodeGroupLabel}); err != nil {
		return ctrl.Result{}, fmt.Errorf("list nodes: %w", err)
	}

	var minUbuntuVersion, minDebianVersion *semver.Version
	for i := range nodeList.Items {
		osImage := nodeList.Items[i].Status.NodeInfo.OSImage
		if osImage == "" {
			continue
		}

		switch {
		case osImageUbuntuRegex.MatchString(osImage):
			rawVersion := osImageUbuntuRegex.FindStringSubmatch(osImage)[1]
			normalizedVersion := normalizeUbuntuVersionForSemver(rawVersion)
			v, err := semver.Parse(normalizedVersion)
			if err != nil {
				log.Error(err, "failed to parse Ubuntu version", "osImage", osImage, "version", normalizedVersion)
				continue
			}
			if minUbuntuVersion == nil || v.LT(*minUbuntuVersion) {
				minUbuntuVersion = &v
			}

		case osImageDebianRegex.MatchString(osImage):
			rawVersion := osImageDebianRegex.FindStringSubmatch(osImage)[1]
			v, err := semver.Parse(rawVersion)
			if err != nil {
				log.Error(err, "failed to parse Debian version", "osImage", osImage, "version", rawVersion)
				continue
			}
			if minDebianVersion == nil || v.LT(*minDebianVersion) {
				minDebianVersion = &v
			}
		}
	}

	// Reset all previously set values so stale series are removed.
	minOSVersionGauge.Reset()

	if minUbuntuVersion != nil {
		minOSVersionGauge.With(prometheus.Labels{
			"os":      "ubuntu",
			"version": minUbuntuVersion.String(),
		}).Set(1)
	}
	if minDebianVersion != nil {
		minOSVersionGauge.With(prometheus.Labels{
			"os":      "debian",
			"version": minDebianVersion.String(),
		}).Set(1)
	}

	log.V(1).Info("updated minimal OS version metrics",
		"minUbuntu", versionOrNone(minUbuntuVersion),
		"minDebian", versionOrNone(minDebianVersion),
	)

	return ctrl.Result{}, nil
}

// normalizeUbuntuVersionForSemver converts Ubuntu version format to semver format:
// 20.04.3 -> 20.4.3, 20.04 -> 20.4.0
func normalizeUbuntuVersionForSemver(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return version
	}

	major := strings.TrimLeft(parts[0], "0")
	if major == "" {
		major = "0"
	}

	minor := strings.TrimLeft(parts[1], "0")
	if minor == "" {
		minor = "0"
	}

	patch := "0"
	if len(parts) > 2 {
		patch = strings.TrimLeft(parts[2], "0")
		if patch == "" {
			patch = "0"
		}
	}

	return major + "." + minor + "." + patch
}

func versionOrNone(v *semver.Version) string {
	if v == nil {
		return "<none>"
	}
	return v.String()
}
