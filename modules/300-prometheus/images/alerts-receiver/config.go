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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	reconcileTime = 5 * time.Minute
	nameSpace     = "d8-monitoring"
)

type configStruct struct {
	listenHost          string
	listenPort          string
	alertsQueueCapacity int
	logLevel            log.Level
	k8sClient           *kubernetes.Clientset
}

func newConfig() *configStruct {
	c := &configStruct{}
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
		c.alertsQueueCapacity = 100
	} else {
		l, err := strconv.Atoi(q)
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
		c.alertsQueueCapacity = l
	}

	c.logLevel = log.InfoLevel
	if d := os.Getenv("DEBUG"); d == "YES" {
		c.logLevel = log.DebugLevel
	}

	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}

	c.k8sClient, err = kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		log.Fatal(err)
	}

	return c
}
