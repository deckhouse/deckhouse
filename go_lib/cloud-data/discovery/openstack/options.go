package openstack

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
)

func WithOptionsFromEnv() Option {
	return func(d *Discoverer) error {
		authOpts, err := openstack.AuthOptionsFromEnv()
		if err != nil {
			return fmt.Errorf("cannot get opts from env: %w", err)
		}
		d.authOpts = authOpts

		region := os.Getenv("OS_REGION")
		if region == "" {
			return fmt.Errorf("cannot get OS_REGION env")
		}
		d.region = region

		clusterUUID, ok := os.LookupEnv("CLUSTER_UUID")
		if ok {
			d.clusterUUID = clusterUUID
		}

		moduleConfig, ok := os.LookupEnv("MODULE_CONFIG")
		if ok {
			d.moduleConfig = []byte(moduleConfig)
		}

		if caCertPath := os.Getenv("OS_CACERT"); caCertPath != "" {
			err = WithCaCertFile(caCertPath)(d)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

func WithAuthOptions(authOpts gophercloud.AuthOptions) Option {
	return func(d *Discoverer) error {
		d.authOpts = authOpts

		return nil
	}
}

func WithRegion(region string) Option {
	return func(d *Discoverer) error {
		d.region = region

		return nil
	}
}

func WithCaCert(pemData []byte) Option {
	return func(d *Discoverer) error {
		ok := d.loadCaCert(pemData, false)
		if !ok {
			return fmt.Errorf("error parsing CA Cert")
		}
		return nil
	}
}

func WithCaCertFile(caCertPath string) Option {
	return func(d *Discoverer) error {
		d.caCertPath = caCertPath
		return nil
	}
}

// loadCaCert adds PEM CA certificate to discoverer transport. Cert will be appended to chain, if overwrite is set to false.
// If overwrite is true, provided cert will replace the whole existing chain (needed to preserve previous version's logic).
func (d *Discoverer) loadCaCert(pemData []byte, overwrite bool) bool {
	if overwrite {
		config := &tls.Config{}
		config.RootCAs = x509.NewCertPool()
		d.transport.TLSClientConfig = config
	}
	return d.transport.TLSClientConfig.
		RootCAs.AppendCertsFromPEM(bytes.TrimSpace(pemData))
}
