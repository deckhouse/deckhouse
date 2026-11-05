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

package nelm

import (
	"context"
	"slices"

	"github.com/werf/nelm/v2/pkg/resource/spec"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/resourcesize"
)

// autoscaling is how the resource requests of a package meet the autoscalers of its release.
type autoscaling struct {
	// fallbacks are the targets whose range no autoscaler applies, so it falls back to fixed values.
	fallbacks []resourcesize.Target
	// pins are the targets fixed by value although an autoscaler scales them, which pins it.
	pins []resourcesize.Target
}

// renderWithFallbacks renders the chart with opts, and once more with the fallback patches when a
// range of the package's resource requests has no autoscaler in the first render: only the render
// tells which autoscalers the chart has. It returns the manifests, the patches they were rendered
// with, which the install must reuse to apply the same manifests, and how the requests meet the
// autoscalers.
func (s *Service) renderWithFallbacks(ctx context.Context, namespace string, pkg Package, opts nelm.InstallOptions) (string, []spec.Patch, autoscaling, error) {
	rendered, err := s.client.Render(ctx, namespace, pkg.GetName(), opts)
	if err != nil {
		return "", nil, autoscaling{}, err
	}

	report, fallbackPatches, err := meetAutoscalers(resourceRequests(pkg), rendered)
	if err != nil {
		return "", nil, autoscaling{}, err
	}

	if len(fallbackPatches) == 0 {
		return rendered, opts.RenderPatches, report, nil
	}

	opts.RenderPatches = append(slices.Clone(opts.RenderPatches), fallbackPatches...)

	rendered, err = s.client.Render(ctx, namespace, pkg.GetName(), opts)
	if err != nil {
		return "", nil, autoscaling{}, err
	}

	return rendered, opts.RenderPatches, report, nil
}

// meetAutoscalers reads the autoscalers of the rendered release and returns how the requests meet
// them, with the patches that fall the ranges no autoscaler applies back to fixed values. Without
// requests the manifests are left unread.
func meetAutoscalers(requests []resourcesize.Request, renderedManifests string) (autoscaling, []spec.Patch, error) {
	if len(requests) == 0 {
		return autoscaling{}, nil, nil
	}

	autoscaled, err := readAutoscaled(renderedManifests)
	if err != nil {
		return autoscaling{}, nil, err
	}

	fallbacks, patches, err := resourcesize.FallbackPatches(requests, autoscaled)
	if err != nil {
		return autoscaling{}, nil, err
	}

	return autoscaling{fallbacks: fallbacks, pins: resourcesize.Pins(requests, autoscaled)}, patches, nil
}

// publishAutoscaling replaces the autoscaler gauges of an application: one per target that falls
// back for a missing autoscaler, and one per target that pins an autoscaler. A module reports none.
func (s *Service) publishAutoscaling(namespace string, pkg Package, report autoscaling) {
	app, ok := pkg.(application)
	if !ok || s.metrics == nil {
		return
	}

	gauges := []struct {
		metric  string
		targets []resourcesize.Target
	}{
		{metrics.ApplicationAutoscalerMissing, report.fallbacks},
		{metrics.ApplicationAutoscalerPinned, report.pins},
	}

	for _, gauge := range gauges {
		// Expire only this gauge in the package's group, not every collector's.
		s.metrics.Grouped().ExpireGroupMetricByName(pkg.GetName(), gauge.metric)

		for _, target := range gauge.targets {
			s.metrics.Grouped().GaugeSet(pkg.GetName(), gauge.metric, 1, map[string]string{
				metrics.LabelNamespace:  namespace,
				metrics.LabelName:       app.GetInstance(),
				metrics.LabelKind:       target.Kind,
				metrics.LabelWorkload:   target.Name,
				metrics.LabelContainer:  target.Container,
				metrics.LabelResource:   target.Resource,
				metrics.LabelAutoscaler: target.Autoscaler(),
			})
		}
	}
}

// expireAutoscaling drops the autoscaler gauges of a package whose release is gone.
func (s *Service) expireAutoscaling(name string) {
	if s.metrics == nil {
		return
	}

	s.metrics.Grouped().ExpireGroupMetricByName(name, metrics.ApplicationAutoscalerMissing)
	s.metrics.Grouped().ExpireGroupMetricByName(name, metrics.ApplicationAutoscalerPinned)
}
