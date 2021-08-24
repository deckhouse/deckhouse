/*
Copyright 2021 Flant CJSC

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

	"d8.io/upmeter/pkg/probe/checker"
)

func initSynthetic() []runnerConfig {
	const (
		groupName = "synthetic"
	)

	return []runnerConfig{
		{
			group:  groupName,
			probe:  "access",
			check:  "_",
			period: 5 * time.Second,
			config: checker.SmokeMiniAvailable{
				Path:        "/",
				DnsTimeout:  2 * time.Second,
				HttpTimeout: 2 * time.Second,
			},
		}, {
			group:  groupName,
			probe:  "dns",
			check:  "smoke",
			period: 200 * time.Millisecond,
			config: checker.SmokeMiniAvailable{
				Path:        "/",
				DnsTimeout:  2 * time.Second,
				HttpTimeout: 2 * time.Second,
			},
		}, {
			group:  groupName,
			probe:  "dns",
			check:  "internal",
			period: 200 * time.Millisecond,
			config: checker.DnsAvailable{
				Domain:     "kubernetes.default",
				DnsTimeout: 2 * time.Second,
			},
		}, {
			group:  groupName,
			probe:  "neighbor",
			check:  "_",
			period: 5 * time.Second,
			config: checker.SmokeMiniAvailable{
				Path:        "/neighbor",
				DnsTimeout:  2 * time.Second,
				HttpTimeout: 4 * time.Second,
			},
		}, {
			group:  groupName,
			probe:  "neighbor-via-service",
			check:  "_",
			period: 5 * time.Second,
			config: checker.SmokeMiniAvailable{
				Path:        "/neighbor-via-service",
				DnsTimeout:  2 * time.Second,
				HttpTimeout: 4 * time.Second,
			},
		},
	}
}
