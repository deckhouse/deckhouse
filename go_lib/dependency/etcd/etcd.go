/*
Copyright 2021 Flant JSC

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

package etcd

//go:generate minimock -i Client -o etcd_mock.go

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"net"
	"os"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

type Client interface {
	clientv3.Cluster
	clientv3.KV
	clientv3.Watcher
	clientv3.Maintenance
}

func New(endpoints []string, options ...Option) (Client, error) {
	opts := &etcdOptions{}

	for _, opt := range options {
		opt(opts)
	}

	cfg := clientv3.Config{
		Endpoints:            endpoints,
		DialTimeout:          10 * time.Second,
		AutoSyncInterval:     30 * time.Second,
		DialKeepAliveTime:    60 * time.Second,
		DialKeepAliveTimeout: 5 * time.Second,
	}
	// ignore HTTPS_PROXY settings
	direct := func(ctx context.Context, addr string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", addr)
	}
	cfg.DialOptions = append(cfg.DialOptions, grpc.WithContextDialer(direct))

	if opts.tls != nil {
		cfg.TLS = opts.tls
	}
	if opts.insecureSkipVerify {
		if cfg.TLS == nil {
			cfg.TLS = &tls.Config{}
		}
		cfg.TLS.InsecureSkipVerify = true
	}

	return clientv3.New(cfg)
}

type etcdOptions struct {
	tls                *tls.Config
	insecureSkipVerify bool
}

type Option func(options *etcdOptions)

// WithClientCert add client certificate authentication
func WithClientCert(clientCert *tls.Certificate, caCert *x509.Certificate) Option {
	return func(options *etcdOptions) {
		pool := x509.NewCertPool()
		pool.AddCert(caCert)
		if options.tls == nil {
			options.tls = &tls.Config{}
		}

		options.tls.Certificates = []tls.Certificate{*clientCert}
		options.tls.RootCAs = pool
	}
}

// WithClientCertFile add client certificate authentication from files
func WithClientCertFile(caFilePath, certFilePath, keyFilePath string) Option {
	return func(options *etcdOptions) {
		cert, err := tls.LoadX509KeyPair(certFilePath, keyFilePath)
		if err != nil {
			return
		}

		caData, err := os.ReadFile(caFilePath)
		if err != nil {
			return
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caData)

		if options.tls == nil {
			options.tls = &tls.Config{}
		}

		options.tls.Certificates = []tls.Certificate{cert}
		options.tls.RootCAs = pool
	}
}

func WithBase64Certs(ca, cert, key string) Option {
	return func(options *etcdOptions) {
		caData, err := base64.StdEncoding.DecodeString(ca)
		if err != nil {
			return
		}
		certData, err := base64.StdEncoding.DecodeString(cert)
		if err != nil {
			return
		}
		keyData, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			return
		}

		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caData)

		cert, err := tls.X509KeyPair(certData, keyData)
		if err != nil {
			return
		}

		if options.tls == nil {
			options.tls = &tls.Config{}
		}

		options.tls.Certificates = []tls.Certificate{cert}
		options.tls.RootCAs = pool
	}
}

// WithInsecureSkipVerify skip tls check
func WithInsecureSkipVerify() Option {
	return func(options *etcdOptions) {
		options.insecureSkipVerify = true
	}
}
