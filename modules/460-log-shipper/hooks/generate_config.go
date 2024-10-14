/*
Copyright 2021 Flant JSC

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

package hooks

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/composer"
)

func filterPodLoggingConfig(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var src v1alpha1.PodLoggingConfig

	err := sdk.FromUnstructured(obj, &src)
	if err != nil {
		return nil, err
	}
	return src, nil
}

func filterClusterLoggingConfig(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var src v1alpha1.ClusterLoggingConfig

	err := sdk.FromUnstructured(obj, &src)
	if err != nil {
		return nil, err
	}
	return src, nil
}

func filterClusterLogDestination(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var dst v1alpha1.ClusterLogDestination

	err := sdk.FromUnstructured(obj, &dst)
	if err != nil {
		return nil, err
	}
	return dst, nil
}

func filterNamespaceName(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var namespace corev1.Namespace

	err := sdk.FromUnstructured(obj, &namespace)
	if err != nil {
		return nil, err
	}
	return namespace.GetName(), nil
}

func filterLogShipperTokenSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := new(corev1.Secret)

	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	return string(secret.Data["token"]), nil
}

func filterLokiEndpoints(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	endpoints := new(corev1.Endpoints)

	err := sdk.FromUnstructured(obj, endpoints)
	if err != nil {
		return nil, err
	}

	var lokiEndpoint endpoint

	for _, subset := range endpoints.Subsets {
		for _, p := range subset.Ports {
			if p.Name == "loki" {
				lokiEndpoint.port = strconv.FormatInt(int64(p.Port), 10)

				break
			}
		}

		for _, address := range subset.Addresses {
			lokiEndpoint.addresses = append(lokiEndpoint.addresses, address.IP)
		}
	}

	return lokiEndpoint, nil
}

type endpoint struct {
	addresses []string
	port      string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/log-shipper/generate_config",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "namespaced_log_source",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "PodLoggingConfig",
			FilterFunc: filterPodLoggingConfig,
		},
		{
			Name:       "cluster_log_source",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ClusterLoggingConfig",
			FilterFunc: filterClusterLoggingConfig,
		},
		{
			Name:       "cluster_log_destination",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ClusterLogDestination",
			FilterFunc: filterClusterLogDestination,
		},
		{
			Name:       "namespace",
			ApiVersion: "v1",
			Kind:       "Namespace",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-log-shipper"},
			},
			FilterFunc: filterNamespaceName,
		},
		{
			Name:       "token",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{
					"d8-log-shipper",
				}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				"log-shipper-token",
			}},
			FilterFunc: filterLogShipperTokenSecret,
		},
		{
			Name:       "loki_endpoint",
			ApiVersion: "v1",
			Kind:       "Endpoints",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{
					"d8-monitoring",
				}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				"loki",
			}},
			FilterFunc: filterLokiEndpoints,
		},
	},
}, generateConfig)

func generateConfig(input *go_hook.HookInput) error {
	if len(input.Snapshots["namespace"]) < 1 {
		// there is no namespace to manipulate the config map, the hook will create it later on afterHelm
		input.Values.Set("logShipper.internal.activated", false)
		return nil
	}

	destSnap := input.Snapshots["cluster_log_destination"]
	tokenSnap := input.Snapshots["token"]

	var token string

	if len(tokenSnap) > 0 {
		token = tokenSnap[0].(string)
	}

	lokiEndpointSnap := input.Snapshots["loki_endpoint"]

	var lokiEndpoint endpoint

	if len(lokiEndpointSnap) > 0 {
		lokiEndpoint = lokiEndpointSnap[0].(endpoint)
	}

	clusterDomain := input.Values.Get("global.discovery.clusterDomain").String()

	var destinations []v1alpha1.ClusterLogDestination

	for _, destination := range destSnap {
		dest := destination.(v1alpha1.ClusterLogDestination)

		if dest.Spec.Type != "Loki" || token == "" {
			destinations = append(destinations, dest)

			continue
		}

		d, err := migrateClusterLogDestinationLoki(dest, clusterDomain, lokiEndpoint, token)
		if err != nil {
			return errors.Wrap(err, "failed to migrate cluster log destination loki")
		}

		destinations = append(destinations, *d)
	}

	configContent, err := composer.FromInput(input, destinations).Do()
	if err != nil {
		return err
	}

	activated := len(configContent) != 0
	input.Values.Set("logShipper.internal.activated", activated)

	if !activated {
		input.PatchCollector.Delete(
			"v1", "Secret", "d8-log-shipper", "d8-log-shipper-config",
			object_patch.InBackground())
		return nil
	}

	secret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "d8-log-shipper-config",
			Namespace: "d8-log-shipper",
			Labels: map[string]string{
				"heritage": "deckhouse",
				"module":   "log-shipper",
			},
		},
		Data: map[string][]byte{"vector.json": configContent},
	}
	input.PatchCollector.Create(secret, object_patch.UpdateIfExists())

	event := &eventsv1.Event{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Event",
			APIVersion: "events.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    secret.Namespace,
			GenerateName: secret.Name + "-",
		},
		Regarding: corev1.ObjectReference{
			Kind:       secret.Kind,
			Name:       secret.Name,
			Namespace:  secret.Namespace,
			APIVersion: secret.APIVersion,
		},
		Reason:              "LogShipperConfigCreateUpdate",
		Note:                "Config file has been created or updated.",
		Action:              "Create/Update",
		Type:                corev1.EventTypeNormal,
		EventTime:           metav1.MicroTime{Time: time.Now()},
		ReportingInstance:   "deckhouse",
		ReportingController: "deckhouse",
	}
	input.PatchCollector.Create(event)

	return nil
}

// migrateClusterLogDestinationLoki migrates ClusterLogDestination pointing to d8-loki.
// There may be ClusterLogDestination resources in the cluster pointing to d8-loki besides the one we create in the Deckhouse loki module.
// We also have handleClusterLogDestinationD8Loki function which is notifying users that they should migrate ClusterLogDestination resources manually.
func migrateClusterLogDestinationLoki(destination v1alpha1.ClusterLogDestination, clusterDomain string, endpoint endpoint, token string) (*v1alpha1.ClusterLogDestination, error) {
	endpointURL, err := url.Parse(destination.Spec.Loki.Endpoint)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse loki endpoint '%s'", destination.Spec.Loki.Endpoint)
	}

	if !matchLokiEndpoint(endpointURL.Host, clusterDomain, endpoint) {
		return &destination, nil
	}

	endpointURL.Scheme = "https"

	destination.Spec.Loki.Endpoint = endpointURL.String()

	destination.Spec.Loki.Auth.Strategy = "Bearer"
	destination.Spec.Loki.Auth.Token = token

	verifyHostname := false
	verifyCertificate := false

	destination.Spec.Loki.TLS.VerifyHostname = &verifyHostname
	destination.Spec.Loki.TLS.VerifyCertificate = &verifyCertificate

	return &destination, nil
}

func matchLokiEndpoint(hostPort string, clusterDomain string, endpoint endpoint) bool {
	if hostPort == net.JoinHostPort("loki.d8-monitoring", endpoint.port) ||
		hostPort == net.JoinHostPort("loki.d8-monitoring.", endpoint.port) ||
		hostPort == net.JoinHostPort(fmt.Sprintf("loki.d8-monitoring.svc.%s", clusterDomain), endpoint.port) ||
		hostPort == net.JoinHostPort(fmt.Sprintf("loki.d8-monitoring.svc.%s.", clusterDomain), endpoint.port) {
		return true
	}

	for _, address := range endpoint.addresses {
		if net.JoinHostPort(address, endpoint.port) == hostPort {
			return true
		}
	}

	return false
}
