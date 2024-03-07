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
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	RegistrySecretDiscoveryPeriod = time.Hour
	KubeConfig                    = ""
	ListenAddress                 = ":5080"
	CacheDirectory                = "/var/lib/registry-packages-proxy"
	CacheRetentionSize            = "1Gi"
	// CacheRetentionPeriod by default is 30 days
	CacheRetentionPeriod = 30 * 24 * time.Hour
	LoggerType           = loggerJSON
	LoggerLevel          = int(logrus.InfoLevel)
)

func InitFlags(cmd *kingpin.Application) {
	cmd.Flag("registry-secret-discovery-period", "Period for registry secret discovery").
		Envar("REGISTRY_SECRET_DISCOVERY_PERIOD").
		Default(RegistrySecretDiscoveryPeriod.String()).
		DurationVar(&RegistrySecretDiscoveryPeriod)

	cmd.Flag("listen-address", "Listen address for HTTP").
		Envar("LISTEN_ADDRESS").
		Default(ListenAddress).
		StringVar(&ListenAddress)

	cmd.Flag("kubeconfig", "Path to kubeconfig").
		Envar("KUBECONFIG").
		Default(KubeConfig).
		StringVar(&KubeConfig)

	cmd.Flag("cache-directory", "Path to cache directory").
		Envar("CACHE_DIRECTORY").
		Default(CacheDirectory).
		StringVar(&CacheDirectory)

	cmd.Flag("cache-retention-size", "Cache retention size").
		Envar("CACHE_RETENTION_SIZE").
		Default(CacheRetentionSize).
		StringVar(&CacheRetentionSize)

	cmd.Flag("cache-retention-period", "Cache retention period").
		Envar("CACHE_RETENTION_PERIOD").
		Default(CacheRetentionPeriod.String()).
		DurationVar(&CacheRetentionPeriod)

	cmd.Flag("logger-type", "Format logs output of a discoverer in different ways.").
		Envar("LOGGER_TYPE").
		Default(LoggerType).
		EnumVar(&LoggerType, loggerJSON, loggerSimple)

	cmd.Flag("v", "Logger verbosity").
		Envar("LOGGER_LEVEL").
		Default(strconv.Itoa(int(LoggerLevel))).
		IntVar(&LoggerLevel)
}
