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

package config

import (
	"os"
	"static-routing-manager-controller/pkg/logger"
	"strconv"
	"time"
)

const (
	LogLevelENV            = "LOG_LEVEL"
	RequeueIntervalENV     = "REQUEUE_INTERVAL"
	ProbeAddressPortENV    = "PROBE_ADDRESS_PORT"
	ControllerNamespaceEnv = "CONTROLLER_NAMESPACE"
	ControllerName         = "static-routing-manager-controller"
)

type Options struct {
	Loglevel            logger.Verbosity
	ProbeAddressPort    string
	ControllerNamespace string
	RequeueInterval     time.Duration
}

func NewConfig() *Options {
	var opts Options

	loglevel := os.Getenv(LogLevelENV)
	if loglevel == "" {
		opts.Loglevel = logger.DebugLevel
	} else {
		opts.Loglevel = logger.Verbosity(loglevel)
	}

	probeAddressPort := os.Getenv(ProbeAddressPortENV)
	if probeAddressPort == "" {
		opts.ProbeAddressPort = ":0"
	} else {
		opts.ProbeAddressPort = probeAddressPort
	}

	controllerNamespace := os.Getenv(ControllerNamespaceEnv)
	if controllerNamespace == "" {
		opts.ControllerNamespace = "d8-static-routing-manager"
	} else {
		opts.ControllerNamespace = controllerNamespace
	}

	requeueInterval := os.Getenv(RequeueIntervalENV)
	if requeueInterval != "" {
		ri, err := strconv.ParseInt(requeueInterval, 10, 64)
		if err != nil {
			opts.RequeueInterval = time.Duration(ri)
		} else {
			opts.RequeueInterval = time.Duration(10)
		}
	} else {
		opts.RequeueInterval = time.Duration(10)
	}

	// opts.RequeueInterval = 10

	return &opts
}
