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
	"fmt"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/tools/clientcmd"
)

func InitClient(logger *log.Entry) *kubernetes.Clientset {
	config, err := clientcmd.BuildConfigFromFlags("", KubeConfig)
	if err != nil {
		logger.Fatal(fmt.Errorf("building kube client config: %v", err.Error()))
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatal(fmt.Errorf("creating dynamic client: %v", err.Error()))
	}

	return client
}

func InitDynamicClient(logger *log.Entry) dynamic.Interface {
	config, err := clientcmd.BuildConfigFromFlags("", KubeConfig)
	if err != nil {
		logger.Fatal(fmt.Errorf("building kube client config: %v", err.Error()))
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		logger.Fatal(fmt.Errorf("creating dynamic client: %v", err.Error()))
	}

	return dynClient
}
