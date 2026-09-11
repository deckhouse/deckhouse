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
	"os"
	"path/filepath"
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

// agentAuthority is the file whose presence says the agent is what serves the in-cluster
// address on this node. Generated on the node beside the agent, so a test has to stand in
// for it.
func agentAuthority(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "ca.crt")
	if err := os.WriteFile(path, []byte("AGENT-CA"), 0o600); err != nil {
		t.Fatalf("writing the stand-in authority: %v", err)
	}
	return path
}

// The package controllers hand over the address as the cluster records it. When the registry module
// manages the nodes, that address is the node agent's, and two things about it are not properties of
// the registry the cluster was told about: nothing serves the name inside the cluster, and nothing
// authenticates to the agent.
//
// So on a cluster whose registry module manages the nodes without a cache, every package scan fails
// with `"registry.d8-system.svc:5001/system/deckhouse/packages" credentials not found in the
// dockerCfg` — and once credentials are supplied, with `lookup registry.d8-system.svc: no such host`.
func TestPackagesAreFetchedThroughTheAgent(t *testing.T) {
	t.Cleanup(utils.WithAgentAuthority(agentAuthority(t)))

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

// TestPackagesUseTheRecordedAddressBeforeTheHandover is the state a cluster is in while it is
// still being migrated from the previous implementation of the registry module.
//
// That implementation serves the same in-cluster address from a proxy Service, which does ask
// for credentials and is verified by what the cluster recorded for it. Reading the address as
// the agent's there clears both and dials a port nothing listens on, and because this is the
// path packages are fetched through, the process cannot get far enough to install the agent
// that would have made the reading correct.
func TestPackagesUseTheRecordedAddressBeforeTheHandover(t *testing.T) {
	t.Cleanup(utils.WithAgentAuthority(filepath.Join(t.TempDir(), "absent")))

	manager := NewPackageServiceManager(log.NewNop())

	recorded := registry_const.Host + "/system/deckhouse/packages"
	service, err := manager.Service(recorded, utils.RegistryConfig{
		DockerConfig: inClusterDockerConfig(),
		Scheme:       "HTTPS",
		UserAgent:    "test",
	})
	if err != nil {
		t.Fatalf("building a client for %q failed: %v", recorded, err)
	}

	if got := service.Package("probe").GetRoot(); got != recorded+"/probe" {
		t.Fatalf("the client dials %q; the proxy serving this address answers to %q", got, recorded)
	}
}

// What a cluster on the previous implementation records: credentials for the in-cluster proxy,
// because that is what its nodes and its controller pull through.
func inClusterDockerConfig() string {
	return base64.StdEncoding.EncodeToString([]byte(
		`{"auths":{"` + registry_const.Host + `":{"username":"registry","password":"secret"}}}`))
}
