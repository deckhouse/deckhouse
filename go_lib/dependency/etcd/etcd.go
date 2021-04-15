package etcd

//go:generate minimock -i Client -o etcd_mock.go

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type Client interface {
	clientv3.KV
	clientv3.Watcher
}

func New(endpoints []string, options ...Option) (Client, error) {
	opts := &etcdOptions{}

	for _, opt := range options {
		opt(opts)
	}

	cfg := clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 10 * time.Second,
	}
	if opts.tls != nil {
		cfg.TLS = opts.tls
	}

	cli, err := clientv3.New(cfg)

	return cli, err
}

type etcdOptions struct {
	tls *tls.Config
}

type Option func(options *etcdOptions)

// WithClientCert add client certificate authentication
func WithClientCert(clientCert, caCert *x509.Certificate) Option {
	return func(options *etcdOptions) {
		pool := x509.NewCertPool()
		pool.AddCert(caCert)
		if options.tls == nil {
			options.tls = &tls.Config{}
		}

		options.tls.Certificates = []tls.Certificate{{Leaf: clientCert}}
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

		caData, err := ioutil.ReadFile(caFilePath)
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
