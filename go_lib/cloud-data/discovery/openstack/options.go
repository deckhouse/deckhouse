package openstack

import (
	"bytes"
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
			err = WithCertFile(caCertPath)(d)
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
		ok := d.transport.TLSClientConfig.
			RootCAs.AppendCertsFromPEM(bytes.TrimSpace(pemData))
		if !ok {
			return fmt.Errorf("error parsing CA Cert")
		}
		return nil
	}
}

// TODO: check ca cert file continuously at runtime.
// Ca cert updating might be needed to support prev discoverer version's
// functionality
func WithCertFile(caCertPath string) Option {
	return func(d *Discoverer) error {
		caCert, err := os.ReadFile(caCertPath)
		if err != nil {
			return fmt.Errorf("error reading CA Cert: %s", err)
		}
		return WithCaCert(caCert)(d)
	}
}
