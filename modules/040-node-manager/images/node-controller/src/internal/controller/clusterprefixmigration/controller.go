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

// Package clusterprefixmigration seeds the cluster prefix into the global
// ModuleConfig. The prefix is being migrated out of ClusterConfiguration
// (cloud.prefix, which is going away together with the whole cloud section)
// into the global ModuleConfig as spec.settings.prefix. This controller copies
// the value from the deprecated ClusterConfiguration.cloud.prefix into the
// global ModuleConfig when it is not already set, so in-cluster consumers can
// rely on global.prefix.
package clusterprefixmigration

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
	clusterConfigurationSecretNamespace = "kube-system"
	clusterConfigurationSecretName      = "d8-cluster-configuration"
	clusterConfigurationSecretKey       = "cluster-configuration.yaml"

	globalModuleConfigName = "global"
	// globalModuleConfigVersion is the current config-version of the global
	// module settings (see global-hooks/openapi/config-values.yaml).
	globalModuleConfigVersion = 2

	// checkInterval re-evaluates the migration as a safety net: the
	// ClusterConfiguration value lives in a Secret this controller does not watch.
	checkInterval     = 5 * time.Minute
	requeueOnConflict = 5 * time.Second
)

// reconcileRequest is the single synthetic request all triggers collapse to:
// the migration is cluster-global, not tied to any one object.
var reconcileRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "cluster-prefix-migration"}}

func newModuleConfig() *unstructured.Unstructured {
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleConfig"})
	return mc
}

func init() {
	// The global ModuleConfig is the primary object: editing it re-runs the
	// migration check, so a manually cleared prefix is re-seeded promptly.
	register.RegisterController("cluster-prefix-migration", newModuleConfig(), &Reconciler{})
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
	// Evaluate once at startup even if the global ModuleConfig does not exist yet.
	w.WatchesRawSource(source.Func(func(_ context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
		q.Add(reconcileRequest)
		return nil
	}))
}

func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	cloudPrefix, err := r.clusterConfigurationCloudPrefix(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cloudPrefix == "" {
		// Nothing to migrate: static cluster, or cloud.prefix unset/already removed.
		return ctrl.Result{RequeueAfter: checkInterval}, nil
	}

	mc := newModuleConfig()
	getErr := r.apiReader.Get(ctx, types.NamespacedName{Name: globalModuleConfigName}, mc)
	switch {
	case getErr == nil:
		if existing, _, _ := unstructured.NestedString(mc.Object, "spec", "settings", "prefix"); existing != "" {
			// Already migrated.
			return ctrl.Result{RequeueAfter: checkInterval}, nil
		}
		patched := mc.DeepCopy()
		if err := unstructured.SetNestedField(patched.Object, cloudPrefix, "spec", "settings", "prefix"); err != nil {
			return ctrl.Result{}, fmt.Errorf("set spec.settings.prefix: %w", err)
		}
		if err := r.Client.Patch(ctx, patched, client.MergeFrom(mc)); err != nil {
			return ctrl.Result{}, fmt.Errorf("patch global ModuleConfig prefix: %w", err)
		}
		logger.Info("seeded global ModuleConfig prefix from the deprecated ClusterConfiguration.cloud.prefix", "prefix", cloudPrefix)

	case errors.IsNotFound(getErr):
		created := newModuleConfig()
		created.SetName(globalModuleConfigName)
		if err := unstructured.SetNestedField(created.Object, int64(globalModuleConfigVersion), "spec", "version"); err != nil {
			return ctrl.Result{}, fmt.Errorf("set spec.version: %w", err)
		}
		if err := unstructured.SetNestedField(created.Object, cloudPrefix, "spec", "settings", "prefix"); err != nil {
			return ctrl.Result{}, fmt.Errorf("set spec.settings.prefix: %w", err)
		}
		if err := r.Client.Create(ctx, created); err != nil {
			if errors.IsAlreadyExists(err) {
				return ctrl.Result{RequeueAfter: requeueOnConflict}, nil
			}
			return ctrl.Result{}, fmt.Errorf("create global ModuleConfig: %w", err)
		}
		logger.Info("created global ModuleConfig with prefix from the deprecated ClusterConfiguration.cloud.prefix", "prefix", cloudPrefix)

	default:
		return ctrl.Result{}, fmt.Errorf("get global ModuleConfig: %w", getErr)
	}

	return ctrl.Result{RequeueAfter: checkInterval}, nil
}

func (r *Reconciler) clusterConfigurationCloudPrefix(ctx context.Context) (string, error) {
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
		Cloud struct {
			Prefix string `json:"prefix"`
		} `json:"cloud"`
	}
	if err := sigsyaml.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse %s: %w", clusterConfigurationSecretKey, err)
	}
	return cfg.Cloud.Prefix, nil
}
