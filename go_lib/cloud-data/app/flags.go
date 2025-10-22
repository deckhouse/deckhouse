// Copyright 2023 Flant JSC
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

	"github.com/alecthomas/kingpin"
	"github.com/deckhouse/deckhouse/pkg/log"
)

var (
	DiscoveryPeriod = 1 * time.Hour
	KubeConfig      = ""
	ListenAddress   = "127.0.0.1:9000"
	LoggerLevel     = int(log.LevelInfo)
)

func InitFlags(cmd *kingpin.Application) {
	cmd.Flag("discovery-period", "Period for request cloud data").
		Envar("DISCOVERY_PERIOD").
		Default(DiscoveryPeriod.String()).
		DurationVar(&DiscoveryPeriod)

	cmd.Flag("listen-address", "Listen address for HTTP").
		Envar("LISTEN_ADDRESS").
		Default(ListenAddress).
		StringVar(&ListenAddress)

	cmd.Flag("kubeconfig", "Path to kubeconfig").
		Envar("KUBECONFIG").
		Default(KubeConfig).
		StringVar(&KubeConfig)

	cmd.Flag("v", "Logger verbosity").
		Envar("LOGGER_LEVEL").
		Default(strconv.Itoa(LoggerLevel)).
		IntVar(&LoggerLevel)
}
