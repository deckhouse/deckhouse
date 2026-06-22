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

// Command webhook is a Kubernetes mutating admission webhook that rewrites
// ModuleSource.spec.registry to the local in-cluster registry service.
package main

import (
	"log/slog"
	"os"

	"registry-webhook/internal/creds"
	"registry-webhook/internal/server"
)

const (
	// pkiDir is the mount path of the registry-module-pki secret.
	pkiDir = "/etc/registry-module-pki"
	// certsDir is the mount path of the webhook TLS secret.
	certsDir = "/certs"
	// listenAddr is the address the webhook listens on.
	listenAddr = ":9443"
)

func main() {
	local, err := creds.Load(pkiDir)
	if err != nil {
		slog.Error("load creds", "err", err)
		os.Exit(1)
	}

	h := server.Handler(local)

	slog.Info("registry-webhook starting", "addr", listenAddr)
	if err := server.ListenAndServeTLS(listenAddr, certsDir, h); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
