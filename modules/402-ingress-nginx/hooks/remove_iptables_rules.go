/*
Copyright 2024 Flant JSC
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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	iptablesRemoveJobName = "failover-iptables-remove-rules-job"
	moduleName            = "ingress-nginx"
	heritageDeckhouse     = "deckhouse"
	// systemNamespace       = "d8-system"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterDeleteHelm: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(removeIptablesRules))

func removeIptablesRules(input *go_hook.HookInput, dc dependency.Container) (err error) {
	input.Logger.Info("Remove iptables rule for proxy-failover...")
	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	kubeClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: ingressNamespace,
		},
	}, v1.CreateOptions{})

	registry := input.Values.Get("global.modulesImages.registry.base").String()
	digest := input.Values.Get("global.modulesImages.digests.ingressNginx.proxyFailoverIptables").String()
	job := generateJob(registry, digest)
	_, err = kubeClient.BatchV1().Jobs(ingressNamespace).Create(context.Background(), job, v1.CreateOptions{})
	if err != nil {
		input.Logger.Error("Failed to run job for removing iptables rules", "error", err)
		return err
	}

	kubeClient.CoreV1().Namespaces().Delete(context.Background(), ingressNamespace, v1.DeleteOptions{})
	// input.Logger.Info("Remove iptables remove rules job from cluster...")
	// input.PatchCollector.Delete("batch/v1", "Job", systemNamespace, iptablesRemoveJobName)

	return nil
}

func generateJob(registry, digest string) *batchv1.Job {
	return &batchv1.Job{
		TypeMeta: v1.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      iptablesRemoveJobName,
			Namespace: ingressNamespace,
			Labels: map[string]string{
				"name":     iptablesRemoveJobName,
				"heritage": heritageDeckhouse,
				"module":   moduleName,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To(int32(1)),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ImagePullSecrets:              []corev1.LocalObjectReference{{Name: "deckhouse-registry"}},
					HostNetwork:                   true,
					DNSPolicy:                     corev1.DNSClusterFirstWithHostNet,
					TerminationGracePeriodSeconds: ptr.To(int64(300)),
					Containers: []corev1.Container{
						{
							Name:  "iptables-remove-rules",
							Image: fmt.Sprintf("%s@%s", registry, digest),
							Command: []string{
								"/failover",
								"remove",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "xtables-lock",
									ReadOnly:  false,
									MountPath: "/run/xtables.lock",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory:           resource.MustParse("20Mi"),
									corev1.ResourceCPU:              resource.MustParse("10m"),
									corev1.ResourceEphemeralStorage: resource.MustParse("50Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot: ptr.To(false),
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"NET_RAW", "NET_ADMIN"},
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "xtables-lock",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/run/xtables.lock",
									Type: ptr.To(corev1.HostPathFileOrCreate),
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
}
