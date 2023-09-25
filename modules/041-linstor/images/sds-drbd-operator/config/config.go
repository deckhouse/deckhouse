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

package config

import (
	"os"
)

// ScanInterval Scan block device interval seconds
const (
	ScanInterval     = 10
	ConfigSecretName = "d8-sds-drbd-operator-config"
	NodeName         = "NODE_NAME"
	MetricsPortEnv   = "METRICS_PORT"
)

type Options struct {
	ScanInterval     int
	ConfigSecretName string
	MetricsPort      string
}

func NewConfig() (*Options, error) {
	var opts Options
	opts.ScanInterval = ScanInterval
	opts.ConfigSecretName = ConfigSecretName

	opts.MetricsPort = os.Getenv(MetricsPortEnv)
	if opts.MetricsPort == "" {
		opts.MetricsPort = ":8080"
	}

	return &opts, nil
}
