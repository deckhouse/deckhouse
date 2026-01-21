package controller

import (
	"context"
	"fmt"
	"update-observer/cluster"
	"update-observer/common"

	"go.yaml.in/yaml/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *reconciler) getConfigMap(ctx context.Context) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	err := r.client.Get(ctx, client.ObjectKey{
		Name:      common.ConfigMapName,
		Namespace: common.KubeSystemNamespace,
	}, cm)

	if client.IgnoreNotFound(err) != nil {
		return nil, err
	}

	if err != nil {
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ConfigMapName,
				Namespace: common.KubeSystemNamespace,
				Labels: map[string]string{
					common.HeritageLabelKey: common.DeckhouseLabel,
				},
			},
			Data: map[string]string{},
		}
	}

	return cm, nil
}

func (r *reconciler) touchConfigMap(ctx context.Context, configMap *corev1.ConfigMap, clusterState *cluster.State) error {
	if configMap.Data == nil {
		configMap.Data = map[string]string{}
	}

	if clusterState != nil {
		specBytes, err := yaml.Marshal(clusterState.Spec)
		if err != nil {
			return fmt.Errorf("failed to marshal Spec: %w", err)
		}
		configMap.Data["spec"] = string(specBytes)

		statusBytes, err := yaml.Marshal(clusterState.Status)
		if err != nil {
			return fmt.Errorf("failed to marshal Status: %w", err)
		}
		configMap.Data["status"] = string(statusBytes)
	} else {
		clusterState = &cluster.State{
			Status: cluster.Status{
				Phase: cluster.ClusterUnknown,
			},
		}

		statusBytes, err := yaml.Marshal(clusterState.Status)
		if err != nil {
			return fmt.Errorf("failed to marshal Status: %w", err)
		}
		configMap.Data["status"] = string(statusBytes)
	}

	if configMap.ResourceVersion == "" {
		if err := r.client.Create(ctx, configMap); err != nil {
			return fmt.Errorf("failed to create configMap: %w", err)
		}
	} else {
		if err := r.client.Update(ctx, configMap); err != nil {
			return fmt.Errorf("failed to update configMap: %w", err)
		}
	}

	return nil
}
