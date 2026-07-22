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

// Package defaultcrimigration emits the D8ObsoleteDefaultCRIInClusterConfiguration
// alert metric. The `defaultCRI` option is being migrated from the deprecated
// ClusterConfiguration field to the node-manager ModuleConfig; this controller
// reports when a cluster still relies on the ClusterConfiguration value.
package defaultcrimigration

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	// defaultCRIType is the built-in container runtime used when neither
	// ClusterConfiguration nor the node-manager ModuleConfig specify one.
	defaultCRIType = "Containerd"

	clusterConfigurationSecretNamespace = "kube-system"
	clusterConfigurationSecretName      = "d8-cluster-configuration"
	clusterConfigurationSecretKey       = "cluster-configuration.yaml"

	nodeManagerModuleConfigName = "node-manager"

	// checkInterval is how often the migration state is re-evaluated. The
	// ClusterConfiguration value lives in a Secret this controller does not
	// watch, so the periodic requeue is what catches its edits.
	checkInterval = 5 * time.Minute
)

// reconcileRequest is the single synthetic request all triggers collapse to:
// the check is cluster-global, not tied to any one object.
var reconcileRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "default-cri-migration"}}

func newModuleConfig() *unstructured.Unstructured {
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleConfig"})
	return mc
}

func init() {
	// The node-manager ModuleConfig is the primary object: editing it (i.e.
	// completing the migration) re-runs the check and clears the alert promptly.
	register.RegisterController("default-cri-migration", newModuleConfig(), &Reconciler{})
}

type Reconciler struct {
	register.Base
	apiReader client.Reader
}

func (r *Reconciler) Setup(mgr ctrl.Manager) error {
	// Read directly from the API server: the manager cache does not watch the
	// d8-cluster-configuration Secret.
	r.apiReader = mgr.GetAPIReader()
	return nil
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	// Evaluate once at startup even if the node-manager ModuleConfig does not
	// exist yet (no informer event would fire in that case).
	w.WatchesRawSource(source.Func(func(_ context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
		q.Add(reconcileRequest)
		return nil
	}))
}

func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	ccCRI, err := r.clusterConfigurationDefaultCRI(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	mcCRI, err := r.moduleConfigDefaultCRI(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	obsolete := isObsoleteDefaultCRI(ccCRI, mcCRI)
	setObsoleteDefaultCRIMetric(obsolete)
	if obsolete {
		logger.Info("defaultCRI is set in ClusterConfiguration but not migrated to the node-manager ModuleConfig",
			"clusterConfigurationDefaultCRI", ccCRI)
	}

	return ctrl.Result{RequeueAfter: checkInterval}, nil
}

// isObsoleteDefaultCRI reports whether ClusterConfiguration still carries a
// non-default defaultCRI that has not been migrated to the node-manager
// ModuleConfig (which is either unset or left at the default value).
func isObsoleteDefaultCRI(clusterConfigCRI, moduleConfigCRI string) bool {
	if clusterConfigCRI == "" || clusterConfigCRI == defaultCRIType {
		return false
	}
	return moduleConfigCRI == "" || moduleConfigCRI == defaultCRIType
}

func (r *Reconciler) clusterConfigurationDefaultCRI(ctx context.Context) (string, error) {
	secret := &corev1.Secret{}
	err := r.apiReader.Get(ctx, types.NamespacedName{
		Namespace: clusterConfigurationSecretNamespace,
		Name:      clusterConfigurationSecretName,
	}, secret)
	if err != nil {
		if errors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("get secret %s/%s: %w", clusterConfigurationSecretNamespace, clusterConfigurationSecretName, err)
	}

	data, ok := secret.Data[clusterConfigurationSecretKey]
	if !ok {
		return "", nil
	}

	var cfg struct {
		DefaultCRI string `json:"defaultCRI"`
	}
	if err := sigsyaml.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse %s: %w", clusterConfigurationSecretKey, err)
	}
	return cfg.DefaultCRI, nil
}

func (r *Reconciler) moduleConfigDefaultCRI(ctx context.Context) (string, error) {
	mc := newModuleConfig()
	err := r.apiReader.Get(ctx, types.NamespacedName{Name: nodeManagerModuleConfigName}, mc)
	if err != nil {
		if errors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("get ModuleConfig %q: %w", nodeManagerModuleConfigName, err)
	}

	cri, _, err := unstructured.NestedString(mc.Object, "spec", "settings", "defaultCRI")
	if err != nil {
		return "", fmt.Errorf("read spec.settings.defaultCRI from ModuleConfig %q: %w", nodeManagerModuleConfigName, err)
	}
	return cri, nil
}
