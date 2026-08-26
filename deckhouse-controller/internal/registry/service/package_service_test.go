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

package service

import (
	"encoding/base64"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// A docker config for the registry the cluster was installed from — and for nothing else, which is
// what an identity secret carries: those fields are read from outside the cluster, so they name a
// registry an outsider can reach.
func upstreamDockerConfig() string {
	return base64.StdEncoding.EncodeToString([]byte(
		`{"auths":{"dev-registry.example.com":{"username":"license-token","password":"secret"}}}`))
}

// The package controllers hand over the address as the cluster records it. When the registry module
// manages the nodes, that address is the node agent's, and two things about it are not properties of
// the registry the cluster was told about: nothing serves the name inside the cluster, and nothing
// authenticates to the agent.
//
// Measured on a cluster where the module manages nodes without a cache: every package scan failed with
// `"registry.d8-system.svc:5001/system/deckhouse/packages" credentials not found in the dockerCfg`,
// and with credentials added by hand, with `lookup registry.d8-system.svc: no such host`.
func TestPackagesAreFetchedThroughTheAgent(t *testing.T) {
	manager := NewPackageServiceManager(log.NewNop())

	recorded := registry_const.Host + "/system/deckhouse/packages"
	service, err := manager.Service(recorded, utils.RegistryConfig{
		DockerConfig: upstreamDockerConfig(),
		Scheme:       "HTTPS",
		UserAgent:    "test",
	})
	if err != nil {
		t.Fatalf("building a client for %q failed: %v", recorded, err)
	}

	want := registry_const.ProxyHost + "/system/deckhouse/packages/probe"
	if got := service.Package("probe").GetRoot(); got != want {
		t.Fatalf("the client dials %q; it has to dial the agent at %q", got, want)
	}
}

// And an ordinary registry is left exactly as it was handed over: the translation above applies to the
// agent's address and to nothing else.
func TestAnOrdinaryRegistryIsUntouched(t *testing.T) {
	manager := NewPackageServiceManager(log.NewNop())

	recorded := "dev-registry.example.com/sys/deckhouse-oss/packages"
	service, err := manager.Service(recorded, utils.RegistryConfig{
		DockerConfig: upstreamDockerConfig(),
		Scheme:       "HTTPS",
		UserAgent:    "test",
	})
	if err != nil {
		t.Fatalf("building a client for %q failed: %v", recorded, err)
	}

	if got := service.Package("probe").GetRoot(); got != recorded+"/probe" {
		t.Fatalf("the client dials %q, not the address it was given (%q)", got, recorded)
	}
}
