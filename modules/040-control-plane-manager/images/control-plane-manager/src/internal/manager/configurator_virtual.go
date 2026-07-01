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

package manager

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimemanager "sigs.k8s.io/controller-runtime/pkg/manager"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	virtualcontrolplaneconfiguration "control-plane-manager/internal/controllers/virtual-control-plane-configuration"
	virtualcontrolplanenode "control-plane-manager/internal/controllers/virtual-control-plane-node"
	virtualcontrolplaneoperation "control-plane-manager/internal/controllers/virtual-control-plane-operation"
)

type virtualConfigurator struct{}

func (c *virtualConfigurator) configureOptions(opts *controllerruntime.Options) {
	opts.LeaderElection = true
	opts.LeaderElectionID = constants.VirtualControlPlaneManagerName
	opts.Client = client.Options{
		Cache: &client.CacheOptions{
			DisableFor: []client.Object{
				&corev1.Namespace{},
				&corev1.Secret{},
				&corev1.Service{},
				&corev1.Pod{},
				&appsv1.StatefulSet{},
			},
		},
	}

	opts.Cache = cache.Options{
		ReaderFailOnMissingInformer: false,
		DefaultTransform:            cache.TransformStripManagedFields(),
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Secret{}: {
				Namespaces: map[string]cache.Config{
					constants.KubeSystemNamespace: {
						FieldSelector: fields.OneTermEqualSelector("metadata.name", constants.VirtualControlPlaneConfigSecretName),
					},
				},
			},
			&controlplanev1alpha1.ControlPlaneNode{}: {
				Label: labels.SelectorFromSet(labels.Set{
					constants.ControlPlaneTypeLabelKey: string(constants.ControlPlaneTypeVirtual),
				}),
			},
			&controlplanev1alpha1.ControlPlaneOperation{}: {
				Label: labels.SelectorFromSet(labels.Set{
					constants.ControlPlaneTypeLabelKey: string(constants.ControlPlaneTypeVirtual),
				}),
			},
		},
	}
}

func (c *virtualConfigurator) configureRuntimeManager(runtimeManager runtimemanager.Manager) error {
	if err := virtualcontrolplaneconfiguration.BuildController(runtimeManager); err != nil {
		return fmt.Errorf("build virtual-control-plane-configuration controller: %w", err)
	}

	if err := virtualcontrolplanenode.BuildController(runtimeManager); err != nil {
		return fmt.Errorf("build virtual-control-plane-node controller: %w", err)
	}

	if err := virtualcontrolplaneoperation.BuildController(runtimeManager); err != nil {
		return fmt.Errorf("build virtual-control-plane-operation controller: %w", err)
	}

	return nil
}
