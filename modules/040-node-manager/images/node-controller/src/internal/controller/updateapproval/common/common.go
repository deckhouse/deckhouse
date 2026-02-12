/*
Copyright 2025 Flant JSC

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

package common

import (
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// Re-exported from internal/common so existing consumers using the ua. alias continue to work.
const (
	NodeGroupLabel                   = nodecommon.NodeGroupLabel
	ConfigurationChecksumAnnotation  = nodecommon.ConfigurationChecksumAnnotation
	MachineNamespace                 = nodecommon.MachineNamespace
	ConfigurationChecksumsSecretName = nodecommon.ConfigurationChecksumsSecretName
	ApprovedAnnotation               = nodecommon.ApprovedAnnotation
	WaitingForApprovalAnnotation     = nodecommon.WaitingForApprovalAnnotation
	DisruptionRequiredAnnotation     = nodecommon.DisruptionRequiredAnnotation
	DisruptionApprovedAnnotation     = nodecommon.DisruptionApprovedAnnotation
	RollingUpdateAnnotation          = nodecommon.RollingUpdateAnnotation
	DrainingAnnotation               = nodecommon.DrainingAnnotation
	DrainedAnnotation                = nodecommon.DrainedAnnotation
)

type NodeInfo struct {
	Name      string
	NodeGroup string

	ConfigurationChecksum string

	IsReady              bool
	IsApproved           bool
	IsDisruptionApproved bool
	IsWaitingForApproval bool
	IsDisruptionRequired bool
	IsUnschedulable      bool
	IsDraining           bool
	IsDrained            bool
	IsRollingUpdate      bool
}

func BuildNodeInfo(node *corev1.Node) NodeInfo {
	annotations := node.Annotations
	if annotations == nil {
		annotations = make(map[string]string)
	}

	info := NodeInfo{
		Name:                  node.Name,
		NodeGroup:             node.Labels[NodeGroupLabel],
		ConfigurationChecksum: annotations[ConfigurationChecksumAnnotation],
		IsUnschedulable:       node.Spec.Unschedulable,
	}

	_, info.IsApproved = annotations[ApprovedAnnotation]
	_, info.IsWaitingForApproval = annotations[WaitingForApprovalAnnotation]
	_, info.IsDisruptionRequired = annotations[DisruptionRequiredAnnotation]
	_, info.IsDisruptionApproved = annotations[DisruptionApprovedAnnotation]
	_, info.IsRollingUpdate = annotations[RollingUpdateAnnotation]

	if v, ok := annotations[DrainingAnnotation]; ok && v == "bashible" {
		info.IsDraining = true
	}
	if v, ok := annotations[DrainedAnnotation]; ok && v == "bashible" {
		info.IsDrained = true
	}

	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			info.IsReady = true
			break
		}
	}

	return info
}

func GetApprovalMode(ng *v1.NodeGroup) string {
	if ng.Spec.Disruptions != nil && ng.Spec.Disruptions.ApprovalMode != "" {
		return string(ng.Spec.Disruptions.ApprovalMode)
	}
	return "Automatic"
}

func CalculateConcurrency(maxConcurrent *intstr.IntOrString, totalNodes int) int {
	if maxConcurrent == nil {
		return 1
	}

	switch maxConcurrent.Type {
	case intstr.Int:
		return maxConcurrent.IntValue()
	case intstr.String:
		s := maxConcurrent.String()
		if strings.HasSuffix(s, "%") {
			percentStr := strings.TrimSuffix(s, "%")
			percent, _ := strconv.Atoi(percentStr)
			concurrency := totalNodes * percent / 100
			if concurrency == 0 {
				concurrency = 1
			}
			return concurrency
		}
		return maxConcurrent.IntValue()
	}

	return 1
}

func IsInAllowedWindow(windows []v1.DisruptionWindow, now time.Time) bool {
	if len(windows) == 0 {
		return true
	}
	for _, w := range windows {
		if IsWindowAllowed(w, now) {
			return true
		}
	}
	return false
}

func IsWindowAllowed(w v1.DisruptionWindow, now time.Time) bool {
	now = now.UTC()

	const hhMM = "15:04"
	fromInput, err := time.Parse(hhMM, w.From)
	if err != nil {
		return false
	}
	toInput, err := time.Parse(hhMM, w.To)
	if err != nil {
		return false
	}

	fromTime := time.Date(now.Year(), now.Month(), now.Day(), fromInput.Hour(), fromInput.Minute(), 0, 0, time.UTC)
	toTime := time.Date(now.Year(), now.Month(), now.Day(), toInput.Hour(), toInput.Minute(), 0, 0, time.UTC)

	if !IsDayAllowed(now, w.Days) {
		return false
	}

	if !toTime.After(fromTime) {
		return now.Equal(fromTime) || now.After(fromTime) || now.Before(toTime) || now.Equal(toTime)
	}

	return now.Equal(fromTime) || now.Equal(toTime) || (now.After(fromTime) && now.Before(toTime))
}

func IsDayAllowed(now time.Time, days []string) bool {
	if len(days) == 0 {
		return true
	}
	for _, d := range days {
		if IsDayEqual(now, d) {
			return true
		}
	}
	return false
}

func IsDayEqual(today time.Time, dayString string) bool {
	var day time.Weekday

	switch strings.ToLower(dayString) {
	case "mon", "monday":
		day = time.Monday
	case "tue", "tuesday":
		day = time.Tuesday
	case "wed", "wednesday":
		day = time.Wednesday
	case "thu", "thursday":
		day = time.Thursday
	case "fri", "friday":
		day = time.Friday
	case "sat", "saturday":
		day = time.Saturday
	case "sun", "sunday":
		day = time.Sunday
	default:
		return false
	}

	return today.Weekday() == day
}
