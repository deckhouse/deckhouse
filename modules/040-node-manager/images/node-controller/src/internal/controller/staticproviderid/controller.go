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

package staticproviderid

import (
	"context"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController("static-provider-id", &corev1.Node{}, &Reconciler{})
}

const (
	nodeTypeLabel         = nodecommon.NodeTypeLabel
	nodeTypeStatic        = "Static"
	uninitializedTaintKey = "node.cloudprovider.kubernetes.io/uninitialized"
	staticProviderIDValue = "static://"
)

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(_ register.Watcher) {}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if node.Labels[nodeTypeLabel] != nodeTypeStatic {
		logger.V(1).Info("skipping: node is not Static type", "node", node.Name, "type", node.Labels[nodeTypeLabel])
		return ctrl.Result{}, nil
	}

	if node.Spec.ProviderID != "" {
		logger.V(1).Info("skipping: providerID already set", "node", node.Name, "providerID", node.Spec.ProviderID)
		return ctrl.Result{}, nil
	}

	for _, taint := range node.Spec.Taints {
		if taint.Key == uninitializedTaintKey {
			logger.V(1).Info("skipping: node has uninitialized taint", "node", node.Name)
			return ctrl.Result{}, nil
		}
	}

	logger.Info("patching providerID on static node", "node", node.Name, "providerID", staticProviderIDValue)

	patch, err := json.Marshal(map[string]interface{}{
		"spec": map[string]string{
			"providerID": staticProviderIDValue,
		},
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Client.Patch(ctx, node, client.RawPatch(types.MergePatchType, patch)); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("providerID set successfully", "node", node.Name)
	return ctrl.Result{}, nil
}
