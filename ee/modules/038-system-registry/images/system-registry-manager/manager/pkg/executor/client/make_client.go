/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package client

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"net/http"
)

type Client struct {
	logger  *logrus.Entry
	address string
	port    int
	client  *http.Client
}

func NewClient(logger *logrus.Entry, address string, port int) *Client {
	return &Client{
		logger:  logger,
		address: address,
		port:    port,
		client:  &http.Client{},
	}
}

func (client *Client) getUrl() string {
	return fmt.Sprintf("http://%s:%d", client.address, client.port)
}

func (client *Client) getHeaders() map[string]string {
	return map[string]string{}
}

func (client *Client) RequestCheckRegistry(request *CheckRegistryRequest) (*CheckRegistryResponse, error) {
	var response CheckRegistryResponse
	err := RequestCheckRegistry(client.logger, client.client, client.getUrl(), client.getHeaders(), request, &response)
	return &response, err
}

func (client *Client) RequestCreateRegistry(request *CreateRegistryRequest) error {
	return RequestCreateRegistry(client.logger, client.client, client.getUrl(), client.getHeaders(), request)
}

func (client *Client) RequestUpdateRegistry(request *UpdateRegistryRequest) error {
	return RequestUpdateRegistry(client.logger, client.client, client.getUrl(), client.getHeaders(), request)
}

func (client *Client) RequestDeleteRegistry() error {
	return RequestDeleteRegistry(client.logger, client.client, client.getUrl(), client.getHeaders())
}

func (client *Client) RequestIsBusy(request *IsBusyRequest) (*IsBusyResponse, error) {
	var response IsBusyResponse
	err := RequestIsBusy(client.logger, client.client, client.getUrl(), client.getHeaders(), request, &response)
	return &response, err
}
