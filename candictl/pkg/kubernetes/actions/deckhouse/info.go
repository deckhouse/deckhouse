package deckhouse

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/candictl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/candictl/pkg/log"
	"github.com/deckhouse/deckhouse/candictl/pkg/util/retry"
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
