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

package constant

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// TestHostIsNotAPort pins the in-cluster registry name to its literal.
//
// It is recorded in every pod spec, every ModuleSource and `deckhouse-registry`, and in the name
// of a containerd host directory on every node, so a change to it renames every image the cluster
// runs. It used to be assembled from the one port constant there was, and moving the store's
// listener would have moved it as well. The `:5001` in it is part of a name nothing dials.
func TestHostIsNotAPort(t *testing.T) {
	if Host != "registry.d8-system.svc:5001" {
		t.Fatalf("Host is %q; it is a name recorded in every image reference of every cluster and must not change", Host)
	}
	if HostWithPath != "registry.d8-system.svc:5001/system/deckhouse" {
		t.Fatalf("HostWithPath is %q; it is what image references are built from and must not change", HostWithPath)
	}
}

// TestAgentStaysOnItsAddress pins the node agent's address.
//
// Unlike the store's ports it is dialled by processes that update on their own schedule —
// Deckhouse itself, registry-packages-proxy, the static pod of an immutable node — and each of
// them spells it out or takes it from here at its own build.
func TestAgentStaysOnItsAddress(t *testing.T) {
	if ProxyHost != "127.0.0.1:5001" {
		t.Fatalf("ProxyHost is %q; the agent's address is a contract with components released separately", ProxyHost)
	}
}

// TestStorePortsAreInThePlatformRange keeps the store's listeners where a host firewall written
// from the documentation lets them through.
//
// Both cross between nodes — agents and replicas reach the serving one, the ingress controller
// reaches the write one — and the documented host firewall opens 4200-4299 and the ports listed
// beside it. A port outside that range is a cache that silently stops working between nodes on a
// cluster configured exactly as documented.
func TestStorePortsAreInThePlatformRange(t *testing.T) {
	for name, port := range map[string]int{"StoragePort": StoragePort, "StorageWritePort": StorageWritePort} {
		if port < 4200 || port > 4299 {
			t.Errorf("%s is %d, outside the 4200-4299 range the platform documents for host firewalls", name, port)
		}
	}

	if StoragePort != 4281 || StorageWritePort != 4283 {
		t.Errorf("the store listens on %d and %d; docs/documentation/_data/deckhouse-ports.yml, the "+
			"templates and the bashible steps of the bootstrap registry name 4281 and 4283",
			StoragePort, StorageWritePort)
	}

	// The store and the agent are on the same node on every master; they used to share a number on
	// two addresses, which only held while nothing took the port on all of them.
	if StoragePort == AgentPort || StorageWritePort == AgentPort {
		t.Errorf("a store port equals the agent's %d", AgentPort)
	}
}

// TestStorageHostIsTheServiceAtTheServingPort pins what a process with no agent to go through dials
// to read the store itself: dhctl in the pod network, and registry-packages-proxy on a master that
// has no agent yet, whose own literal TestRegistryPackagesProxyCarriesTheseLiterals compares with
// this one.
//
// The Service is headless, so its name resolves to the masters' node addresses and the port is the
// client's: it has to be the one the replicas serve on, not the `:5001` in Host, which on a node is
// only the agent on its loopback. And the name has to stay Host's, because that is the one the
// store's serving certificate is issued for.
func TestStorageHostIsTheServiceAtTheServingPort(t *testing.T) {
	if StorageHost != "registry.d8-system.svc:4281" {
		t.Fatalf("StorageHost is %q; the store serves on %d under the Service's name", StorageHost, StoragePort)
	}
	if StorageHostWithPath != "registry.d8-system.svc:4281/system/deckhouse" {
		t.Fatalf("StorageHostWithPath is %q", StorageHostWithPath)
	}

	serviceName, _, _ := strings.Cut(Host, ":")
	storageName, _, _ := strings.Cut(StorageHost, ":")
	if serviceName != storageName {
		t.Errorf("StorageHost names %q and Host names %q; the certificate covers only the Service's name",
			storageName, serviceName)
	}
	if StorageHost == Host {
		t.Errorf("StorageHost is Host; at %s a node has only the agent, on its loopback", Host)
	}
}

// rppAddresses is the file in which registry-packages-proxy spells out the addresses below. The
// proxy is its own Go module and does not import this package, so its literals are read from the
// source.
const rppAddresses = "../../../modules/039-registry-packages-proxy/images/registry-packages-proxy/src/internal/credentials/agent.go"

// TestRegistryPackagesProxyCarriesTheseLiterals compares the addresses registry-packages-proxy
// carries as literals with the values here, so that a change on either side fails a test.
//
// The proxy's own test compares its literals with copies of the same strings, which catches an
// edit to the proxy and nothing else: StoragePort moved here, with this package's tests updated to
// match, would have left the proxy dialling the previous port on a master with no agent, and every
// test green.
func TestRegistryPackagesProxyCarriesTheseLiterals(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), rppAddresses, nil, 0)
	if err != nil {
		t.Fatalf("reading the proxy's literals: %v", err)
	}

	literals := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value := spec.(*ast.ValueSpec)
			for i, name := range value.Names {
				if i >= len(value.Values) {
					continue
				}
				if lit, ok := value.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					literals[name.Name], _ = strconv.Unquote(lit.Value)
				}
			}
		}
	}

	for name, want := range map[string]string{
		"recordedHost":          Host,
		"storePath":             strings.TrimLeft(Path, "/"),
		"storageHost":           StorageHost,
		"proxyHost":             ProxyHost,
		"agentCAFile":           AgentCAFile,
		"imageAddressConfigMap": ImageAddressConfigMapName,
		"imageAddressKey":       ImageAddressConfigMapKey,
	} {
		got, ok := literals[name]
		if !ok {
			t.Errorf("the proxy no longer declares %s as a string literal in %s", name, rppAddresses)
			continue
		}
		if got != want {
			t.Errorf("the proxy's %s is %q, and here it is %q", name, got, want)
		}
	}
}
