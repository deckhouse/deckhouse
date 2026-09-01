// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package external

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

const (
	networkTCP  = "tcp"
	networkUnix = "unix"

	loopbackHost = "127.0.0.1"

	socketDirPrefix = "validator"
	socketFileName  = "validator.sock"
	socketDirMode   = 0o700
)

// Endpoint is an address a validator is told to listen on and the host then dials.
type Endpoint interface {
	// Network is the network the validator listens on: tcp, unix.
	Network() string
	// Address is what the validator binds.
	Address() string
	// DialTarget is the same endpoint in gRPC's notation.
	DialTarget() string
	// Accepting reports whether something already listens on the endpoint.
	Accepting(timeout time.Duration) bool
	// Free releases whatever the endpoint reserved.
	Free() error
	// String is a human-readable form for logs and errors.
	String() string
}

// tcpEndpoint is the whole of a TCP tcpEndpoint and the shared part of every other one.
type tcpEndpoint struct {
	network string
	address string
}

func (e tcpEndpoint) Network() string {
	return e.network
}

func (e tcpEndpoint) Address() string {
	return e.address
}

func (e tcpEndpoint) DialTarget() string {
	return e.address
}

func (e tcpEndpoint) Free() error {
	return nil
}

func (e tcpEndpoint) String() string {
	return fmt.Sprintf("%s://%s", e.network, e.address)
}

func (e tcpEndpoint) Accepting(timeout time.Duration) bool {
	conn, err := net.DialTimeout(e.network, e.address, timeout)
	if err != nil {
		return false
	}

	_ = conn.Close()

	return true
}

// NewTCPEndpoint reserves a loopback port. Loopback only: the protocol carries
// provider credentials and has no TLS.
func NewTCPEndpoint() (Endpoint, error) {
	listener, err := net.Listen(networkTCP, net.JoinHostPort(loopbackHost, "0"))
	if err != nil {
		return nil, fmt.Errorf("failed to bind TCP port: %w", err)
	}

	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		return nil, fmt.Errorf("failed to close listener: %w", err)
	}

	return tcpEndpoint{network: networkTCP, address: address}, nil
}

// unixEndpoint is a socket in a temporary directory it owns.
type unixEndpoint struct {
	tcpEndpoint

	socketDir string
}

// NewUnixEndpoint creates a Unix socket endpoint in a directory of its own under tmpDir.
func NewUnixEndpoint(tmpDir string) (Endpoint, error) {
	if tmpDir != "" {
		if err := os.MkdirAll(tmpDir, socketDirMode); err != nil {
			return nil, fmt.Errorf("failed to create temp dir %s: %w", tmpDir, err)
		}
	}

	dir, err := os.MkdirTemp(tmpDir, socketDirPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	return &unixEndpoint{
		tcpEndpoint: tcpEndpoint{
			network: networkUnix,
			address: filepath.Join(dir, socketFileName),
		},
		socketDir: dir,
	}, nil
}

func (e *unixEndpoint) DialTarget() string {
	return "unix://" + e.address
}

func (e *unixEndpoint) Free() error {
	if e.socketDir == "" {
		return nil
	}

	if err := os.RemoveAll(e.socketDir); err != nil {
		return fmt.Errorf("failed to remove socket directory %s: %w", e.socketDir, err)
	}

	return nil
}
