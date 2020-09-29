package deckhouse

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flant/candictl/pkg/log"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"flant/candictl/pkg/kubernetes/client"
)

type logLine struct {
	Module    string `json:"module,omitempty"`
	Level     string `json:"level,omitempty"`
	Output    string `json:"output,omitempty"`
	Message   string `json:"msg,omitempty"`
	Component string `json:"operator.component,omitempty"`
}

func PrintDeckhouseLogs(kubeCl *client.KubernetesClient, stopChan *chan struct{}) error {
	pods, err := kubeCl.CoreV1().Pods("d8-system").List(metav1.ListOptions{LabelSelector: "app=deckhouse"})
	if err != nil {
		return fmt.Errorf("Waiting for an API")
	}

	if len(pods.Items) < 1 {
		return fmt.Errorf("No Deckhouse pod found")
	}

	for _, pod := range pods.Items {
		message := fmt.Sprintf("Deckhouse pod found: %s (%s)", pod.Name, pod.Status.Phase)
		if pod.Status.Phase != corev1.PodRunning {
			return fmt.Errorf(message)
		}
		log.InfoLn(message)
		log.InfoLn("Running pod found! Checking logs...")
	}

	logOptions := corev1.PodLogOptions{Container: "deckhouse", TailLines: int64Pointer(5)}

	timer := time.NewTicker(3 * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-*stopChan:
			return nil
		case <-timer.C:
			request := kubeCl.CoreV1().Pods("d8-system").GetLogs(pods.Items[0].Name, &logOptions)
			result, err := request.DoRaw()
			if err != nil {
				return fmt.Errorf("Request failed. Probably pod doesn't exist anymore.")
			}

			currentTime := metav1.NewTime(time.Now())
			logOptions = corev1.PodLogOptions{Container: "deckhouse", SinceTime: &currentTime}

			printLogsByLine(result)
		}
	}
}

func printLogsByLine(content []byte) {
	reader := bufio.NewReader(bytes.NewReader(content))
	for {
		l, _, err := reader.ReadLine()
		if err != nil {
			break
		}
		var line logLine
		if err := json.Unmarshal(l, &line); err != nil {
			continue
		}

		if line.Level == "error" || (line.Output == "stderr" && line.Component != "tiller") {
			log.ErrorF("\t%s\n", line.Message)
			continue
		}

		// TODO use module.state label
		if line.Message == "Module run success" || line.Message == "ModuleRun success, module is ready" {
			log.InfoF("\tModule %q run successfully\n", line.Module)
			continue
		}
	}
}

func int64Pointer(i int) *int64 {
	r := int64(i)
	return &r
}
