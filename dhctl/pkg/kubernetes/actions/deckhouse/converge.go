package deckhouse

import (
	"context"
	"encoding/json"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func ConvergeDeckhouseConfiguration(ctx context.Context, kubeCl *client.KubernetesClient, clusterUUID string, clusterConfig []byte, providerClusterConfig []byte) error {
	tasks := []actions.ManifestTask{
		{
			Name:     `Secret "d8-cluster-configuration"`,
			Manifest: func() interface{} { return manifests.SecretWithClusterConfig(clusterConfig) },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("kube-system").Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("kube-system").Update(ctx, manifest.(*apiv1.Secret), metav1.UpdateOptions{})
				return err
			},
		},
		{
			Name: `Secret "d8-provider-cluster-configuration"`,
			Manifest: func() interface{} {
				return manifests.SecretWithProviderClusterConfig(
					providerClusterConfig, nil,
				)
			},
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("kube-system").Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				data, err := json.Marshal(manifest.(*apiv1.Secret))
				if err != nil {
					return err
				}
				_, err = kubeCl.CoreV1().Secrets("kube-system").Patch(ctx,
					"d8-provider-cluster-configuration",
					types.MergePatchType,
					data,
					metav1.PatchOptions{},
				)
				return err
			},
		},
		{
			Name: `ConfigMap "d8-cluster-uuid"`,
			Manifest: func() interface{} {
				return manifests.ClusterUUIDConfigMap(clusterUUID)
			},
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().ConfigMaps(manifests.ClusterUUIDCmNamespace).Create(ctx, manifest.(*apiv1.ConfigMap), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().ConfigMaps(manifests.ClusterUUIDCmNamespace).Update(ctx, manifest.(*apiv1.ConfigMap), metav1.UpdateOptions{})
				return err
			},
		},
	}

	return log.Process("default", "Converge deckhouse configuration", func() error {
		for _, task := range tasks {
			err := task.CreateOrUpdate()
			if err != nil {
				return err
			}
		}
		return nil
	})
}
