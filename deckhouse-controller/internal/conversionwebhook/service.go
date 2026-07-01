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

// Package conversionwebhook applies the ConversionWebhook resources a package
// ships in its Helm templates, ahead of the package's own templates. The
// cluster's webhook-handler picks the resources up, patches the target CRDs'
// spec.conversion, and serves conversions — so this must run before the package
// creates custom resources that need converting.
package conversionwebhook

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/flant/kube-client/client"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// tracer names the OpenTelemetry spans this service emits.
	tracer = "conversion-webhook-service"

	// webhookGroup/webhookKind identify the ConversionWebhook resources this
	// service selects from the rendered manifests; webhookResource is their plural
	// for the dynamic client. Everything else the render produces is left to the
	// Helm release.
	webhookGroup    = "deckhouse.io"
	webhookKind     = "ConversionWebhook"
	webhookResource = "conversionwebhooks"

	// fieldManager owns the fields this service server-side applies. nelm adopts
	// the resource into the package's release on the following install/upgrade
	// (ForceAdoption), so this apply never conflicts with Helm ownership.
	fieldManager = "deckhouse-conversion-webhooks"

	// decodeBufferSize is the read-ahead the streaming YAML decoder uses to tell
	// YAML from JSON documents in the rendered manifest stream.
	decodeBufferSize = 4096
)

// renderer renders a package's Helm chart into a YAML manifest stream.
type renderer interface {
	Render(ctx context.Context, namespace string, pkg nelm.Package) (string, error)
}

// Service selects the ConversionWebhook resources from a package's rendered
// chart and applies them to the cluster. It is safe for concurrent use.
type Service struct {
	renderer renderer
	client   *client.Client

	logger *log.Logger
}

// NewService returns a Service that renders packages via renderer and applies
// their ConversionWebhook resources with the given client.
func NewService(renderer renderer, client *client.Client, logger *log.Logger) *Service {
	return &Service{
		renderer: renderer,
		client:   client,
		logger:   logger.Named(tracer),
	}
}

// Install renders the package's chart, selects its ConversionWebhook resources
// and applies them to the cluster. It is a no-op when the package ships no Helm
// chart or declares no ConversionWebhook.
func (s *Service) Install(ctx context.Context, namespace string, pkg nelm.Package) error {
	ctx, span := otel.Tracer(tracer).Start(ctx, "Install")
	defer span.End()

	span.SetAttributes(attribute.String("name", pkg.GetName()))

	manifests, err := s.renderer.Render(ctx, namespace, pkg)
	if err != nil {
		// A package without a Helm chart ships no ConversionWebhook resources.
		if errors.Is(err, nelm.ErrPackageNotHelm) {
			return nil
		}

		return fmt.Errorf("render chart: %w", err)
	}

	webhooks, err := selectWebhooks(manifests)
	if err != nil {
		return fmt.Errorf("select conversion webhooks: %w", err)
	}

	if len(webhooks) == 0 {
		return nil
	}

	s.logger.Debug("apply conversion webhooks",
		slog.String("name", pkg.GetName()),
		slog.Int("webhooks", len(webhooks)))

	for _, webhook := range webhooks {
		if err := s.apply(ctx, webhook); err != nil {
			return fmt.Errorf("apply conversion webhook %q: %w", webhook.GetName(), err)
		}
	}

	return nil
}

// apply server-side applies a single cluster-scoped ConversionWebhook resource.
func (s *Service) apply(ctx context.Context, webhook *unstructured.Unstructured) error {
	gvr := schema.GroupVersionResource{
		Group:    webhookGroup,
		Version:  webhook.GroupVersionKind().Version,
		Resource: webhookResource,
	}

	_, err := s.client.Dynamic().
		Resource(gvr).
		Apply(ctx, webhook.GetName(), webhook, metav1.ApplyOptions{FieldManager: fieldManager, Force: true})
	if err != nil {
		return fmt.Errorf("apply: %w", err)
	}

	return nil
}

// selectWebhooks decodes the rendered manifest stream and returns the
// ConversionWebhook resources in declaration order.
func selectWebhooks(manifests string) ([]*unstructured.Unstructured, error) {
	decoder := utilyaml.NewYAMLOrJSONDecoder(strings.NewReader(manifests), decodeBufferSize)

	var webhooks []*unstructured.Unstructured
	for {
		obj := new(unstructured.Unstructured)
		if err := decoder.Decode(obj); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("decode manifest: %w", err)
		}

		// Skip empty documents (comment-only or blank stretches between "---").
		if len(obj.Object) == 0 {
			continue
		}

		gvk := obj.GroupVersionKind()
		if gvk.Group != webhookGroup || gvk.Kind != webhookKind {
			continue
		}

		webhooks = append(webhooks, obj)
	}

	return webhooks, nil
}
