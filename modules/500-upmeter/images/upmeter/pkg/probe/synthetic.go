package probe

import (
	"time"

	"d8.io/upmeter/pkg/probe/checker"
)

func Synthetic() []runnerConfig {
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
				DnsTimeout:  100 * time.Millisecond,
				HttpTimeout: 100 * time.Millisecond,
			},
		}, {
			group:  groupName,
			probe:  "dns",
			check:  "internal",
			period: 200 * time.Millisecond,
			config: checker.DnsAvailable{
				Domain:     "kubernetes.default",
				DnsTimeout: 100 * time.Millisecond,
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
