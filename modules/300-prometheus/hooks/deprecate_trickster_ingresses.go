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
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/prometheus/deprecate_trickster_ingresses",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "helm_releases",
			Crontab: "0 * * * *", // every hour
		},
	},
}, dependency.WithExternalDependencies(handleTricksterIngresses))

func handleTricksterIngresses(input *go_hook.HookInput, dc dependency.Container) error {
	client, err := dc.GetK8sClient()
	if err != nil {
		return err
	}
	ctx := context.Background()
	return exposeTricksterIngressMetrics(ctx, input, client)
}

const (
	ingressTargetBackendService = "trickster"
	ingressListIterationStep    = int64(30)
)

func exposeTricksterIngressMetrics(ctx context.Context, input *go_hook.HookInput, kubeClient k8s.Client) error {
	var next string

	for {
		ingressList, err := kubeClient.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{
			Limit:    ingressListIterationStep,
			Continue: next,
		})
		if err != nil {
			return err
		}

		for _, ingress := range ingressList.Items {
			for _, rule := range ingress.Spec.Rules {
				if rule.IngressRuleValue.HTTP == nil {
					continue
				}
				for _, path := range rule.IngressRuleValue.HTTP.Paths {
					if path.Backend.Service == nil {
						continue
					}
					if strings.Contains(path.Backend.Service.Name, ingressTargetBackendService) {
						input.MetricsCollector.Set("d8_trickster_deprecated_ingresses",
							1, map[string]string{
								"ingress":   sanitizeLabelName(ingress.Name),
								"namespace": sanitizeLabelName(ingress.Namespace),
								"backend":   sanitizeLabelName(path.Backend.Service.Name),
							},
						)
					}
				}
			}
		}

		if ingressList.GetContinue() == "" {
			break
		}

		next = ingressList.Continue
	}

	return nil
}
