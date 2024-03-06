/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package zvirtapi

import (
	"errors"
	"log"

	ovirtclientlog "github.com/ovirt/go-ovirt-client-log/v3"
	ovirtclient "github.com/ovirt/go-ovirt-client/v3"
)

var ErrNotFound = errors.New("not found")

type ZvirtCloudAPI struct {
	ComputeSvc *ComputeService
}

func NewZvirtCloudAPI(apiURL, username, password string, insecure bool) (*ZvirtCloudAPI, error) {
	logger := ovirtclientlog.NewGoLogger(log.Default())

	tls := ovirtclient.TLS()

	if insecure {
		tls.Insecure()
	} else {
		tls.CACertsFromSystem()
	}

	client, err := ovirtclient.New(
		apiURL,
		username,
		password,
		tls,
		logger,
		nil,
	)
	if err != nil {
		return nil, err
	}

	return &ZvirtCloudAPI{
		ComputeSvc: NewComputeService(client),
	}, nil
}
