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

package registry

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	namespace           = "d8-system"
	stateControllerName = "registry-state-controller"
)

var _ reconcile.Reconciler = &StateController{}

type StateController struct {
	controllerName string
	namespace      string
	client         client.Client
	dataCh         chan HashedRegistryData
}

type HashedRegistryData struct {
	HashSum string
	Data    map[string]interface{}
}

func (sc *StateController) SetupWithManager(ctx context.Context, ctrlManager ctrl.Manager) <-chan HashedRegistryData {
	sc.controllerName = stateControllerName
	sc.namespace = namespace
	sc.client = ctrlManager.GetClient()
	sc.dataCh = make(chan HashedRegistryData)

	secretsPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if obj.GetNamespace() != sc.namespace {
			return false
		}
		name := obj.GetName()
		return name == deckhouseRegistrySecretName ||
			name == bashibleConfigSecretName
	})

	klog.Infof("Setting up controller %q with manager", sc.controllerName)

	err := ctrl.NewControllerManagedBy(ctrlManager).
		Named(sc.controllerName).
		For(&corev1.Secret{}, builder.WithPredicates(secretsPredicate)).
		Complete(sc)
	if err != nil {
		klog.Fatalf("cannot build %q controller: %v", sc.controllerName, err)
	}
	return sc.dataCh
}

func (sc *StateController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// The module's own layout first, because it decides whether anything else is needed at all:
	// `loadFromInput` below uses it when it exists and ignores the secret entirely.
	bashibleCfgSecret, err := sc.loadBashibleCfgSecret(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot load bashible config secret: %w", err)
	}

	// And the secret is required only when nothing else describes the registry.
	//
	// It used to be loaded unconditionally, which made its absence fatal here — and this controller is
	// what produces a node's bootstrap data. Measured on a cluster where that secret was removed on
	// purpose: no bootstrap data secret was produced at all, CAPI reported `Secret "worker-6a06716b"
	// not found`, and no worker VM ever appeared; the installation ended on "waiting for a Ready worker
	// node". Nothing in that path had needed the secret: the layout was there and is what the node is
	// configured from.
	deckhouseRegistrySecret, err := sc.loadDeckhouseRegistrySecret(ctx)
	if err != nil {
		if bashibleCfgSecret == nil {
			return ctrl.Result{}, fmt.Errorf("cannot load deckhouse registry secret: %w", err)
		}

		klog.Infof("%s: no usable %q secret, continuing from %q, which is what the node is configured from: %v",
			sc.controllerName, deckhouseRegistrySecretName, bashibleConfigSecretName, err)
	}

	rData := RegistryData{}
	err = rData.loadFromInput(deckhouseRegistrySecret, bashibleCfgSecret)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot create registry data: %w", err)
	}

	err = rData.validate()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("validation error for registry data: %w", err)
	}

	hashSum, err := rData.hashSum()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("hash sum generate error for registry data: %w", err)
	}

	mapData, err := rData.toMap()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to map registry data: %w", err)
	}

	sc.dataCh <- HashedRegistryData{HashSum: hashSum, Data: mapData}
	return ctrl.Result{}, nil
}

func (sc *StateController) loadDeckhouseRegistrySecret(ctx context.Context) (deckhouseRegistrySecret, error) {
	ret := deckhouseRegistrySecret{}
	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      deckhouseRegistrySecretName,
		Namespace: sc.namespace,
	}

	if err := sc.client.Get(ctx, key, &secret); err != nil {
		return ret, fmt.Errorf("cannot get secret %v k8s object: %w", key.Name, err)
	}

	ret.decode(&secret)

	if err := ret.validate(); err != nil {
		return ret, fmt.Errorf("validation error: %w", err)
	}

	return ret, nil
}

func (sc *StateController) loadBashibleCfgSecret(ctx context.Context) (*bashibleConfigSecret, error) {
	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      bashibleConfigSecretName,
		Namespace: sc.namespace,
	}

	if err := sc.client.Get(ctx, key, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot get secret %v k8s object: %w", key.Name, err)
	}

	ret := bashibleConfigSecret{}
	if err := ret.decode(&secret); err != nil {
		return nil, fmt.Errorf("cannot decode from secret: %w", err)
	}

	if err := ret.validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	return &ret, nil
}
