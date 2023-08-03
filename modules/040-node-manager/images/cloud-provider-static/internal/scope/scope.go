package scope

import (
	"k8s.io/client-go/rest"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
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
