/*
Copyright 2023 Flant JSC

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

package scope

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
