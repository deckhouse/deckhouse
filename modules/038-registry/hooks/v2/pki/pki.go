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

// Package pki holds the certificates and credentials of the in-cluster storage of
// the controller-based implementation.
//
// This is deliberately separate from the PKI of the node agent, which is generated
// on the node at bootstrap and never leaves it. The agent's only relying party is
// the containerd on its own node, so there is nothing about it to agree with the
// cluster; splitting the two means neither has to wait for the other, and a node
// joining an unreachable cluster still gets a working agent.
//
// What is generated here is the other half: the certificate the storage serves
// with, the token service that authorizes pulls against it, and the credentials
// used to read from it and to write into it.
package pki

import (
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

// Names of the things this package generates, used both in the secrets and in the
// values.
const (
	// CommonNameCA is the authority every storage certificate is signed by.
	CommonNameCA = "registry-storage-ca"

	// CommonNameDistribution is the certificate the storage serves with.
	CommonNameDistribution = "registry-storage"

	// CommonNameAuth is the token service that authorizes pulls.
	CommonNameAuth = "registry-storage-auth"

	// CommonNameToken signs the tokens the auth service issues. The storage
	// verifies them against this certificate, which is why it is a separate key
	// from the serving one: a leaked serving key must not let anyone mint tokens.
	CommonNameToken = "registry-storage-token"

	// CommonNameIngressClient is the client certificate the ingress controller
	// presents when proxying writes. The storage trusts this authority for the write
	// path only, so the publication endpoint cannot be reached by anything that has
	// not gone through the ingress.
	CommonNameIngressClient = "registry-storage-ingress-client"
)

// User names. Read and write credentials are separate on purpose: the write
// endpoint is reachable from outside the cluster, and a leak of the credentials
// every node uses to pull must not also grant the ability to replace images.
const (
	// UserRO is used by the node agents. Pull and nothing else: this password is on every
	// node of the cluster, so it must not be able to change what a node will pull next.
	UserRO = "registry-ro"

	// UserRW is used by `d8 mirror push` against the publication endpoint.
	UserRW = "registry-rw"

	// UserSync is used by a replica writing into its OWN store — the fill from an upstream
	// and the replication from the leader.
	//
	// A third identity rather than either of the two above, and neither would do. The node
	// account cannot write at all, which is right: it is distributed everywhere. Reusing the
	// publication account would put the password that can replace images through an
	// internet-facing endpoint into the environment of every replica on every master, turning
	// one exposed surface into as many as there are masters.
	//
	// This one is never handed to a node and never reachable through the ingress: it exists
	// inside the storage's own pods, where the registry it writes to is a container beside it.
	//
	// Its absence was a real defect, and an invisible one: replication was assigned the node
	// account, so every follower in an air-gapped cluster asked for `pull,push`, was issued
	// `pull`, and had all 629 references refused with "insufficient scope" — while its status
	// said only that it was replicating from the leader.
	UserSync = "registry-sync"
)

// CertKey is a certificate with its private key, in PEM.
type CertKey struct {
	Cert string `json:"cert"`
	Key  string `json:"key"`
}

// User is a set of credentials, with the hash the token service compares against.
type User struct {
	Name         string `json:"name"`
	Password     string `json:"password"`
	PasswordHash string `json:"passwordHash"`
}

// State is everything the storage needs to serve and to be talked to.
type State struct {
	CA            CertKey `json:"ca"`
	Distribution  CertKey `json:"distribution"`
	Auth          CertKey `json:"auth"`
	Token         CertKey `json:"token"`
	IngressClient CertKey `json:"ingressClient"`

	// HTTPSecret signs the state carried in upload URLs. Identical across replicas,
	// otherwise an upload started against one replica cannot be finished against
	// another — which is exactly what happens behind a load-balancing ingress.
	HTTPSecret string `json:"httpSecret"`

	RO User `json:"ro"`
	RW User `json:"rw"`

	// Sync is what a replica writes to its own store with. See UserSync.
	Sync User `json:"sync"`
}

// Hosts the serving certificate has to be valid for.
type Hosts struct {
	// Service is the in-cluster name every image reference is built from.
	Service string

	// Additional are the addresses replicas are also reached at: the node addresses,
	// since the storage publishes a host port so that a node can pull before the
	// cluster network is up.
	Additional []string
}

func (h Hosts) all() []string {
	hosts := make([]string, 0, len(h.Additional)+3)
	if h.Service != "" {
		hosts = append(hosts, h.Service)
	}
	// The loopback is covered too, but not for the syncer: the pod is host-networked, so the
	// loopback it shares belongs to the NODE, where the registry agent answers on the same
	// port. The syncer therefore talks to its own replica over the address it serves on, which
	// is in Additional. What the loopback covers is anything inside the pod that does mean
	// itself — the probes, and a deployment that is not host-networked.
	hosts = append(hosts, "127.0.0.1", "localhost")
	hosts = append(hosts, h.Additional...)
	return hosts
}

// Generate builds a complete, self-consistent state.
func Generate(hosts Hosts) (State, error) {
	ca, err := pki.GenerateCACertificate(CommonNameCA)
	if err != nil {
		return State{}, fmt.Errorf("generating the certificate authority: %w", err)
	}

	state := State{}
	if state.CA, err = encode(ca); err != nil {
		return State{}, fmt.Errorf("encoding the certificate authority: %w", err)
	}

	for _, item := range []struct {
		name   string
		hosts  []string
		target *CertKey
	}{
		{name: CommonNameDistribution, hosts: hosts.all(), target: &state.Distribution},
		{name: CommonNameAuth, hosts: hosts.all(), target: &state.Auth},
		// The token certificate signs tokens rather than serving traffic, so it needs
		// no host names at all.
		{name: CommonNameToken, target: &state.Token},
		{name: CommonNameIngressClient, target: &state.IngressClient},
	} {
		certificate, err := pki.GenerateCertificate(item.name, ca, item.hosts...)
		if err != nil {
			return State{}, fmt.Errorf("generating the %s certificate: %w", item.name, err)
		}
		if *item.target, err = encode(certificate); err != nil {
			return State{}, fmt.Errorf("encoding the %s certificate: %w", item.name, err)
		}
	}

	if state.HTTPSecret, err = pki.GenerateRandomSecret(); err != nil {
		return State{}, fmt.Errorf("generating the HTTP secret: %w", err)
	}

	if state.RO, err = generateUser(UserRO); err != nil {
		return State{}, fmt.Errorf("generating the read user: %w", err)
	}
	if state.Sync, err = generateUser(UserSync); err != nil {
		return State{}, fmt.Errorf("generating the sync user: %w", err)
	}
	if state.RW, err = generateUser(UserRW); err != nil {
		return State{}, fmt.Errorf("generating the write user: %w", err)
	}

	return state, nil
}

// EnsureHosts reissues the serving certificates when the set of addresses the
// storage is reached at has changed.
//
// Reissued rather than left alone because a master node joining the cluster is
// ordinary, and a certificate that does not cover its address would make every
// agent on that node fail to verify the storage — with no error anywhere near the
// node list that changed.
//
// Only the leaf certificates change; the authority stays, so nothing that already
// trusts it has to be told.
func (s *State) EnsureHosts(hosts Hosts) (bool, error) {
	ca, err := pki.DecodeCertKey([]byte(s.CA.Cert), []byte(s.CA.Key))
	if err != nil {
		return false, fmt.Errorf("decoding the certificate authority: %w", err)
	}

	covered, err := certificateCovers(s.Distribution.Cert, hosts.all())
	if err != nil {
		return false, err
	}
	if covered {
		return false, nil
	}

	for _, item := range []struct {
		name   string
		target *CertKey
	}{
		{name: CommonNameDistribution, target: &s.Distribution},
		{name: CommonNameAuth, target: &s.Auth},
	} {
		certificate, err := pki.GenerateCertificate(item.name, ca, hosts.all()...)
		if err != nil {
			return false, fmt.Errorf("reissuing the %s certificate: %w", item.name, err)
		}
		if *item.target, err = encode(certificate); err != nil {
			return false, fmt.Errorf("encoding the %s certificate: %w", item.name, err)
		}
	}

	return true, nil
}

// Complete reports whether the state has everything the storage needs.
//
// A partially filled state is worse than an empty one: the empty one is
// regenerated, while the partial one would be handed to the storage and fail at
// TLS handshake time with nothing pointing at the missing piece.
func (s *State) Complete() bool {
	if s == nil {
		return false
	}
	for _, certificate := range []CertKey{s.CA, s.Distribution, s.Auth, s.Token, s.IngressClient} {
		if certificate.Cert == "" || certificate.Key == "" {
			return false
		}
	}
	if s.HTTPSecret == "" {
		return false
	}
	for _, user := range []User{s.RO, s.RW, s.Sync} {
		if user.Name == "" || user.Password == "" || user.PasswordHash == "" {
			return false
		}
	}
	return true
}

func generateUser(name string) (User, error) {
	password, err := pki.GenerateUserPassword()
	if err != nil {
		return User{}, err
	}
	hash, err := pki.GeneratePasswordHash(password)
	if err != nil {
		return User{}, err
	}
	return User{Name: name, Password: password, PasswordHash: hash}, nil
}

func encode(value pki.CertKey) (CertKey, error) {
	cert, key, err := pki.EncodeCertKey(value)
	if err != nil {
		return CertKey{}, err
	}
	return CertKey{Cert: string(cert), Key: string(key)}, nil
}
