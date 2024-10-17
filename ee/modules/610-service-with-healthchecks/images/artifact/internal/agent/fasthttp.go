/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package agent

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
	corev1 "k8s.io/api/core/v1"
)

var (
	maxIdleConnDuration, _ = time.ParseDuration("100s")
)

type FastHTTPProbeTarget struct {
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

func (h FastHTTPProbeTarget) GetID() string {
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

func (h FastHTTPProbeTarget) SetSuccessCount(count int32) Prober {
	h.successCount = count
	return h
}

func (h FastHTTPProbeTarget) SetFailureCount(count int32) Prober {
	h.failureCount = count
	return h
}

func (h FastHTTPProbeTarget) FailureCount() int32 {
	return h.failureCount
}

func (h FastHTTPProbeTarget) SuccessCount() int32 {
	return h.successCount
}

func (h FastHTTPProbeTarget) SuccessThreshold() int32 {
	return h.successThreshold
}

func (h FastHTTPProbeTarget) FailureThreshold() int32 {
	return h.failureThreshold
}

func (h FastHTTPProbeTarget) PerformCheck() error {
	client := &fasthttp.Client{
		ReadTimeout:                   time.Duration(h.timeoutSeconds) * time.Second,
		WriteTimeout:                  time.Duration(h.timeoutSeconds) * time.Second,
		MaxIdleConnDuration:           maxIdleConnDuration,
		NoDefaultUserAgentHeader:      true,
		DisableHeaderNamesNormalizing: true,
		DisablePathNormalizing:        true,
	}
	url := fmt.Sprintf("%s://%s:%d/%s", h.scheme, h.targetHost, h.targetPort, h.path)
	req := fasthttp.AcquireRequest()
	req.SetRequestURI(url)
	req.Header.SetMethod(h.method)

	resp := fasthttp.AcquireResponse()

	if h.host != "" {
		req.Header.Add("Host", h.host)
	}
	for i := range h.httpHeaders {
		req.Header.Add(h.httpHeaders[i].Name, h.httpHeaders[i].Value)
	}
	err := client.Do(req, resp)
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("HTTP bad status code %d", resp.StatusCode())
	}
	return nil
}

func (h FastHTTPProbeTarget) GetPort() int {
	return h.targetPort
}

func (h FastHTTPProbeTarget) GetMode() string {
	return "http"
}
