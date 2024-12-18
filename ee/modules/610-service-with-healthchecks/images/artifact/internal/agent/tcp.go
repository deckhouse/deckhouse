/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package agent

import (
	"fmt"
	"strings"
	"time"
)

type TCPProbeTarget struct {
	targetPort       int
	successThreshold int32
	failureThreshold int32
	successCount     int32
	failureCount     int32
	timeoutSeconds   int32
	targetHost       string
}

func (t TCPProbeTarget) GetID() string {
	var sb strings.Builder
	sb.WriteString("tcp#")
	sb.WriteString(t.targetHost)
	sb.WriteString("#")
	sb.WriteString(fmt.Sprintf("%d", t.targetPort))
	return sb.String()
}

func (t TCPProbeTarget) SuccessThreshold() int32 {
	return t.successThreshold
}

func (t TCPProbeTarget) FailureThreshold() int32 {
	return t.failureThreshold
}

func (t TCPProbeTarget) SuccessCount() int32 {
	return t.successCount
}

func (t TCPProbeTarget) FailureCount() int32 {
	return t.failureCount
}

func (t TCPProbeTarget) SetSuccessCount(count int32) Prober {
	t.successCount = count
	return t
}

func (t TCPProbeTarget) SetFailureCount(count int32) Prober {
	t.failureCount = count
	return t
}

func (t TCPProbeTarget) PerformCheck() error {
	timeoutDuration := time.Duration(t.timeoutSeconds) * time.Second
	d := ProbeDialer()
	d.Timeout = timeoutDuration
	conn, err := d.Dial("tcp", fmt.Sprintf("%s:%d", t.targetHost, t.targetPort))
	if err != nil {
		// Convert errors to failures to handle timeouts.
		return err
	}

	conn.SetWriteDeadline(time.Now().Add(timeoutDuration))
	if _, err = conn.Write([]byte("test")); err != nil {
		return err
	}

	err = conn.Close()
	if err != nil {
		return err
	}
	return nil
}

func (t TCPProbeTarget) GetPort() int {
	return t.targetPort
}

func (t TCPProbeTarget) GetMode() string {
	return "tcp"
}
