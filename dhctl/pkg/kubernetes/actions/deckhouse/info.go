// Copyright 2021 Flant CJSC
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

package deckhouse

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func GetClusterInfo(kubeCl *client.KubernetesClient) string {
	var globalData string
	err := retry.NewSilentLoop("Get info from Deckhouse ConfigMap", 5, 2*time.Second).Run(func() error {
		deckhouseConfigMap, err := kubeCl.CoreV1().ConfigMaps("d8-system").Get(context.TODO(), "deckhouse", metav1.GetOptions{})
		if err != nil {
			return err
		}

		globalData = deckhouseConfigMap.Data["global"]
		return nil
	})
	if err != nil {
		return globalData
	}

	log.DebugLn(globalData)
	var clusterInfo struct {
		ClusterName string `yaml:"clusterName,omitempty"`
		Project     string `yaml:"project,omitempty"`
	}

	err = yaml.Unmarshal([]byte(globalData), &clusterInfo)
	if err != nil {
		log.InfoLn(err)
		return ""
	}

	return fmt.Sprintf("Cluster:\t%s\nProject:\t%s\n", clusterInfo.ClusterName, clusterInfo.Project)
}
