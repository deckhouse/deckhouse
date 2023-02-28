/*
Copyright 2023 Flant JSC

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

package probe

import (
	"time"

	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/checker"
)

func initExtensions(access kubernetes.Access, preflight checker.Doer) []runnerConfig {
	const (
		groupExtensions     = "extensions"
		controlPlaneTimeout = 5 * time.Second
	)

	controlPlanePinger := checker.DoOrUnknown(controlPlaneTimeout, preflight)

	return []runnerConfig{
		{
			group:  groupExtensions,
			probe:  "cluster-scaling",
			check:  "bashible-apiserver",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:           access,
				Timeout:          5 * time.Second,
				Namespace:        "d8-cloud-instance-manager",
				LabelSelector:    "app=bashible-apiserver",
				PreflightChecker: controlPlanePinger,
			},
		}, {
			group:  groupExtensions,
			probe:  "cluster-scaling",
			check:  "machine-controller-manager",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:           access,
				Timeout:          5 * time.Second,
				Namespace:        "d8-cloud-instance-manager",
				LabelSelector:    "app=machine-controller-manager",
				PreflightChecker: controlPlanePinger,
			},
		}, {
			group:  groupExtensions,
			probe:  "cluster-scaling",
			check:  "cloud-controller-manager",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:           access,
				Timeout:          5 * time.Second,
				Namespace:        access.CloudControllerManagerNamespace(),
				LabelSelector:    "app=cloud-controller-manager",
				PreflightChecker: controlPlanePinger,
			},
		}, {
			group:  groupExtensions,
			probe:  "cluster-autoscaler",
			check:  "cluster-autoscaler",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:           access,
				Timeout:          5 * time.Second,
				Namespace:        "d8-cloud-instance-manager",
				LabelSelector:    "app=cluster-autoscaler",
				PreflightChecker: controlPlanePinger,
			},
		}, {
			group:  groupExtensions,
			probe:  "grafana",
			check:  "pod",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:           access,
				Timeout:          5 * time.Second,
				Namespace:        "d8-monitoring",
				LabelSelector:    "app=grafana",
				PreflightChecker: controlPlanePinger,
			},
		}, {
			group:  groupExtensions,
			probe:  "openvpn",
			check:  "pod",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:           access,
				Timeout:          5 * time.Second,
				Namespace:        "d8-openvpn",
				LabelSelector:    "app=openvpn",
				PreflightChecker: controlPlanePinger,
			},
		}, {
			group:  groupExtensions,
			probe:  "prometheus-longterm",
			check:  "pod",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:           access,
				Timeout:          5 * time.Second,
				Namespace:        "d8-monitoring",
				LabelSelector:    "prometheus=longterm",
				PreflightChecker: controlPlanePinger,
			},
		}, {
			group:  groupExtensions,
			probe:  "prometheus-longterm",
			check:  "api",
			period: 10 * time.Second,
			config: checker.PrometheusApiAvailable{
				Access:   access,
				Timeout:  5 * time.Second,
				Endpoint: "https://prometheus-longterm.d8-monitoring:9090/api/v1/query?query=vector(1)",
			},
		}, {
			group:  groupExtensions,
			probe:  "dashboard",
			check:  "pod",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:           access,
				Timeout:          5 * time.Second,
				Namespace:        "d8-dashboard",
				LabelSelector:    "app=dashboard",
				PreflightChecker: controlPlanePinger,
			},
		}, {
			group:  groupExtensions,
			probe:  "dex",
			check:  "pod",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:           access,
				Timeout:          5 * time.Second,
				Namespace:        "d8-user-authn",
				LabelSelector:    "app=dex",
				PreflightChecker: controlPlanePinger,
			},
		}, {
			group:  groupExtensions,
			probe:  "dex",
			check:  "keys",
			period: 10 * time.Second,
			config: checker.DexApiAvailable{
				Access:   access,
				Timeout:  5 * time.Second,
				Endpoint: "https://dex.d8-user-authn/keys",
			},
		},
	}
}
