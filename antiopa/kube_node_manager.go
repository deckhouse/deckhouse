package main

import (
	"encoding/json"
	"github.com/romana/rlog"
	"os"
	"sort"
	"time"

	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	KubeNodeChanged chan bool

	nodeListResourceVersion string
	nodeListChecksum        string
)

func InitKubeNodeManager() {
	rlog.Debug("Init kube node manager")

	KubeNodeChanged = make(chan bool, 1)

	nodeList, err := KubernetesClient.CoreV1().Nodes().List(metaV1.ListOptions{})
	if err != nil {
		rlog.Errorf("Cannot get nodes list: %s", err)
		os.Exit(1)
	}

	nodeListChecksum = calculateNodeListChecksum(nodeList)
}

func RunKubeNodeManager() {
	rlog.Debug("Run kube node manager")

	for {
		time.Sleep(time.Duration(10) * time.Second)

		nodeList, err := KubernetesClient.CoreV1().Nodes().List(metaV1.ListOptions{})
		if err != nil {
			rlog.Errorf("KUBE-NODES watch list failed: %s", err)
			continue
		}

		checksum := calculateNodeListChecksum(nodeList)
		if nodeListChecksum != checksum {
			nodeListChecksum = checksum

			KubeNodeChanged <- true
		}
	}
}

func calculateNodeListChecksum(nodeList *v1.NodeList) string {
	checksum := ""
	for _, node := range nodeList.Items {
		checksum = calculateChecksum(checksum, node.Name)

		annotationKeys := make([]string, 0, len(node.Annotations))
		for k := range node.Annotations {
			annotationKeys = append(annotationKeys, k)
		}
		sort.Strings(annotationKeys)
		for _, k := range annotationKeys {
			checksum = calculateChecksum(checksum, k, node.Annotations[k])
		}

		labelsKeys := make([]string, 0, len(node.Labels))
		for k := range node.Labels {
			labelsKeys = append(labelsKeys, k)
		}
		sort.Strings(labelsKeys)
		for _, k := range labelsKeys {
			checksum = calculateChecksum(checksum, k, node.Labels[k])
		}

		jsonSpec, _ := json.Marshal(node.Spec)
		checksum = calculateChecksum(checksum, string(jsonSpec))
	}
	return checksum
}
