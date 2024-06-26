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
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

// TODO: Remove this migration hook after deprecating ingress controllers of 1.10< versions

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/ingress-nginx/deprecated_geoip_version",
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "0 * * * *"},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "controller",
			ApiVersion:                   "deckhouse.io/v1",
			Kind:                         "IngressNginxController",
			ExecuteHookOnSynchronization: pointer.Bool(true),
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc:                   inletHostWithFailoverFilter,
		},
	},
}, dependency.WithExternalDependencies(searchForDeprecatedGeoip))

var (
	metricsGroup            = "d8_deprecated_geoip_version"
	ingressNamespace        = "d8-ingress-nginx"
	ingressAnnotationPrefix = "nginx.ingress.kubernetes.io/"

	eolVersion = semver.MustParse("1.10")

	geoipVarsRegexp = regexp.MustCompile(`\$geoip_(country_(code3|code|name)|area_code|city_continent_code|city_country_(code3|code|name)|dma_code|latitude|longitude|region|region_name|city|postal_code|org)([^_a-zA-Z0-9]|$)+`)

	// objectBatchSize - how many ingress objects to list from k8s at once
	objectBatchSize = int64(30)
	// fetchSecretsInterval pause between fetching ingress objects from apiserver
	fetchSecretsInterval = 1 * time.Second
)

type controllerVersion struct {
	Name    string
	Version string
}

func inletHostWithFailoverFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	controller := controllerVersion{}
	controller.Name = obj.GetName()
	controllerVersion, ok, err := unstructured.NestedString(obj.Object, "spec", "controllerVersion")
	if err != nil {
		return nil, fmt.Errorf("couldn't get controllerVersion field from ingress controller %s: %w", controller.Name, err)
	}

	if ok {
		controller.Version = controllerVersion
	}
	return controller, nil
}

func searchForDeprecatedGeoip(input *go_hook.HookInput, dc dependency.Container) (err error) {
	kubeClient := dc.MustGetK8sClient()
	input.MetricsCollector.Expire(metricsGroup)
	defaultVersion, err := semver.NewVersion(input.Values.Get("ingressNginx.defaultControllerVersion").String())
	if err != nil {
		return fmt.Errorf("couldn't parse defaultControllerVersion as semver: %w", err)
	}

	controllers := input.Snapshots["controller"]

	// check ingressnginxcontrollers' configs
	for _, c := range controllers {
		controller := c.(controllerVersion)
		var cVer *semver.Version

		if len(controller.Version) == 0 {
			cVer = defaultVersion
		} else {
			cVer, err = semver.NewVersion(controller.Version)
			if err != nil {
				return fmt.Errorf("couldn't parse controller's version as semver: %w", err)
			}
		}

		if cVer.Compare(eolVersion) >= 0 {
			continue
		}

		err := checkControllerConfigMaps(kubeClient, input.MetricsCollector, controller.Name)
		if err != nil {
			return fmt.Errorf("couldn't check %s controller's configMaps: %w", controller.Name, err)
		}
	}

	// check ingress objects' annotations
	var next string

	for {
		ingressList, err := kubeClient.NetworkingV1().Ingresses("").List(context.Background(), metav1.ListOptions{
			Limit:                objectBatchSize,
			Continue:             next,
			ResourceVersion:      "0",
			ResourceVersionMatch: metav1.ResourceVersionMatchNotOlderThan,
		})
		if err != nil {
			return fmt.Errorf("couldn't list ingresses: %w", err)
		}
		ingressList.GetRemainingItemCount()

		for _, ingress := range ingressList.Items {
			annotations := ingress.Annotations
			if len(annotations) == 0 {
				continue
			}

			for k, v := range annotations {
				if strings.HasPrefix(k, ingressAnnotationPrefix) && geoipVarsRegexp.MatchString(v) {
					input.MetricsCollector.Set(metricsGroup, 1, map[string]string{"kind": "Ingress", "resource_namespace": ingress.Namespace, "resource_name": ingress.Name, "resource_key": k}, metrics.WithGroup(metricsGroup))
				}
			}
		}

		if ingressList.Continue == "" {
			break
		}

		next = ingressList.Continue
		time.Sleep(fetchSecretsInterval)
	}

	return nil
}

func checkControllerConfigMaps(client k8s.Client, collector go_hook.MetricsCollector, controllerName string) error {
	configMaps := []string{fmt.Sprintf("%s-config", controllerName), fmt.Sprintf("%s-custom-headers", controllerName)}

	for _, cmName := range configMaps {
		configMap, err := client.CoreV1().ConfigMaps(ingressNamespace).Get(context.Background(), cmName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("couldn't get %s controller's configmap %s: %w", controllerName, cmName, err)
		}
		for k, v := range configMap.Data {
			if geoipVarsRegexp.MatchString(v) {
				collector.Set(metricsGroup, 1, map[string]string{"kind": "IngressNginxController", "resource_namespace": "", "resource_name": controllerName, "resource_key": k}, metrics.WithGroup(metricsGroup))
			}
		}
	}

	return nil
}
