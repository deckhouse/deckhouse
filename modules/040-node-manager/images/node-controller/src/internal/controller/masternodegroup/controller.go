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

// Package masternodegroup ensures the default `master` NodeGroup metadata object
// exists. It replaces the OnStartup hook create_master_node_group.
//
// The NodeGroup is metadata only: during bootstrap the master Node is registered
// directly by kubeadm via bashible, so the object is not on the cluster's critical
// path. The reconcile is idempotent (create-if-not-exists) and never patches an
// existing object, to avoid clobbering user changes.
package masternodegroup

import (
	"context"
	"encoding/base64"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	sigsyaml "sigs.k8s.io/yaml"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/register"
)

const (
	masterNodeGroupName          = "master"
	clusterConfigSecretName      = "d8-cluster-configuration"
	clusterConfigSecretNamespace = "kube-system"
	clusterConfigKey             = "cluster-configuration.yaml"
	clusterTypeStatic            = "Static"
)

func init() {
	register.RegisterController("master-node-group", &deckhousev1.NodeGroup{}, &Reconciler{})
}

type Reconciler struct {
	register.Base
	apiReader client.Reader
}

func (r *Reconciler) Setup(mgr ctrl.Manager) error {
	r.apiReader = mgr.GetAPIReader()
	return nil
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	// Only react to the `master` NodeGroup: recreate it if the user deletes it,
	// ignore events for every other NodeGroup.
	w.WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == masterNodeGroupName
	}))

	// On a fresh cluster the `master` NodeGroup does not exist yet, so the primary
	// watch never fires. Enqueue it once at startup so we create it.
	w.WatchesRawSource(source.Func(func(_ context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
		q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: masterNodeGroupName}})
		return nil
	}))
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Name != masterNodeGroupName {
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx)

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(deckhousev1.GroupVersion.WithKind("NodeGroup"))
	err := r.Client.Get(ctx, types.NamespacedName{Name: masterNodeGroupName}, existing)
	if err == nil {
		// Do not patch: preserve user changes to the master NodeGroup.
		return ctrl.Result{}, nil
	}
	if !errors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("getting NodeGroup %s: %w", masterNodeGroupName, err)
	}

	clusterType, err := r.readClusterType(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("reading cluster type: %w", err)
	}

	ng := defaultMasterNodeGroup(clusterType)
	if err := r.Client.Create(ctx, ng); err != nil {
		if errors.IsAlreadyExists(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("creating NodeGroup %s: %w", masterNodeGroupName, err)
	}

	logger.Info("created default master NodeGroup", "clusterType", clusterType)
	return ctrl.Result{}, nil
}

func (r *Reconciler) readClusterType(ctx context.Context) (string, error) {
	secret := &corev1.Secret{}
	if err := r.apiReader.Get(ctx, types.NamespacedName{
		Namespace: clusterConfigSecretNamespace,
		Name:      clusterConfigSecretName,
	}, secret); err != nil {
		return "", err
	}

	raw, ok := secret.Data[clusterConfigKey]
	if !ok {
		return "", fmt.Errorf("secret %s/%s has no %s key", clusterConfigSecretNamespace, clusterConfigSecretName, clusterConfigKey)
	}
	// The value is stored base64-wrapped inside the secret; unwrap it if so.
	if decoded, err := base64.StdEncoding.DecodeString(string(raw)); err == nil {
		raw = decoded
	}

	var cfg struct {
		ClusterType string `json:"clusterType"`
	}
	if err := sigsyaml.Unmarshal(raw, &cfg); err != nil {
		return "", fmt.Errorf("parsing %s: %w", clusterConfigKey, err)
	}
	return cfg.ClusterType, nil
}

// defaultMasterNodeGroup mirrors the object built by the create_master_node_group
// hook. It is intentionally an unstructured object with only the fields below set:
// a typed NodeGroup would marshal empty CRI/CloudInstances structs that fail
// admission validation for a master NodeGroup.
func defaultMasterNodeGroup(clusterType string) *unstructured.Unstructured {
	nodeType := "CloudPermanent"
	if clusterType == clusterTypeStatic {
		nodeType = "Static"
	}

	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "NodeGroup",
		"metadata": map[string]interface{}{
			"name": masterNodeGroupName,
		},
		"spec": map[string]interface{}{
			"nodeType": nodeType,
			"disruptions": map[string]interface{}{
				"approvalMode": "Manual",
			},
			"nodeTemplate": map[string]interface{}{
				"labels": map[string]interface{}{
					"node-role.kubernetes.io/control-plane": "",
					"node-role.kubernetes.io/master":        "",
				},
				"taints": []interface{}{
					map[string]interface{}{
						"key":    "node-role.kubernetes.io/control-plane",
						"effect": "NoSchedule",
					},
				},
			},
		},
	}}
}
