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
	"os"
	"path/filepath"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
)

// createSAKeysIfNotExists ensures that the Service Account key pair (sa.key + sa.pub) exists.
//
// Three cases are handled:
//  1. Both sa.key and sa.pub exist — nothing to do.
//  2. sa.key exists but sa.pub is missing — sa.pub is restored from the existing private key.
//     Since the public key is mathematically derived from the private key, restoration is always
//     possible without loss of information.
//  3. sa.key does not exist — a new key pair is generated and both files are written.
//
// Note: sa.key and sa.pub are not X.509 certificates. They are a raw asymmetric key pair
// used by kube-controller-manager to sign ServiceAccount JWT tokens (sa.key) and by
// kube-apiserver to verify them (sa.pub).
func createSAKeysIfNotExists(cfg config) error {
	key, err := pkiutil.LoadKey(keyPath(cfg.pkiDir, "sa"))
	if err != nil && !isNotExistError(err) {
		return fmt.Errorf("failed to load SA private key: %w", err)
	}

	if err == nil {
		// sa.key exists — ensure sa.pub is also present.
		pubPath := filepath.Join(cfg.pkiDir, "sa.pub")
		if _, statErr := os.Stat(pubPath); statErr == nil {
			return nil
		}
		// sa.pub is missing — restore it from the existing key.
		return writeSAPublicKey(cfg.pkiDir, key)
	}

	// sa.key does not exist — create a new key pair.
	key, err = pkiutil.NewPrivateKey(cfg.EncryptionAlgorithmType)
	if err != nil {
		return fmt.Errorf("failed to generate SA private key: %w", err)
	}

	if err := writeKey(cfg.pkiDir, "sa", key); err != nil {
		return fmt.Errorf("failed to write SA private key: %w", err)
	}

	return writeSAPublicKey(cfg.pkiDir, key)
}
