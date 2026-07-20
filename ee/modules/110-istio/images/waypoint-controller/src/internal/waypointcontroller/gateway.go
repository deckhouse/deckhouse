/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package waypointcontroller

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	networkv1alpha1 "waypoint-controller/pkg/apis/network.deckhouse.io/v1alpha1"
)

func (r *WaypointController) ensureWaypointGateway(ctx context.Context, instance *networkv1alpha1.WaypointInstance) error {
	hostnameAddressType := gatewayv1.HostnameAddressType
	namespacesFromSame := gatewayv1.NamespacesFromSame

	allowedRouteNamespaces, err := buildAllowedRouteNamespaces(instance, &namespacesFromSame)
	if err != nil {
		return err
	}

	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceBaseName(instance.Name),
			Namespace: instance.Namespace,
		},
	}

	_, err = controllerutil.CreateOrPatch(ctx, r.Client, gateway, func() error {
		gateway.Labels = make(map[string]string)
		for k, v := range instanceLabels(instance) {
			gateway.Labels[k] = v
		}
		for k, v := range istioLabels(instance, r.istioRevision, r.istioNetworkName) {
			gateway.Labels[k] = v
		}
		gateway.Labels["gateway.networking.k8s.io/gateway-name"] = resourceBaseName(instance.Name)
		gateway.Labels[WaypointComponentLabelKey] = "gateway"

		addrValue := resourceBaseName(instance.Name) + "." + instance.Namespace + ".svc." + r.clusterDomain

		gateway.Spec = gatewayv1.GatewaySpec{
			GatewayClassName: gatewayv1.ObjectName("istio-waypoint"),
			Addresses: []gatewayv1.GatewaySpecAddress{
				{
					Type:  &hostnameAddressType,
					Value: addrValue,
				},
			},
			Listeners: []gatewayv1.Listener{
				{
					Name:     "mesh",
					Port:     gatewayv1.PortNumber(15008),
					Protocol: gatewayv1.ProtocolType("HBONE"),
					AllowedRoutes: &gatewayv1.AllowedRoutes{
						Namespaces: allowedRouteNamespaces,
					},
				},
			},
		}

		if err := controllerutil.SetControllerReference(instance, gateway, r.scheme); err != nil {
			return err
		}

		klog.V(4).InfoS(
			"Gateway spec set",
			"name", gateway.Name,
			"namespace", gateway.Namespace,
		)

		return nil
	})

	return err
}

func buildAllowedRouteNamespaces(instance *networkv1alpha1.WaypointInstance, defaultFrom *gatewayv1.FromNamespaces) (*gatewayv1.RouteNamespaces, error) {
	from := defaultFrom
	var selector *metav1.LabelSelector

	if cfg := instance.Spec.AllowedRoutes; cfg != nil {
		if ns := cfg.Namespaces; ns != nil {
			if ns.From != nil {
				f := gatewayv1.FromNamespaces(*ns.From)
				from = &f
			}
			selector = ns.Selector
		}
	}

	if from != nil && *from == gatewayv1.NamespacesFromSelector && selector == nil {
		return nil, fmt.Errorf("allowedRoutes.namespaces.selector is required when from is Selector")
	}

	return &gatewayv1.RouteNamespaces{
		From:     from,
		Selector: selector,
	}, nil
}
