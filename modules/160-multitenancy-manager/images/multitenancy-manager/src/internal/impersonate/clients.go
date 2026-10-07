/*
Copyright 2026 Flant JSC

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

// Package impersonate hands out clients that act as another user than the controller. The controller
// applies spec.manifests of a project template with such a client, so that the objects of a template
// get what the user of the project may do and not what the controller may: everything else, the Helm
// release storage included, keeps the client of the manager and the identity of the controller.
package impersonate

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// errNoUser is what For answers an empty user with: a client without a user to act as would act as
// the controller.
var errNoUser = errors.New("a client that impersonates needs a user to act as")

// Clients makes the clients that impersonate a user, from the REST config of the controller.
type Clients struct {
	config *rest.Config
	scheme *runtime.Scheme
	mapper meta.RESTMapper
}

// NewClients returns the maker of the clients that impersonate a user. The config is copied for every
// client and never changed, so the clients that share it keep the identity of the controller. The
// scheme and the REST mapper are shared with the manager, so a client costs no discovery.
func NewClients(config *rest.Config, scheme *runtime.Scheme, mapper meta.RESTMapper) *Clients {
	return &Clients{config: config, scheme: scheme, mapper: mapper}
}

// For returns a client whose every request acts as the user: the API server authorizes and admits
// the request as that user, with no groups but system:authenticated, which it adds to every user.
// The client reads from the API server, not from a cache. client-go keeps one transport for every
// config with the same TLS settings, so a client per call adds no connections.
func (c *Clients) For(user string) (client.Client, error) {
	if user == "" {
		return nil, errNoUser
	}

	config := rest.CopyConfig(c.config)
	config.Impersonate = rest.ImpersonationConfig{UserName: user}
	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("make the HTTP client that acts as %s: %w", user, err)
	}

	impersonating, err := client.New(config, client.Options{HTTPClient: httpClient, Scheme: c.scheme, Mapper: c.mapper})
	if err != nil {
		return nil, fmt.Errorf("make the client that acts as %s: %w", user, err)
	}
	return impersonating, nil
}
