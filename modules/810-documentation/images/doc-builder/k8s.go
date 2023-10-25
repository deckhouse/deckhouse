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

package main

import (
	"context"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createSyncConfigMap() error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("create rest config: %w", err)
	}

	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("new client set: %w", err)
	}

	const (
		ns   = "d8-system"
		name = "docs-sync"
	)

	cm := &core.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	_, err = kclient.CoreV1().ConfigMaps(ns).Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}

	return nil
}
