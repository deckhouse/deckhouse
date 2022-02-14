// Copyright 2021 Flant JSC
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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

var (
	ErrListPods      = errors.New("No Deckhouse pod found.")
	ErrTimedOut      = errors.New("Time is out waiting for Deckhouse readiness.")
	ErrRequestFailed = errors.New("Request failed. Probably pod was restarted during installation.")
	ErrIncorrectNode = errors.New("Deckhouse on wrong node")
)

type logLine struct {
	Module    string    `json:"module,omitempty"`
	Level     string    `json:"level,omitempty"`
	Output    string    `json:"output,omitempty"`
	Message   string    `json:"msg,omitempty"`
	Component string    `json:"operator.component,omitempty"`
	TaskID    string    `json:"task.id,omitempty"`
	Source    string    `json:"source,omitempty"`
	Time      time.Time `json:"time,omitempty"`
}

func (l *logLine) String() string {
	return fmt.Sprintf("\t%s/%s: %s\n", l.Module, l.Component, l.Message)
}

func (l *logLine) StringWithLogLevel() string {
	return fmt.Sprintf("\t%s/%s: [%s] %s\n", l.Module, l.Component, l.Level, l.Message)
}

func isErrorLine(line *logLine) bool {
	if line.Level == "error" {
		badSubStrings := []string{
			"Client.Timeout exceeded while awaiting headers",
			// skip this message because hook may receive entrypoint, but
			// api server didn't create yet.
			// But after next iteration hook has pod and entry point together
			// It can confuse dhctl user.
			// We cannot skip this error in hook,
			// because kube version needs for next installation steps
			"Not found k8s versions",
		}
		for _, p := range badSubStrings {
			if strings.Contains(line.Message, p) {
				return false
			}
		}
		return true
	}

	if line.Output == "stderr" {
		// skip tiller output
		if line.Component == "tiller" {
			return false
		}

		return false
	}

	return false
}

func parseLogByLine(content []byte, action func(line *logLine) bool) {
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

		cont := action(&line)
		if !cont {
			break
		}
	}
}

func (d *LogPrinter) printErrorsForTask(taskID string, errorTaskTime time.Time) {
	if taskID == "" {
		return
	}

	logOptions := corev1.PodLogOptions{Container: "deckhouse", TailLines: int64Pointer(100)}
	if !d.lastErrorTime.IsZero() {
		t := metav1.NewTime(d.lastErrorTime)
		logOptions = corev1.PodLogOptions{Container: "deckhouse", SinceTime: &t}
	}

	var result []byte

	var lastErr error
	err := retry.NewSilentLoop("getting logs for error", 2, 1*time.Second).Run(func() error {
		request := d.kubeCl.CoreV1().Pods("d8-system").GetLogs(d.deckhousePod.Name, &logOptions)
		result, lastErr = request.DoRaw(context.TODO())
		if lastErr != nil {
			return ErrRequestFailed
		}

		return nil
	})
	if err != nil {
		log.DebugLn(lastErr)
		return
	}

	parseLogByLine(result, func(line *logLine) bool {
		if line.TaskID == "" || line.TaskID != taskID {
			return true
		}

		if line.Source == "klog" {
			return true
		}

		if line.Time.IsZero() {
			return true
		}

		if line.Time.After(errorTaskTime) {
			return false
		}

		if !d.lastErrorTime.IsZero() && line.Time.Equal(d.lastErrorTime) {
			return true
		}

		if line.Level == "error" || line.Output == "stderr" {
			log.ErrorF(line.String())
		}
		return true
	})

	d.lastErrorTime = errorTaskTime
}

func (d *LogPrinter) printLogsByLine(content []byte) {
	parseLogByLine(content, func(line *logLine) bool {
		if isErrorLine(line) {
			d.printErrorsForTask(line.TaskID, line.Time)
			return true
		}

		// TODO use module.state label
		if line.Message == "Module run success" || line.Message == "ModuleRun success, module is ready" {
			log.InfoF("\tModule %q run successfully\n", line.Module)
			return true
		}

		if !d.stopOutputNoMoreConvergeTasks && line.Message == "Queue 'main' contains 0 converge tasks after handle 'ModuleHookRun'" {
			log.InfoLn("No more converge tasks found in Deckhouse queue.")
			d.stopOutputNoMoreConvergeTasks = true
			return true
		}

		// let it be in debug
		log.DebugF(line.StringWithLogLevel())
		return true
	})
}

type LogPrinter struct {
	kubeCl *client.KubernetesClient

	deckhousePod       *corev1.Pod
	waitPodBecomeReady bool

	lastErrorTime time.Time

	stopOutputNoMoreConvergeTasks bool

	excludeNodeName string
}

func NewLogPrinter(kubeCl *client.KubernetesClient) *LogPrinter {
	return &LogPrinter{kubeCl: kubeCl}
}

func (d *LogPrinter) WaitPodBecomeReady() *LogPrinter {
	d.waitPodBecomeReady = true
	return d
}

func (d *LogPrinter) WithExcludeNode(nodeName string) *LogPrinter {
	d.excludeNodeName = nodeName
	return d
}

func (d *LogPrinter) GetPod() error {
	pod, err := GetPod(d.kubeCl)
	if err != nil {
		return err
	}

	message := fmt.Sprintf("Deckhouse pod found: %s (%s)", pod.Name, pod.Status.Phase)
	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf(message)
	}

	log.InfoLn(message)
	log.InfoLn("Running pod found! Checking logs...")

	d.deckhousePod = pod
	return nil
}

func (d *LogPrinter) checkDeckhousePodReady() (bool, error) {
	if !d.waitPodBecomeReady || d.deckhousePod == nil {
		return false, nil
	}

	runningPod, err := d.kubeCl.CoreV1().Pods("d8-system").Get(context.TODO(), d.deckhousePod.Name, metav1.GetOptions{})
	if err != nil {
		return false, ErrRequestFailed
	}

	if d.excludeNodeName != "" && runningPod.Spec.NodeName == d.excludeNodeName {
		return false, ErrIncorrectNode
	}

	ready := true
	for _, condition := range runningPod.Status.Conditions {
		if condition.Status == corev1.ConditionTrue {
			continue
		}
		ready = false
		log.DebugF("Pod %s is not ready: %s = %s\n", d.deckhousePod.Name, condition.Type, condition.Status)
	}

	return ready, nil
}

func (d *LogPrinter) Print(ctx context.Context) (bool, error) {
	if err := d.GetPod(); err != nil {
		return false, err
	}

	logOptions := corev1.PodLogOptions{Container: "deckhouse", TailLines: int64Pointer(5)}
	defer func() { d.deckhousePod = nil }()

	for {
		select {
		case <-ctx.Done():
			return false, ErrTimedOut
		default:
			ready, err := d.checkDeckhousePodReady()
			if err != nil {
				return false, err
			}
			if ready {
				return true, nil
			}

			request := d.kubeCl.CoreV1().Pods("d8-system").GetLogs(d.deckhousePod.Name, &logOptions)
			result, err := request.DoRaw(context.TODO())
			if err != nil {
				log.DebugLn(err)
				return false, ErrRequestFailed
			}

			d.printLogsByLine(result)

			time.Sleep(time.Second)
			currentTime := metav1.NewTime(time.Now())
			logOptions = corev1.PodLogOptions{Container: "deckhouse", SinceTime: &currentTime}
		}
	}
}

func int64Pointer(i int) *int64 {
	r := int64(i)
	return &r
}
