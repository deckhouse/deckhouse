/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package scopes

import (
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Scope defines a scope.
type Scope struct {
	Client client.Client
	Config *rest.Config

	PatchHelper *patch.Helper
	Logger      logr.Logger
}

// NewScope creates a new scope.
func NewScope(
	client client.Client,
	config *rest.Config,
	logger logr.Logger,
) (*Scope, error) {
	if client == nil {
		return nil, errors.New("Client is required when creating a Scope")
	}
	if config == nil {
		return nil, errors.New("Config is required when creating a Scope")
	}

	return &Scope{
		Client: client,
		Config: config,
		Logger: logger,
	}, nil
}
