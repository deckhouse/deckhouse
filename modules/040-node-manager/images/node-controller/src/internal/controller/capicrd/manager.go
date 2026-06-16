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

// Package capicrd owns the lifecycle of the core Cluster API CRDs
// (cluster.x-k8s.io and runtime.cluster.x-k8s.io). It installs the CRDs,
// keeps the conversion webhook configuration up to date and performs the
// guarded migration of the storage version from v1beta1 to v1beta2.
//
// The storage flip is guarded: storage stays v1beta1 until the
// capi-controller-manager (which serves the v1beta1<->v1beta2 conversion
// webhook) is fully rolled out to the expected image. This guarantees that a
// v1beta2-capable converter is available before anything is stored as or read
// as v1beta2, which avoids the upgrade deadlock where the old controller
// cannot convert to v1beta2.
package capicrd

import (
	"context"
	"embed"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/yaml"
)

//go:embed crds/*.yaml
var crdFS embed.FS

const (
	capiNamespace            = "d8-cloud-instance-manager"
	capiControllerDeployment = "capi-controller-manager"
	webhookSecretName        = "capi-webhook-tls"
	webhookServiceName       = "capi-webhook-service"

	servedVersionV1beta1 = "v1beta1"
	targetStorageVersion = "v1beta2"

	reconcileInterval = 15 * time.Second
)

var log = logf.Log.WithName("capi-crd-manager")

// Manager installs and maintains the core CAPI CRDs and migrates their storage
// version to v1beta2 once the new controller is ready.
type Manager struct {
	client  client.Client
	dynamic dynamic.Interface

	crds []apiextensionsv1.CustomResourceDefinition
}

var _ manager.Runnable = (*Manager)(nil)

func newManager(cfg *rest.Config) (*Manager, error) {
	crds, err := loadCRDs()
	if err != nil {
		return nil, fmt.Errorf("load embedded CRDs: %w", err)
	}

	scheme := runtime.NewScheme()
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("add apiextensions to scheme: %w", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("add apps to scheme: %w", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("add core to scheme: %w", err)
	}

	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("build direct client: %w", err)
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build dynamic client: %w", err)
	}

	return &Manager{client: cl, dynamic: dyn, crds: crds}, nil
}

// EnsureCRDs installs the core CAPI CRDs with both versions served and storage
// kept at v1beta1. It MUST run before the controller-runtime manager starts:
// the manager cache watches MachineDeployment v1beta2, which fails to sync if
// the CRD does not yet serve v1beta2 (e.g. upgrading from a CAPI 1.10.x release
// that only had v1beta1). The guarded storage flip to v1beta2 happens later in
// the Runnable.
func EnsureCRDs(ctx context.Context, cfg *rest.Config) error {
	m, err := newManager(cfg)
	if err != nil {
		return err
	}

	caBundle, err := m.readCABundle(ctx)
	if err != nil {
		return fmt.Errorf("read conversion CA bundle: %w", err)
	}

	for i := range m.crds {
		if err := m.ensureCRD(ctx, &m.crds[i], servedVersionV1beta1, caBundle); err != nil {
			return fmt.Errorf("ensure CRD %s: %w", m.crds[i].Name, err)
		}
	}
	return nil
}

// SetupWithManager registers the storage-flip Runnable on mgr.
func SetupWithManager(mgr manager.Manager) error {
	m, err := newManager(mgr.GetConfig())
	if err != nil {
		return err
	}
	return mgr.Add(m)
}

// Start runs the reconcile loop until ctx is cancelled.
func (m *Manager) Start(ctx context.Context) error {
	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()

	for {
		if err := m.reconcile(ctx); err != nil {
			log.Error(err, "reconcile failed, will retry")
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (m *Manager) reconcile(ctx context.Context) error {
	gateOpen, err := m.capiControllerReady(ctx)
	if err != nil {
		return fmt.Errorf("check capi-controller-manager readiness: %w", err)
	}

	storageVersion := servedVersionV1beta1
	if gateOpen {
		storageVersion = targetStorageVersion
	}

	caBundle, err := m.readCABundle(ctx)
	if err != nil {
		return fmt.Errorf("read conversion CA bundle: %w", err)
	}

	for i := range m.crds {
		if err := m.ensureCRD(ctx, &m.crds[i], storageVersion, caBundle); err != nil {
			return fmt.Errorf("ensure CRD %s: %w", m.crds[i].Name, err)
		}
	}

	if !gateOpen {
		log.V(1).Info("storage flip gated: waiting for capi-controller-manager rollout")
		return nil
	}

	if err := m.migrateStorage(ctx); err != nil {
		return fmt.Errorf("migrate storage to %s: %w", targetStorageVersion, err)
	}

	return nil
}

// capiControllerReady reports whether the capi-controller-manager Deployment is
// fully rolled out on its current generation. Because helm applies the
// capi-controller-manager and node-controller specs in the same release before
// this pod starts, a completed rollout means only the new (v1beta2-capable)
// pods serve the conversion webhook — so no separate image check is needed.
func (m *Manager) capiControllerReady(ctx context.Context) (bool, error) {
	var dep appsv1.Deployment
	key := client.ObjectKey{Namespace: capiNamespace, Name: capiControllerDeployment}
	if err := m.client.Get(ctx, key, &dep); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("get deployment: %w", err)
	}

	return deploymentRolloutComplete(&dep), nil
}

func deploymentRolloutComplete(dep *appsv1.Deployment) bool {
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas == 0 {
		return false
	}
	if dep.Status.ObservedGeneration < dep.Generation {
		return false
	}
	desired := *dep.Spec.Replicas
	return dep.Status.UpdatedReplicas == desired &&
		dep.Status.ReadyReplicas == desired &&
		dep.Status.AvailableReplicas == desired
}

func (m *Manager) readCABundle(ctx context.Context) ([]byte, error) {
	var secret corev1.Secret
	key := client.ObjectKey{Namespace: capiNamespace, Name: webhookSecretName}
	if err := m.client.Get(ctx, key, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get secret: %w", err)
	}

	ca := secret.Data["ca.crt"]
	if len(ca) == 0 {
		return nil, nil
	}
	return ca, nil
}

func loadCRDs() ([]apiextensionsv1.CustomResourceDefinition, error) {
	entries, err := crdFS.ReadDir("crds")
	if err != nil {
		return nil, fmt.Errorf("read embedded crds dir: %w", err)
	}

	crds := make([]apiextensionsv1.CustomResourceDefinition, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}

		data, err := crdFS.ReadFile("crds/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read embedded crd %s: %w", e.Name(), err)
		}

		var crd apiextensionsv1.CustomResourceDefinition
		if err := yaml.Unmarshal(data, &crd); err != nil {
			return nil, fmt.Errorf("unmarshal crd %s: %w", e.Name(), err)
		}
		crds = append(crds, crd)
	}

	return crds, nil
}

func conversionConfig(caBundle []byte) *apiextensionsv1.CustomResourceConversion {
	port := int32(443)
	path := "/convert"
	return &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ClientConfig: &apiextensionsv1.WebhookClientConfig{
				Service: &apiextensionsv1.ServiceReference{
					Namespace: capiNamespace,
					Name:      webhookServiceName,
					Path:      &path,
					Port:      &port,
				},
				CABundle: caBundle,
			},
			ConversionReviewVersions: []string{"v1"},
		},
	}
}

// ensureCRD creates or updates a single CAPI CRD with the desired conversion
// webhook configuration and storage version. node-controller is the sole owner
// of these CRDs, so the spec is overwritten wholesale.
func (m *Manager) ensureCRD(ctx context.Context, src *apiextensionsv1.CustomResourceDefinition, storageVersion string, caBundle []byte) error {
	desired := src.DeepCopy()
	desired.Spec.Conversion = conversionConfig(caBundle)
	for i := range desired.Spec.Versions {
		desired.Spec.Versions[i].Storage = desired.Spec.Versions[i].Name == storageVersion
	}
	if desired.Labels == nil {
		desired.Labels = make(map[string]string, 1)
	}
	desired.Labels["heritage"] = "deckhouse"

	var existing apiextensionsv1.CustomResourceDefinition
	err := m.client.Get(ctx, client.ObjectKey{Name: desired.Name}, &existing)
	if apierrors.IsNotFound(err) {
		if err := m.client.Create(ctx, desired); err != nil {
			return fmt.Errorf("create: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get: %w", err)
	}

	existing.Spec = desired.Spec
	if existing.Labels == nil {
		existing.Labels = make(map[string]string, 1)
	}
	existing.Labels["heritage"] = "deckhouse"
	if err := m.client.Update(ctx, &existing); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	return nil
}
