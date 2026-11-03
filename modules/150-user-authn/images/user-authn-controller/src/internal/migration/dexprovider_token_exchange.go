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

package migration

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

// DexProviderTokenExchangeMarker is the ConfigMap in the Dex namespace that says the migration is done. The module
// reads it into internal.dexProviderTokenExchangeMigrated: until it exists, the Dex config lets a DexProvider without
// spec.tokenExchange exchange tokens, as before the field existed; after it, such a provider does not.
const DexProviderTokenExchangeMarker = "dexprovider-token-exchange-migration"

// exchangingTypes are the DexProvider types whose connectors exchange tokens in Dex.
var exchangingTypes = []string{"OIDC", "Gitlab"}

// DexProviderTokenExchange sets spec.tokenExchange.enabled once on the OIDC and Gitlab DexProviders that existed
// before the field: true while some DexClient with the allow-access-to-kubernetes annotation can exchange a token
// through the provider for one the Kubernetes API accepts, false otherwise. Then it creates the marker.
//
// A provider that already has the field is left as it is: the administrator set it, or a previous pass did. Once the
// marker exists, the migration does nothing, so a provider created later without the field stays without exchange,
// and one an administrator turned off is not turned on again.
type DexProviderTokenExchange struct {
	client   client.Client
	reader   client.Reader
	log      logr.Logger
	interval time.Duration
}

// NewDexProviderTokenExchange returns the migration. It reads through reader (the API server): it runs once, and a
// cache would start informers it no longer needs. interval 0 means the default retry interval.
func NewDexProviderTokenExchange(c client.Client, reader client.Reader, log logr.Logger, interval time.Duration) *DexProviderTokenExchange {
	if interval <= 0 {
		interval = defaultRetryInterval
	}
	return &DexProviderTokenExchange{client: c, reader: reader, log: log, interval: interval}
}

// NeedLeaderElection makes the migration run on the leader only.
func (m *DexProviderTokenExchange) NeedLeaderElection() bool {
	return true
}

// Start runs the migration until it is done or ctx ends, trying again every interval after a failure.
func (m *DexProviderTokenExchange) Start(ctx context.Context) error {
	for {
		err := m.Run(ctx)
		if err == nil {
			return nil
		}
		if ctx.Err() == nil {
			m.log.Error(err, "Failed to set token exchange on the DexProviders")
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(m.interval):
		}
	}
}

// Run makes one pass, unless the marker says the migration is done.
func (m *DexProviderTokenExchange) Run(ctx context.Context) error {
	marker := &corev1.ConfigMap{}
	err := m.reader.Get(ctx, client.ObjectKey{Namespace: naming.DexNamespace, Name: DexProviderTokenExchangeMarker}, marker)
	switch {
	case err == nil:
		return nil
	case !apierrors.IsNotFound(err):
		return fmt.Errorf("get the marker: %w", err)
	}

	exchanging, err := m.clientsAllowedToKubernetes(ctx)
	if err != nil {
		return err
	}

	providers := controller.List(controller.DexProviderGVK)
	if err := m.reader.List(ctx, providers); err != nil {
		return fmt.Errorf("list DexProviders: %w", err)
	}
	var enabled, disabled []string
	for i := range providers.Items {
		provider := &providers.Items[i]
		providerType, _, _ := unstructured.NestedString(provider.Object, "spec", "type")
		if !slices.Contains(exchangingTypes, providerType) {
			continue
		}
		if _, set, _ := unstructured.NestedFieldNoCopy(provider.Object, "spec", "tokenExchange", "enabled"); set {
			continue
		}
		// Every DexClient may use every connector: DexClient has no allowedConnectors, so an annotated one makes
		// every exchanging provider a way into the Kubernetes API.
		on := len(exchanging) > 0
		if err := unstructured.SetNestedField(provider.Object, on, "spec", "tokenExchange", "enabled"); err != nil {
			return fmt.Errorf("set token exchange of %s: %w", provider.GetName(), err)
		}
		// The update carries the resourceVersion read: a provider changed since then is refused with a conflict and
		// read again at the next pass.
		if err := m.client.Update(ctx, provider); err != nil {
			return fmt.Errorf("update DexProvider %s: %w", provider.GetName(), err)
		}
		if on {
			enabled = append(enabled, provider.GetName())
		} else {
			disabled = append(disabled, provider.GetName())
		}
	}

	marker = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DexProviderTokenExchangeMarker,
			Namespace: naming.DexNamespace,
			Labels:    map[string]string{"heritage": "deckhouse", "module": "user-authn"},
		},
		Data: map[string]string{
			"enabled":  strings.Join(enabled, ","),
			"disabled": strings.Join(disabled, ","),
			"clients":  strings.Join(exchanging, ","),
		},
	}
	if err := m.client.Create(ctx, marker); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create the marker: %w", err)
	}
	m.log.Info("Set token exchange on the DexProviders", "enabled", enabled, "disabled", disabled, "clients", exchanging)
	return nil
}

// clientsAllowedToKubernetes returns the DexClients, as namespace/name, whose allow-access-to-kubernetes annotation
// makes them trusted peers of the kubernetes client: through an exchanging connector, such a client gets a token the
// Kubernetes API accepts. A DexClient being deleted is no longer a peer (the kubernetesclient controller drops it).
//
// The other peers are not counted. A DexAuthenticator with the annotation is one too, but its oauth2-proxy never
// exchanges tokens: only someone who took the client secret from the Secret in the namespace of the application could,
// and that path is not worth keeping open. The peers of the module configuration are kubeconfig clients, to which patch
// 425 refuses token exchange.
func (m *DexProviderTokenExchange) clientsAllowedToKubernetes(ctx context.Context) ([]string, error) {
	clients := &metav1.PartialObjectMetadataList{}
	clients.SetGroupVersionKind(controller.DexClientGVK.GroupVersion().WithKind(controller.DexClientGVK.Kind + "List"))
	if err := m.reader.List(ctx, clients); err != nil {
		return nil, fmt.Errorf("list DexClients: %w", err)
	}
	var allowed []string
	for _, c := range clients.Items {
		if c.DeletionTimestamp != nil {
			continue
		}
		// A value that is not a boolean gives no access (controller.AllowsAccessToKubernetes).
		if ok, _ := controller.AllowsAccessToKubernetes(c.Annotations, controller.DexClientAllowAccessToKubernetesAnnotation); ok {
			allowed = append(allowed, c.Namespace+"/"+c.Name)
		}
	}
	slices.Sort(allowed)
	return allowed, nil
}
