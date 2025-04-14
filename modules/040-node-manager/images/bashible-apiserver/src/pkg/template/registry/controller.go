// Copyright 2024 Flant JSC
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
	Namespace      = "d8-system"
	ControllerName = "registry-state-controller"
)

type RegistryStateController = registryStateController

var _ reconcile.Reconciler = &registryStateController{}

type registryStateController struct {
	Namespace      string
	client         client.Client
	registryDataCh chan RegistryDataWithHash
}

type RegistryDataWithHash struct {
	HashSum      string
	RegistryData RegistryData
}

func NewStateController() *RegistryStateController {
	return &RegistryStateController{
		Namespace: Namespace,
	}
}

func (sc *registryStateController) SetupWithManager(ctx context.Context, ctrlManager ctrl.Manager) chan RegistryDataWithHash {
	controllerName := ControllerName
	sc.client = ctrlManager.GetClient()
	sc.registryDataCh = make(chan RegistryDataWithHash)

	secretsPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if obj.GetNamespace() != sc.Namespace {
			return false
		}
		name := obj.GetName()
		return name == DeckhouseRegistrySecretName ||
			name == RegistryBashibleConfigSecretName
	})

	klog.Infof("Setting up controller %q with manager", controllerName)

	err := ctrl.NewControllerManagedBy(ctrlManager).
		Named(controllerName).
		For(&corev1.Secret{}, builder.WithPredicates(secretsPredicate)).
		Complete(sc)
	if err != nil {
		klog.Fatalf("cannot build %q controller: %v", controllerName, err)
	}
	return sc.registryDataCh
}

func (sc *registryStateController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	deckhouseRegistrySecret, err := sc.loadDeckhouseRegistrySecrets(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot load deckhouse registry secrets: %w", err)
	}

	registryBashibleConfigSecret, err := sc.loadRegistryBashibleConfigSecret(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot load registry bashible config secrets: %w", err)
	}

	registryData := RegistryData{}
	err = registryData.FromInputData(deckhouseRegistrySecret, registryBashibleConfigSecret)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot create registry data: %w", err)
	}

	err = registryData.Validate()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("validation error for registry data: %w", err)
	}

	hashSum, err := registryData.hashSum()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("hash sum generate error for registry data: %w", err)
	}

	sc.registryDataCh <- RegistryDataWithHash{HashSum: hashSum, RegistryData: registryData}
	return ctrl.Result{}, nil
}

func (sc *registryStateController) loadDeckhouseRegistrySecrets(ctx context.Context) (deckhouseRegistry, error) {
	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      DeckhouseRegistrySecretName,
		Namespace: sc.Namespace,
	}

	if err := sc.client.Get(ctx, key, &secret); err != nil {
		return deckhouseRegistry{}, fmt.Errorf("cannot get secret %v k8s object: %w", key.Name, err)
	}

	ret := deckhouseRegistry{}
	if err := ret.DecodeSecret(&secret); err != nil {
		return deckhouseRegistry{}, fmt.Errorf("cannot decode from secret: %w", err)
	}

	if err := ret.Validate(); err != nil {
		return deckhouseRegistry{}, fmt.Errorf("validation error: %w", err)
	}

	return ret, nil
}

func (sc *registryStateController) loadRegistryBashibleConfigSecret(ctx context.Context) (*registryBashibleConfig, error) {
	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      RegistryBashibleConfigSecretName,
		Namespace: sc.Namespace,
	}

	if err := sc.client.Get(ctx, key, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot get secret %v k8s object: %w", key.Name, err)
	}

	ret := registryBashibleConfig{}
	if err := ret.DecodeSecret(&secret); err != nil {
		return nil, fmt.Errorf("cannot decode from secret: %w", err)
	}

	if err := ret.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	return &ret, nil
}
