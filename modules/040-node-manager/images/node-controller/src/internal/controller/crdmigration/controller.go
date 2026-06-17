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

package crdmigration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/deckhouse/node-controller/internal/register"
)

const (
	capiNamespace       = "d8-cloud-instance-manager"
	webhookServiceName  = "capi-webhook-service"
	webhookSecretName   = "capi-webhook-tls"
	webhookServicePort  = int32(443)
	crdDir              = "/crds"
	requeuePrecondition = 30 * time.Second
	requeueAfterCreate  = 5 * time.Second
)

// capiCRDNames is the set of CAPI CRD names managed by this controller.
var capiCRDNames = map[string]bool{
	"clusters.cluster.x-k8s.io":            true,
	"machines.cluster.x-k8s.io":            true,
	"machinesets.cluster.x-k8s.io":         true,
	"machinedeployments.cluster.x-k8s.io":  true,
	"machinehealthchecks.cluster.x-k8s.io": true,
	"machinedrainrules.cluster.x-k8s.io":   true,
	"machinepools.cluster.x-k8s.io":        true,
	"extensionconfigs.runtime.cluster.x-k8s.io": true,
}

func init() {
	register.RegisterController("capi-crd-migration", &apiextensionsv1.CustomResourceDefinition{}, &Reconciler{})
}

type Reconciler struct {
	register.Base
	apiReader client.Reader
	crdSpecs  map[string]*apiextensionsv1.CustomResourceDefinition
}

func (r *Reconciler) Setup(mgr ctrl.Manager) error {
	r.apiReader = mgr.GetAPIReader()

	crds, err := loadCRDs(crdDir)
	if err != nil {
		return fmt.Errorf("loading CRD manifests: %w", err)
	}
	r.crdSpecs = crds

	return nil
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	// On fresh install, CAPI CRDs don't exist yet — the informer is empty.
	// Enqueue all 8 CRD names at startup to trigger creation.
	w.WatchesRawSource(source.Func(func(ctx context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
		for name := range capiCRDNames {
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
		}
		return nil
	}))
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	crdName := req.Name
	if !capiCRDNames[crdName] {
		return ctrl.Result{}, nil
	}

	embeddedCRD, ok := r.crdSpecs[crdName]
	if !ok {
		logger.Info("no embedded CRD manifest found", "crd", crdName)
		return ctrl.Result{}, nil
	}

	// Check preconditions via uncached reader.
	caBundle, err := r.checkPreconditions(ctx)
	if err != nil {
		logger.Info("preconditions not met, requeue", "crd", crdName, "reason", err.Error())
		return ctrl.Result{RequeueAfter: requeuePrecondition}, nil
	}

	// Get current CRD state (uncached).
	existing := &apiextensionsv1.CustomResourceDefinition{}
	err = r.apiReader.Get(ctx, types.NamespacedName{Name: crdName}, existing)
	if errors.IsNotFound(err) {
		// Fresh install — create CRD with v1beta2 storage + conversion webhook.
		logger.Info("CRD not found, creating with migration applied", "crd", crdName)
		newCRD := embeddedCRD.DeepCopy()
		setMigrationSpec(newCRD, caBundle)
		if err := r.Client.Create(ctx, newCRD); err != nil {
			if errors.IsAlreadyExists(err) {
				// Race condition — another replica or process created it first.
				return ctrl.Result{RequeueAfter: requeueAfterCreate}, nil
			}
			return ctrl.Result{}, fmt.Errorf("creating CRD %s: %w", crdName, err)
		}
		logger.Info("CRD created", "crd", crdName)
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting CRD %s: %w", crdName, err)
	}

	// Check if already fully migrated.
	if isMigrated(existing, caBundle) {
		logger.V(1).Info("CRD already migrated", "crd", crdName)
		return ctrl.Result{}, nil
	}

	// Patch CRD: switch storage to v1beta2 and set conversion webhook.
	logger.Info("migrating CRD", "crd", crdName)
	patch := existing.DeepCopy()
	setMigrationSpec(patch, caBundle)

	if err := r.Client.Patch(ctx, patch, client.MergeFrom(existing)); err != nil {
		return ctrl.Result{}, fmt.Errorf("patching CRD %s: %w", crdName, err)
	}

	logger.Info("CRD migrated successfully", "crd", crdName)
	return ctrl.Result{}, nil
}

// checkPreconditions verifies that the CA bundle for the conversion webhook is available.
// We do NOT wait for capi-controller-manager Deployment — it may be failing because CRDs
// are not yet served in v1beta2. We only need the CA to configure the conversion webhook.
func (r *Reconciler) checkPreconditions(ctx context.Context) ([]byte, error) {
	secret := &corev1.Secret{}
	if err := r.apiReader.Get(ctx, types.NamespacedName{
		Name:      webhookSecretName,
		Namespace: capiNamespace,
	}, secret); err != nil {
		return nil, fmt.Errorf("secret %s/%s: %w", capiNamespace, webhookSecretName, err)
	}

	caBundle := secret.Data["ca.crt"]
	if len(caBundle) == 0 {
		return nil, fmt.Errorf("secret %s/%s has empty ca.crt", capiNamespace, webhookSecretName)
	}

	return caBundle, nil
}

// isMigrated checks if the CRD is already in the target state.
func isMigrated(crd *apiextensionsv1.CustomResourceDefinition, caBundle []byte) bool {
	// Check v1beta2 is storage version.
	v1beta2Storage := false
	for _, v := range crd.Spec.Versions {
		if v.Name == "v1beta2" && v.Storage {
			v1beta2Storage = true
		}
	}
	if !v1beta2Storage {
		return false
	}

	// Check conversion webhook is set correctly.
	conv := crd.Spec.Conversion
	if conv == nil || conv.Strategy != apiextensionsv1.WebhookConverter {
		return false
	}
	wh := conv.Webhook
	if wh == nil || wh.ClientConfig == nil || wh.ClientConfig.Service == nil {
		return false
	}
	svc := wh.ClientConfig.Service
	if svc.Name != webhookServiceName || svc.Namespace != capiNamespace {
		return false
	}

	// Check caBundle matches.
	if string(wh.ClientConfig.CABundle) != string(caBundle) {
		return false
	}

	return true
}

// setMigrationSpec modifies the CRD spec to v1beta2 storage + conversion webhook.
func setMigrationSpec(crd *apiextensionsv1.CustomResourceDefinition, caBundle []byte) {
	// Switch storage versions.
	for i := range crd.Spec.Versions {
		switch crd.Spec.Versions[i].Name {
		case "v1beta2":
			crd.Spec.Versions[i].Storage = true
		case "v1beta1":
			crd.Spec.Versions[i].Storage = false
		}
	}

	// Set conversion webhook.
	crd.Spec.Conversion = &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ClientConfig: &apiextensionsv1.WebhookClientConfig{
				Service: &apiextensionsv1.ServiceReference{
					Name:      webhookServiceName,
					Namespace: capiNamespace,
					Port:      ptrInt32(webhookServicePort),
				},
				CABundle: caBundle,
			},
			ConversionReviewVersions: []string{"v1", "v1beta1"},
		},
	}
}

// loadCRDs reads CRD yaml files from the given directory.
func loadCRDs(dir string) (map[string]*apiextensionsv1.CustomResourceDefinition, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("globbing %s: %w", dir, err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no CRD yaml files found in %s", dir)
	}

	crds := make(map[string]*apiextensionsv1.CustomResourceDefinition, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", f, err)
		}

		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := sigsyaml.Unmarshal(data, crd); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", f, err)
		}

		if crd.Name == "" {
			return nil, fmt.Errorf("CRD in %s has no name", f)
		}

		crds[crd.Name] = crd
	}

	return crds, nil
}

func ptrInt32(v int32) *int32 {
	return &v
}
