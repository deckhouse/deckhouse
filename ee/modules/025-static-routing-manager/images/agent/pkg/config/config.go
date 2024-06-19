/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	LogLevelENV                           = "LOG_LEVEL"
	RequeueIntervalENV                    = "REQUEUE_INTERVAL"
	ProbeAddressPortENV                   = "PROBE_ADDRESS_PORT"
	MetricsAddressPortENV                 = "METRICS_ADDRESS_PORT"
	ControllerNamespaceEnv                = "CONTROLLER_NAMESPACE"
	NodeNameENV                           = "NODE_NAME"
	PeriodicReconciliationIntervalENV     = "PERIODIC_RECONCILE_INTERVAL"
	ControllerName                        = "static-routing-manager-agent"
	defaultRequeueInterval                = 10
	defaultPeriodicReconciliationInterval = 30
)

const (
	ErrorLevel   = "0"
	WarningLevel = "1"
	InfoLevel    = "2"
	DebugLevel   = "3"
	TraceLevel   = "4"
)

const (
	WarnLvl = iota + 1
	InfoLvl
	DebugLvl
	TraceLvl
)

type Options struct {
	Loglevel                       string
	ProbeAddressPort               string
	MetricsAddressPort             string
	ControllerNamespace            string
	RequeueInterval                time.Duration
	PeriodicReconciliationInterval time.Duration
	NodeName                       string
}

func NewConfig() (*Options, error) {
	var opts Options

	loglevel := os.Getenv(LogLevelENV)
	switch loglevel {
	case "Error":
		opts.Loglevel = ErrorLevel
	case "Warning":
		opts.Loglevel = WarningLevel
	case "Info":
		opts.Loglevel = InfoLevel
	case "Debug":
		opts.Loglevel = DebugLevel
	case "Trace":
		opts.Loglevel = TraceLevel
	default:
		opts.Loglevel = DebugLevel
	}

	probeAddressPort := os.Getenv(ProbeAddressPortENV)
	if probeAddressPort == "" {
		opts.ProbeAddressPort = ":0"
	} else {
		opts.ProbeAddressPort = probeAddressPort
	}

	metricsAddressPort := os.Getenv(MetricsAddressPortENV)
	if metricsAddressPort == "" {
		opts.MetricsAddressPort = ":0"
	} else {
		opts.MetricsAddressPort = metricsAddressPort
	}

	controllerNamespace := os.Getenv(ControllerNamespaceEnv)
	if controllerNamespace == "" {
		opts.ControllerNamespace = "d8-static-routing-manager"
	} else {
		opts.ControllerNamespace = controllerNamespace
	}

	requeueInterval := os.Getenv(RequeueIntervalENV)
	if requeueInterval != "" {
		// ri, err := strconv.ParseInt(requeueInterval, 10, 64)
		ri, err := strconv.Atoi(requeueInterval)
		if err != nil {
			opts.RequeueInterval = time.Duration(ri)
		} else {
			opts.RequeueInterval = time.Duration(defaultRequeueInterval)
		}
	} else {
		opts.RequeueInterval = time.Duration(defaultRequeueInterval)
	}

	periodicReconciliationInterval := os.Getenv(PeriodicReconciliationIntervalENV)
	if periodicReconciliationInterval != "" {
		// ri, err := strconv.ParseInt(requeueInterval, 10, 64)
		pri, err := strconv.Atoi(periodicReconciliationInterval)
		if err != nil {
			opts.PeriodicReconciliationInterval = time.Duration(pri)
		} else {
			opts.PeriodicReconciliationInterval = time.Duration(defaultPeriodicReconciliationInterval)
		}
	} else {
		opts.PeriodicReconciliationInterval = time.Duration(defaultPeriodicReconciliationInterval)
	}

	nodeName := os.Getenv(NodeNameENV)
	if nodeName != "" {
		opts.NodeName = nodeName
	} else {
		return nil, fmt.Errorf("%s environment variable not set", NodeNameENV)
	}

	return &opts, nil
}
