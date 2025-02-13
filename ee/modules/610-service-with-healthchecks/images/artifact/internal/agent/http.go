/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// Currently, the code uses a fasthttp probes implementation, but this implementation,
// which uses only the standard library, was left as a backup.

package agent

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
)

var (
	transport *http.Transport
)

func init() {
	transport = http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxConnsPerHost = 100
	transport.MaxIdleConnsPerHost = 100
}

type HTTPProbeTarget struct {
	targetPort       int
	successThreshold int32
	failureThreshold int32
	successCount     int32
	failureCount     int32
	timeoutSeconds   int32
	targetHost       string
	host             string
	path             string
	scheme           string
	method           string
	httpHeaders      []corev1.HTTPHeader
}

func (h HTTPProbeTarget) GetID() string {
	var sb strings.Builder
	sb.WriteString("http#")
	sb.WriteString(h.targetHost)
	sb.WriteString("#")
	sb.WriteString(fmt.Sprintf("%d", h.targetPort))
	sb.WriteString("#")
	sb.WriteString(h.path)
	sb.WriteString("#")
	sb.WriteString(h.host)
	return sb.String()
}

func (h HTTPProbeTarget) SetSuccessCount(count int32) Prober {
	h.successCount = count
	return h
}

func (h HTTPProbeTarget) SetFailureCount(count int32) Prober {
	h.failureCount = count
	return h
}

func (h HTTPProbeTarget) FailureCount() int32 {
	return h.failureCount
}

func (h HTTPProbeTarget) SuccessCount() int32 {
	return h.successCount
}

func (h HTTPProbeTarget) SuccessThreshold() int32 {
	return h.successThreshold
}

func (h HTTPProbeTarget) FailureThreshold() int32 {
	return h.failureThreshold
}

func (h HTTPProbeTarget) PerformCheck() error {
	c := http.Client{
		Timeout:   time.Duration(h.timeoutSeconds) * time.Second,
		Transport: transport,
	}
	url := fmt.Sprintf("%s://%s:%d/%s", h.scheme, h.targetHost, h.targetPort, h.path)
	req, err := http.NewRequest(h.method, url, nil)
	if err != nil {
		return err
	}
	if h.host != "" {
		req.Header.Add("Host", h.host)
	}
	for i := range h.httpHeaders {
		req.Header.Add(h.httpHeaders[i].Name, h.httpHeaders[i].Value)
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP bad status code %d", resp.StatusCode)
	}
	return nil
}

func (h HTTPProbeTarget) GetPort() int {
	return h.targetPort
}

func (h HTTPProbeTarget) GetMode() string {
	return "http"
}
