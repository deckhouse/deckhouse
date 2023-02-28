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

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/checker"
)

func initNginx(access kubernetes.Access, preflight checker.Doer, names []string) []runnerConfig {
	const (
		groupNginx          = "nginx"
		controlPlaneTimeout = 5 * time.Second
	)

	controlPlanePinger := checker.DoOrUnknown(controlPlaneTimeout, preflight)

	configs := []runnerConfig{}

	for _, controllerName := range names {
		configs = append(configs,
			nginxPodChecker(access, groupNginx, controlPlanePinger, controllerName),
			// TBD: check default backend
		)
	}
	return configs
}

func nginxPodChecker(access kubernetes.Access, groupNginx string, controlPlanePinger check.Checker, controllerName string) runnerConfig {
	return runnerConfig{
		group:  groupNginx,
		probe:  controllerName,
		check:  "pod",
		period: 10 * time.Second,
		config: checker.AtLeastOnePodReady{
			Access:           access,
			Timeout:          5 * time.Second,
			Namespace:        "d8-ingress-nginx",
			LabelSelector:    "app=controller,name=" + controllerName,
			PreflightChecker: controlPlanePinger,
		},
	}
}
