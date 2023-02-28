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

	"github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/checker"
)

func initSynthetic(access kubernetes.Access, logger *logrus.Logger) []runnerConfig {
	const (
		groupSynthetic = "synthetic"
	)

	entry := logger.WithField("group", groupSynthetic)

	return []runnerConfig{
		{
			group:  groupSynthetic,
			probe:  "access",
			check:  "_",
			period: 5 * time.Second,
			config: checker.SmokeMiniAvailable{
				Path:        "/",
				DnsTimeout:  2 * time.Second,
				HttpTimeout: 2 * time.Second,
				Access:      access,
				Logger: entry.
					WithField("probe", "access").
					WithField("checker", "SmokeMiniAvailable"),
			},
		}, {
			group:  groupSynthetic,
			probe:  "dns",
			check:  "smoke",
			period: 200 * time.Millisecond,
			config: checker.SmokeMiniAvailable{
				Path:        "/dns",
				DnsTimeout:  2 * time.Second,
				HttpTimeout: 2 * time.Second,
				Access:      access,
				Logger: entry.
					WithField("probe", "dns").
					WithField("check", "smoke").
					WithField("checker", "SmokeMiniAvailable"),
			},
		}, {
			group:  groupSynthetic,
			probe:  "dns",
			check:  "internal",
			period: 200 * time.Millisecond,
			config: checker.DnsAvailable{
				Domain:     access.ClusterDomain(),
				DnsTimeout: 2 * time.Second,
				Logger: entry.
					WithField("probe", "dns").
					WithField("check", "internal").
					WithField("checker", "DnsAvailable"),
			},
		}, {
			group:  groupSynthetic,
			probe:  "neighbor",
			check:  "_",
			period: 5 * time.Second,
			config: checker.SmokeMiniAvailable{
				Path:        "/neighbor",
				DnsTimeout:  2 * time.Second,
				HttpTimeout: 4 * time.Second,
				Access:      access,
				Logger: entry.
					WithField("probe", "neighbor").
					WithField("checker", "SmokeMiniAvailable"),
			},
		}, {
			group:  groupSynthetic,
			probe:  "neighbor-via-service",
			check:  "_",
			period: 5 * time.Second,
			config: checker.SmokeMiniAvailable{
				Path:        "/neighbor-via-service",
				DnsTimeout:  2 * time.Second,
				HttpTimeout: 4 * time.Second,
				Access:      access,
				Logger: entry.
					WithField("probe", "service").
					WithField("checker", "SmokeMiniAvailable"),
			},
		},
	}
}
