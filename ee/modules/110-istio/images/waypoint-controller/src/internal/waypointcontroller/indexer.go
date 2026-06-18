/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package waypointcontroller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const ownerUIDFieldIndex = ".metadata.ownerReferences.uid"

func ensureOwnerUIDIndex(ctx context.Context, mgr client.FieldIndexer, vpaEnabled bool) error {
	indexedObjects := []client.Object{
		&appsv1.Deployment{},
		&corev1.Service{},
		&corev1.ServiceAccount{},
		&policyv1.PodDisruptionBudget{},
		&autoscalingv2.HorizontalPodAutoscaler{},
		&gatewayv1.Gateway{},
	}
	if vpaEnabled {
		indexedObjects = append(indexedObjects, &vpav1.VerticalPodAutoscaler{})
	}

	for _, obj := range indexedObjects {
		if err := mgr.IndexField(ctx, obj, ownerUIDFieldIndex, ownerUIDs); err != nil {
			return err
		}
	}

	return nil
}

func ownerUIDs(obj client.Object) []string {
	ownerRefs := obj.GetOwnerReferences()
	if len(ownerRefs) == 0 {
		return nil
	}

	uids := make([]string, 0, len(ownerRefs))
	for _, ownerRef := range ownerRefs {
		if ownerRef.UID == "" {
			continue
		}

		uids = append(uids, string(ownerRef.UID))
	}

	return uids
}
