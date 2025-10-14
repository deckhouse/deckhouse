// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"flag"

	"github.com/deckhouse/deckhouse/pkg/log"
	"k8s.io/apimachinery/pkg/api/resource"
)

var RppSignCheck = "false"

type Config struct {
    KubeConfig         string
    ListenAddress      string
    DisableCache       bool
    SignCheck          bool
	CacheDirectory     string
	CacheRetentionSize resource.Quantity
	LogLevel           log.Level
}

func InitFlags() (*Config, error) {
	config := &Config{}

	flag.StringVar(&config.ListenAddress, "listen-address", ":5080", "Listen address for HTTP")
	flag.StringVar(&config.KubeConfig, "kubeconfig", "", "Path to kubeconfig")
	flag.BoolVar(&config.DisableCache, "disable-cache", false, "Disable cache")
	flag.StringVar(&config.CacheDirectory, "cache-directory", "/cache", "Path to cache directory")

	crs := flag.String("cache-retention-size", "1Gi", "Cache retention size")
	v := flag.String("log-level", "info", "Log verbosity")

	flag.Parse()

	var err error
	config.CacheRetentionSize, err = resource.ParseQuantity(*crs)
	if err != nil {
		return nil, err
	}
	config.LogLevel, err = log.ParseLevel(*v)
    if err != nil {
        return nil, err
    }

    if RppSignCheck == "true" {
        config.SignCheck = true
    }

    return config, nil
}
