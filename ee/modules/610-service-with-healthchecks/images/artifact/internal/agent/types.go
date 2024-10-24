/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package agent

import (
	"fmt"
	"net"
	"reflect"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/types"
)

type Prober interface {
	PerformCheck() error
	GetPort() int
	GetMode() string
	FailureThreshold() int32
	SuccessThreshold() int32
	SuccessCount() int32
	FailureCount() int32
	GetID() string
	SetSuccessCount(count int32) Prober
	SetFailureCount(count int32) Prober
}

type HealthcheckTarget struct {
	creationTime       time.Time
	lastCheck          time.Time
	targetHost         string
	podName            string
	podNamespace       string
	podUID             types.UID
	podReady           bool
	probeResultDetails []ProbeResultDetail
}

type ProbeCounts struct {
	failureCount int32
	successCount int32
}

func (ht HealthcheckTarget) GetProbeResultDetailsMap() map[string]ProbeCounts {
	result := make(map[string]ProbeCounts)
	for _, probeResultDetail := range ht.probeResultDetails {
		result[probeResultDetail.id] = ProbeCounts{
			successCount: probeResultDetail.successCount,
			failureCount: probeResultDetail.failureCount,
		}
	}
	return result
}

func (ht HealthcheckTarget) GetRenewedProbes(probes []Prober) []Prober {
	newProbes := make([]Prober, 0, len(probes))
	probesResultDetailsMap := ht.GetProbeResultDetailsMap()
	for _, prob := range probes {
		counts := probesResultDetailsMap[prob.GetID()]
		newProbes = append(newProbes, prob.SetSuccessCount(counts.successCount).SetFailureCount(counts.failureCount))
	}
	return newProbes
}

func (ht HealthcheckTarget) EqualTo(target HealthcheckTarget) bool {
	if !ht.creationTime.Equal(target.creationTime) {
		return false
	}
	if ht.targetHost != target.targetHost {
		return false
	}
	if ht.podName != target.podName {
		return false
	}
	if ht.podUID != target.podUID {
		return false
	}
	if !reflect.DeepEqual(ht.probeResultDetails, target.probeResultDetails) {
		return false
	}
	return true
}

func (ht HealthcheckTarget) FailedProbes() []string {
	var failedProbes []string
	for _, probe := range ht.probeResultDetails {
		if probe.successCount < probe.successThreshold || probe.failureCount >= probe.failureThreshold {
			failedProbes = append(failedProbes, fmt.Sprintf("%s:%s:%d", probe.mode, ht.targetHost, probe.targetPort))
		}
	}
	return failedProbes
}

type ProbeResult struct {
	host         string
	successful   bool
	serviceName  types.NamespacedName
	probeDetails []ProbeResultDetail
}

type ProbeResultDetail struct {
	successful       bool
	targetPort       int
	successCount     int32
	failureCount     int32
	successThreshold int32
	failureThreshold int32
	mode             string
	id               string
}

type ProbeTask struct {
	host        string
	serviceName types.NamespacedName
	probes      []Prober
}

type ProbeTaskIdentity struct {
	host        string
	serviceName types.NamespacedName
}

func ProbeDialer() *net.Dialer {
	dialer := &net.Dialer{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				syscall.SetsockoptLinger(int(fd), syscall.SOL_SOCKET, syscall.SO_LINGER, &syscall.Linger{Onoff: 1, Linger: 1})
			})
		},
	}
	return dialer
}
