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

package main

import (
	"os"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	appName                          = "prometheus"
	reconcileTime                    = 1 * time.Minute
	resolveTimeout                   = 5 * time.Minute
	contextTimeout                   = 10 * time.Second
	severityLabel                    = "severity_level"
	summaryLabel                     = "summary"
	descriptionLabel                 = "description"
	DMSAlertName                     = "DeadMansSwitch"
	MissingDMSAlertName              = "MissingDeadMansSwitch"
	ClusterHasTooManyAlertsAlertName = "ClusterHasTooManyAlerts"
)

type config struct {
	listenHost string
	listenPort string
	capacity   int
	logLevel   log.Level
}

func newConfig() *config {
	c := &config{}
	c.listenHost = os.Getenv("LISTEN_HOST")
	if c.listenHost == "" {
		c.listenHost = "0.0.0.0"
	}

	c.listenPort = os.Getenv("LISTEN_PORT")
	if c.listenPort == "" {
		c.listenPort = "8080"
	}

	q := os.Getenv("ALERTS_QUEUE_LENGTH")
	if q == "" {
		c.capacity = 100
	} else {
		l, err := strconv.Atoi(q)
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
		c.capacity = l
	}

	c.logLevel = log.InfoLevel
	if d := os.Getenv("DEBUG"); d == "YES" {
		c.logLevel = log.DebugLevel
	}

	return c
}
