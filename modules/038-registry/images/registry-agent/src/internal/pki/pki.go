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

// Package pki reads the agent's own certificate from the node.
//
// Generated on the node at bootstrap, and never leaving it. The only party that has
// to trust it is the container runtime on this very node, so there is nothing about it
// to agree with the cluster — which is what lets a node come up with a working agent
// before it can reach the cluster at all, and why this is separate from the storage
// PKI the module generates.
//
// Read from disk rather than held in memory so that a rotation is picked up without
// restarting the agent. Restarting it would mean a window in which nothing on the node
// can pull.
package pki

import (
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// DefaultDir is where bashible puts the agent's material.
const DefaultDir = "/etc/kubernetes/registry-agent/pki"

// File names inside the directory.
const (
	CAFile          = "ca.crt"
	CertificateFile = "agent.crt"
	KeyFile         = "agent.key"
)

// OnDisk is the agent's certificate material on the node.
type OnDisk struct {
	// Dir holds the material. Empty means DefaultDir.
	Dir string

	mutex   sync.Mutex
	loaded  *tls.Certificate
	fromKey string
	fromCrt string
}

func (o *OnDisk) dir() string {
	if o.Dir == "" {
		return DefaultDir
	}
	return o.Dir
}

// Path of one of the files.
func (o *OnDisk) Path(name string) string { return filepath.Join(o.dir(), name) }

// CA returns the certificate authority the runtime is told to trust.
//
// Read on every call rather than cached: it goes into the runtime's drop-in, and a
// rotation that the drop-in does not follow would leave the runtime verifying against
// an authority the agent no longer serves with.
func (o *OnDisk) CA() (string, error) {
	content, err := os.ReadFile(o.Path(CAFile))
	if err != nil {
		return "", fmt.Errorf("reading the agent certificate authority: %w", err)
	}
	if len(content) == 0 {
		return "", fmt.Errorf("the agent certificate authority at %s is empty", o.Path(CAFile))
	}
	return string(content), nil
}

// Certificate returns the current serving certificate, reloading it when the files on
// disk have changed.
//
// Suitable as a tls.Config GetCertificate callback, which is the point: a rotation
// takes effect on the next handshake instead of on a restart.
func (o *OnDisk) Certificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	certificatePEM, err := os.ReadFile(o.Path(CertificateFile))
	if err != nil {
		return nil, fmt.Errorf("reading the agent certificate: %w", err)
	}
	keyPEM, err := os.ReadFile(o.Path(KeyFile))
	if err != nil {
		return nil, fmt.Errorf("reading the agent key: %w", err)
	}

	o.mutex.Lock()
	defer o.mutex.Unlock()

	if o.loaded != nil && o.fromCrt == string(certificatePEM) && o.fromKey == string(keyPEM) {
		return o.loaded, nil
	}

	// Parsed only when the bytes changed: this runs on every handshake, and a pull is
	// many connections.
	certificate, err := tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("the agent certificate and key do not form a pair: %w", err)
	}

	o.loaded = &certificate
	o.fromCrt = string(certificatePEM)
	o.fromKey = string(keyPEM)
	return o.loaded, nil
}

// Ready reports whether the material is present and usable, so the agent can say what
// is missing instead of failing at the first handshake.
//
// Worth checking up front because the material is written by a bashible step, and a
// node where that step has not run yet is an ordinary state rather than a fault.
func (o *OnDisk) Ready() error {
	if _, err := o.CA(); err != nil {
		return err
	}
	if _, err := o.Certificate(nil); err != nil {
		return err
	}
	return nil
}

// TLSConfig serves with the on-disk certificate, reloading it as it rotates.
func (o *OnDisk) TLSConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: o.Certificate,
		MinVersion:     tls.VersionTLS12,
	}
}
