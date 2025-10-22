/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package agent

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
	corev1 "k8s.io/api/core/v1"
)

var (
	maxIdleConnDuration, _ = time.ParseDuration("100s")
	pool                   *sync.Pool
)

func init() {
	pool = &sync.Pool{
		New: func() any {
			return &fasthttp.Client{
				ReadTimeout:                   time.Duration(1) * time.Second,
				WriteTimeout:                  time.Duration(1) * time.Second,
				MaxIdleConnDuration:           maxIdleConnDuration,
				MaxConnsPerHost:               2048,
				NoDefaultUserAgentHeader:      true,
				DisableHeaderNamesNormalizing: true,
				DisablePathNormalizing:        true,
			}
		},
	}
}

type FastHTTPProbeTarget struct {
	insecureSkipTLSVerify bool
	targetPort            int
	successThreshold      int32
	failureThreshold      int32
	successCount          int32
	failureCount          int32
	timeoutSeconds        int32
	targetHost            string
	host                  string
	path                  string
	scheme                string
	method                string
	caCert                string
	httpHeaders           []corev1.HTTPHeader
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
	client := pool.Get().(*fasthttp.Client)
	client.ReadTimeout = time.Duration(h.timeoutSeconds) * time.Second
	client.WriteTimeout = time.Duration(h.timeoutSeconds) * time.Second
	if h.method == "https" {
		client.TLSConfig.InsecureSkipVerify = h.insecureSkipTLSVerify
		if h.caCert != "" {
			client.TLSConfig.ClientCAs.AppendCertsFromPEM([]byte(h.caCert))
		}
	}
	defer pool.Put(client)

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
