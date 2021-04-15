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
