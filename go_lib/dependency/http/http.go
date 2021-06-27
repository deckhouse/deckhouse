/*
Copyright 2021 Flant CJSC

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

package http

//go:generate minimock -i Client -o http_mock.go

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// Client interface
type Client interface {
	Do(req *http.Request) (*http.Response, error)
}

func NewClient(options ...Option) Client {
	opts := &httpOptions{
		timeout: 10 * time.Second,
	}

	for _, opt := range options {
		opt(opts)
	}

	dialer := &net.Dialer{
		Timeout: opts.timeout,
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: opts.insecure,
		},
		IdleConnTimeout:       5 * time.Minute,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext:           dialer.DialContext,
		Dial:                  dialer.Dial,
	}

	return &http.Client{
		Timeout:   opts.timeout,
		Transport: tr,
	}
}

type httpOptions struct {
	timeout  time.Duration
	insecure bool
}

type Option func(options *httpOptions)

// WithTimeout set custom timeout for http request. Default: 10 seconds
func WithTimeout(t time.Duration) Option {
	return func(options *httpOptions) {
		options.timeout = t
	}
}

// WithInsecureSkipVerify skip tls certificate validation
func WithInsecureSkipVerify() Option {
	return func(options *httpOptions) {
		options.insecure = true
	}
}
