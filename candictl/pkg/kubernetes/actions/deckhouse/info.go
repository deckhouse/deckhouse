package deckhouse

import (
	"fmt"

	"sigs.k8s.io/yaml"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"flant/candictl/pkg/kubernetes/client"
	"flant/candictl/pkg/log"
	"flant/candictl/pkg/util/retry"
)

func GetClusterInfo(kubeCl *client.KubernetesClient) string {
	var globalData string
	err := retry.StartSilentLoop("Get info from Deckhouse ConfigMap", 5, 2, func() error {
		deckhouseConfigMap, err := kubeCl.CoreV1().ConfigMaps("d8-system").Get("deckhouse", metav1.GetOptions{})
		if err != nil {
			return err
		}

		globalData = deckhouseConfigMap.Data["global"]
		return nil
	})
	if err != nil {
		return globalData
	}

	log.DebugF(globalData + "\n")
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
