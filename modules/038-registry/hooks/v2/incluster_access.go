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

package v2

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	registryServiceSnapName   = "registry-service"
	registryEndpointsSnapName = "registry-endpoints"
	inClusterAccessSnapName   = "incluster-access"

	// storageSelector is the label of the store's Service (and of its pods); legacyServiceSelector is what
	// the previous implementation's selected under the same name, see
	// modules/038-registry/images/registry-controller/src/internal/controller/layout/legacy_cleanup.go.
	storageSelector       = "registry-storage"
	legacyServiceSelector = "registry-service"
)

// Pods reach the key, registry.d8-system.svc:5001, through the Service `registry`, which leads them to
// the store (templates/v2/storage/service.yaml). Before that the Service was headless, and a Service's
// clusterIP cannot change, so the headless one is deleted here for Helm to create the routable one.
// Between the two the name does not resolve; what dials it at 4281 retries, and the nodes do not dial it.
//
// The previous implementation's Service of the same name is left to the controller's cleanup, and the
// template waits for it to go: taking it over would send a node still on that implementation to the store.
var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		// After the switch (5), before Helm renders the Service.
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 16},
		Queue:        "/modules/registry/v2",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       registryServiceSnapName,
				ApiVersion: "v1",
				Kind:       "Service",
				NameSelector: &types.NameSelector{
					MatchNames: []string{registry_const.ServiceName},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}},
				},
				FilterFunc: filterRegistryService,
			},
			{
				Name:       registryEndpointsSnapName,
				ApiVersion: "discovery.k8s.io/v1",
				Kind:       "EndpointSlice",
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{discoveryv1.LabelServiceName: registry_const.ServiceName},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}},
				},
				FilterFunc: filterReadyStorageEndpoints,
			},
			{
				// Makes InClusterAccess sticky.
				Name:       inClusterAccessSnapName,
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{registry_const.InClusterAccessSecretName},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}},
				},
				FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
					return obj.GetName(), nil
				},
			},
		},
	},
	dependency.WithExternalDependencies(handleInClusterAccess),
)

// registryService is what the Service named after the key is.
type registryService struct {
	Name string `json:"name"`

	// UID and ResourceVersion: the object the snapshot saw, the only one deleted when replacing it.
	UID             string `json:"uid"`
	ResourceVersion string `json:"resourceVersion"`

	Headless bool `json:"headless"`

	// Selectorless: with the cache the registry controller writes its slice (service_endpoint.go), and
	// only that slice counts; the ones Kubernetes wrote for a Service of the same name before do not.
	Selectorless bool `json:"selectorless"`

	// Storage: it is the store's, by its label: with the cache it has no selector, the controller
	// points it at the leader. Legacy: it selects the previous implementation's proxy.
	Storage bool `json:"storage"`
	Legacy  bool `json:"legacy"`

	// Routes: it has the key's port.
	Routes bool `json:"routes"`
}

func filterRegistryService(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var service v1core.Service
	if err := sdk.FromUnstructured(obj, &service); err != nil {
		return nil, fmt.Errorf("converting the registry service: %w", err)
	}

	result := registryService{
		Name:            service.Name,
		UID:             string(service.UID),
		ResourceVersion: service.ResourceVersion,
		Headless:        service.Spec.ClusterIP == v1core.ClusterIPNone,
		Selectorless:    len(service.Spec.Selector) == 0,
		Legacy:          service.Spec.Selector["component"] == legacyServiceSelector,
	}
	result.Storage = !result.Legacy && service.Labels["app"] == storageSelector
	for _, port := range service.Spec.Ports {
		if port.Port == registry_const.ServicePort {
			result.Routes = true
		}
	}
	return result, nil
}

// storageSlice is what a slice of the Service says: how many ready endpoints serve the store's port, and
// who manages it — the registry controller for the selectorless Service, Kubernetes for one with a selector.
type storageSlice struct {
	Ready     int    `json:"ready"`
	ManagedBy string `json:"managedBy"`
}

// filterReadyStorageEndpoints counts the ready endpoints of a slice that serve the store's port.
func filterReadyStorageEndpoints(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var slice discoveryv1.EndpointSlice
	if err := sdk.FromUnstructured(obj, &slice); err != nil {
		return nil, fmt.Errorf("converting the registry endpoints: %w", err)
	}

	result := storageSlice{ManagedBy: slice.Labels[discoveryv1.LabelManagedBy]}
	serves := false
	for _, port := range slice.Ports {
		if port.Port != nil && *port.Port == registry_const.StoragePort {
			serves = true
		}
	}
	if !serves {
		return result, nil
	}

	for _, endpoint := range slice.Endpoints {
		if endpoint.Conditions.Ready == nil || *endpoint.Conditions.Ready {
			result.Ready++
		}
	}
	return result, nil
}

// inClusterAccessState is what the decision is made from.
type inClusterAccessState struct {
	// Service is the Service named after the key, nil when there is none.
	Service *registryService

	// Ready is how many ready endpoints behind it are on the store's port.
	Ready int

	// Published: the access Secret exists.
	Published bool
}

// deleteHeadless answers whether the Service is the store's headless one, to be replaced.
func (s inClusterAccessState) deleteHeadless() bool {
	return s.Service != nil && s.Service.Headless && s.Service.Storage
}

// legacyService answers whether the name is still the previous implementation's.
func (s inClusterAccessState) legacyService() bool {
	return s.Service != nil && s.Service.Legacy
}

// inClusterAccess answers whether pods may be given the account and authority for the key: the routable
// Service exists and has a ready endpoint. Without the cache that is a ready proxy pod; with it, the
// replica the controller names for the nodes, which it marks ready whatever that replica's state
// (service_endpoint.go in the controller). Sticky, as the image address: withdrawn, it would converge
// every module twice for a replica restarting.
func (s inClusterAccessState) inClusterAccess() bool {
	if s.Published {
		return true
	}
	return s.Service != nil && !s.Service.Headless && s.Service.Storage && s.Service.Routes && s.Ready > 0
}

func handleInClusterAccess(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	state, err := readInClusterAccessState(input)
	if err != nil {
		return err
	}

	if state.deleteHeadless() {
		client, err := dc.GetK8sClient()
		if err != nil {
			return fmt.Errorf("getting the kubernetes client: %w", err)
		}
		deleted, err := deleteHeadlessService(ctx, client.CoreV1(), *state.Service)
		if err != nil {
			return err
		}
		if deleted {
			input.Logger.Info("replaced the store's headless Service with a routable one, so pods reach the key",
				"service", registry_const.ServiceName)
		}
	}

	values := accessor(input)
	current := values.Get()
	current.LegacyService = state.legacyService()
	current.InClusterAccess = state.inClusterAccess()
	values.Set(current)
	return nil
}

// deleteHeadlessService deletes exactly the object the snapshot saw: a snapshot behind the cluster must not
// delete the routable Service Helm has created since. Gone or replaced is not an error.
func deleteHeadlessService(ctx context.Context, core corev1client.CoreV1Interface, service registryService) (bool, error) {
	uid := k8stypes.UID(service.UID)
	err := core.Services("d8-system").Delete(ctx, registry_const.ServiceName, metav1.DeleteOptions{
		Preconditions: &metav1.Preconditions{UID: &uid, ResourceVersion: &service.ResourceVersion},
	})
	switch {
	case err == nil:
		return true, nil
	case apierrors.IsNotFound(err), apierrors.IsConflict(err):
		return false, nil
	default:
		return false, fmt.Errorf("deleting the headless Service %s: %w", registry_const.ServiceName, err)
	}
}

// readyFor counts the ready endpoints of the slices that belong to the Service as it is now: the
// controller's for the selectorless one, Kubernetes' for one with a selector. A slice of the Service this
// one replaced, which the garbage collector has not removed yet, does not open the key to pods.
func readyFor(service *registryService, slices []storageSlice) int {
	if service == nil {
		return 0
	}
	managedBy := endpointSliceControllerName
	if service.Selectorless {
		managedBy = registryControllerSliceManager
	}
	ready := 0
	for _, slice := range slices {
		if slice.ManagedBy == managedBy {
			ready += slice.Ready
		}
	}
	return ready
}

const (
	// registryControllerSliceManager labels the slice the registry controller writes (pushManagedBy there).
	registryControllerSliceManager = "registry-controller.deckhouse.io"
	endpointSliceControllerName    = "endpointslice-controller.k8s.io"
)

func readInClusterAccessState(input *go_hook.HookInput) (inClusterAccessState, error) {
	result := inClusterAccessState{}

	services, err := helpers.SnapshotToList[registryService](input, registryServiceSnapName)
	if err != nil {
		return result, fmt.Errorf("reading the registry service: %w", err)
	}
	// By name as well as by the binding's selector, which not every client applies.
	for i := range services {
		if services[i].Name == registry_const.ServiceName {
			result.Service = &services[i]
		}
	}

	slices, err := helpers.SnapshotToList[storageSlice](input, registryEndpointsSnapName)
	if err != nil {
		return result, fmt.Errorf("reading the registry endpoints: %w", err)
	}
	result.Ready = readyFor(result.Service, slices)

	published, err := helpers.SnapshotToList[string](input, inClusterAccessSnapName)
	if err != nil {
		return result, fmt.Errorf("reading the in-cluster access: %w", err)
	}
	for _, name := range published {
		if name == registry_const.InClusterAccessSecretName {
			result.Published = true
		}
	}

	return result, nil
}
