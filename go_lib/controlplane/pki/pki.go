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

package pki

import (
	"fmt"
	"net"
)

// CreatePKIBundle creates the full Kubernetes PKI on disk.
// It is the public entry point equivalent to `kubeadm init phase certs all`.
//
// What is created (with the default cert tree scheme):
//   - 3 self-signed CA certificates: ca, front-proxy-ca, etcd/ca
//   - 7 leaf certificates signed by their respective CAs
//   - SA key pair: sa.key and sa.pub
//
// The operation is idempotent:
//   - Existing CA certificates that pass validation are reused as-is.
//   - Existing leaf certificates that pass validation are kept unchanged.
//   - If a CA certificate fails validation, an error is returned — CAs are never auto-regenerated.
//   - If a leaf certificate fails validation, it is silently regenerated.
//   - If sa.key already exists, sa.pub is ensured to be present (restored from the key if missing).
func CreatePKIBundle(
	nodeName string,
	dnsDomain string,
	advertiseAddress net.IP,
	serviceCIDR string,
	opts ...configOption,
) error {
	cfg, err := newConfig(nodeName, dnsDomain, advertiseAddress, serviceCIDR, opts...)
	if err != nil {
		return fmt.Errorf("failed to create new config: %w", err)
	}

	return createPKIBundle(*cfg)
}

func createPKIBundle(cfg config) error {
	if err := createCertTree(cfg); err != nil {
		return err
	}

	if err := createSAKeysIfNotExists(cfg); err != nil {
		return err
	}

	return nil
}
