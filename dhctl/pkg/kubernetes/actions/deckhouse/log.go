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
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

var (
	ErrListPods      = errors.New("No Deckhouse pod found.")
	ErrReadLease     = errors.New("No Deckhouse leader election lease found.")
	ErrBadLease      = errors.New("Deckhouse leader election lease is malformed.")
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

// isErrorLine returns if log line is an error report:
// - level="error" - log.Error from deckhouse and Go hooks.
// - output="stderr" - errors from shell hooks.
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
			// we can race between creating resources and deploy crds
			// often we get only one error message per module
			// it confuses users and we want hide it
			"is not supported by cluster",
			// all bootstrapped cloud clusters has this message
			// it is normal because wait_for_all_master_nodes_to_become_initialized hook
			// blocks main queue with this error
			"timeout waiting for master nodes",
		}
		for _, p := range badSubStrings {
			if strings.Contains(line.Message, p) {
				return false
			}
		}
		return true
	}

	// Consider stderr messages are errors too.
	if line.Output == "stderr" {
		return true
	}

	return false
}

// isModuleSuccess returns true on message about successful module run.
func isModuleSuccess(line *logLine) bool {
	// Message about successful ModuleRun since PR#126 in flant/addon-operator.
	// https://github.com/flant/addon-operator/blob/7e814fbe92fb12af79c67c4226b4c2781d959f3c/pkg/addon-operator/operator.go#L1376
	return line.Message == "ModuleRun success, module is ready"
}

// isConvergeDone returns true when ConvergeModules task is done reloading all modules.
// Consider the first occurrence is the first converge success.
func isConvergeDone(line *logLine) bool {
	// Message about successful converge since PR#315 in flant/addon-operator.
	// https://github.com/flant/addon-operator/blob/7e814fbe92fb12af79c67c4226b4c2781d959f3c/pkg/addon-operator/operator.go#L588
	if line.Message == "ConvergeModules task done" {
		return true
	}
	// Message about successful converge prior PR#315 in flant/addon-operator.
	if line.Message == "Queue 'main' contains 0 converge tasks after handle 'ModuleHookRun'" {
		return true
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

type LogPrinter struct {
	kubeCl *client.KubernetesClient

	deckhousePod            *corev1.Pod
	waitPodBecomeReady      bool
	leaderElectionLeaseName types.NamespacedName

	lastErrorTime time.Time

	stopOutputNoMoreConvergeTasks bool

	excludeNodeName string

	deckhouseErrors []string
}

func NewLogPrinter(kubeCl *client.KubernetesClient) *LogPrinter {
	return &LogPrinter{
		kubeCl:          kubeCl,
		deckhouseErrors: make([]string, 0),
	}
}

func (d *LogPrinter) WaitPodBecomeReady() *LogPrinter {
	d.waitPodBecomeReady = true
	return d
}

func (d *LogPrinter) WithExcludeNode(nodeName string) *LogPrinter {
	d.excludeNodeName = nodeName
	return d
}

func (d *LogPrinter) WithLeaderElectionAwarenessMode(leaderElectionLease types.NamespacedName) *LogPrinter {
	d.leaderElectionLeaseName = leaderElectionLease
	return d
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
	// kubelet certificate on master can be changed before finish Deckhouse installation
	// and dhctl can not get logs from Deckhouse pod
	logOptions.InsecureSkipTLSVerifyBackend = true

	var result []byte

	var lastErr error
	err := retry.NewSilentLoop("getting logs for error", 2, 1*time.Second).Run(func() error {
		request := d.kubeCl.CoreV1().Pods("d8-system").GetLogs(d.deckhousePod.Name, &logOptions)
		result, lastErr = request.DoRaw(context.TODO())
		if lastErr != nil {
			log.DebugF("printErrorsForTask: %s\n %s", lastErr.Error(), string(result))
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
			d.deckhouseErrors = append(d.deckhouseErrors, line.String())
			log.DebugF("Error during Deckhouse converge: %s", line.String())
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

		if isModuleSuccess(line) {
			log.InfoF("\tModule %q run successfully\n", line.Module)
			return true
		}

		if !d.stopOutputNoMoreConvergeTasks && isConvergeDone(line) {
			log.InfoLn("No more converge tasks found in Deckhouse queue.")
			d.stopOutputNoMoreConvergeTasks = true
			return true
		}

		// let it be in debug
		log.DebugF(line.StringWithLogLevel())
		return true
	})
}

func (d *LogPrinter) GetPod() error {
	pod, err := GetPod(d.kubeCl, d.leaderElectionLeaseName)
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
		log.DebugF("checkDeckhousePodReady: %s\n", err.Error())
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

	logOptions := corev1.PodLogOptions{
		Container: "deckhouse",
		TailLines: int64Pointer(10),

		// kubelet certificate on master can be changed before finish Deckhouse installation
		// and dhctl can not get logs from Deckhouse pod
		InsecureSkipTLSVerifyBackend: true,
	}

	defer func() { d.deckhousePod = nil }()

	for {
		select {
		case <-ctx.Done():
			log.ErrorLn(strings.Join(d.deckhouseErrors, "\n"))
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
				log.DebugF("Print: %s\n %s", err.Error(), string(result))
				return false, ErrRequestFailed
			}

			d.printLogsByLine(result)

			time.Sleep(time.Second)
			currentTime := metav1.NewTime(time.Now())
			logOptions = corev1.PodLogOptions{
				Container: "deckhouse",
				SinceTime: &currentTime,
				// see above
				InsecureSkipTLSVerifyBackend: true,
			}
		}
	}
}

func int64Pointer(i int) *int64 {
	r := int64(i)
	return &r
}
