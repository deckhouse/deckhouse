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

package server

import (
	"fmt"
	"log/slog"
	"regexp"
	"slices"

	"google.golang.org/grpc"
)

const (
	// MaxMessageSize is the limit the protocol mandates in each direction.
	// gRPC's own 4 MiB default is too small for a payload carrying every
	// NodeGroup, InstanceClass and credential Secret of a cluster.
	MaxMessageSize = 8 * 1024 * 1024
	DefaultNetwork = "tcp"
	DefaultAddress = "127.0.0.1:0"

	// How a caller starts a validator binary. Both sides read these names from
	// here: the caller through Args, the validator through RegisterFlags.
	ServeCommand = "serve"
	NetworkFlag  = "network"
	AddressFlag  = "address"

	// ListeningPrefix is the prefix for a log message that a validator writes once
	// it has bound its endpoint. The caller reads it from the process output to
	// learn where to dial, so the format below is part of the protocol.
	//
	// There is a single contract expressed in two forms: what the validator writes
	// and how the caller reads it back. Keep them in sync.
	// The [[ ]] markers distinguish the endpoint from the surrounding log text.
	// The prefix contains no regexp metacharacters, so both lines are visually
	// consistent and easy to read.
	ListeningPrefix  = "dhctl-provider-protocol: listening on "
	listeningFormat  = ListeningPrefix + "[[%s://%s]]"
	listeningPattern = ListeningPrefix + `\[\[([a-z]+)://([^]]+)]]`
)

var listeningRegexp = regexp.MustCompile(listeningPattern)

type Config struct {
	Network     string
	Address     string
	Logger      *slog.Logger
	GRPCOptions []grpc.ServerOption
}

func NewConfig() Config {
	return Config{
		Network: DefaultNetwork,
		Address: DefaultAddress,
		Logger:  slog.Default(),
		GRPCOptions: []grpc.ServerOption{
			grpc.MaxRecvMsgSize(MaxMessageSize),
			grpc.MaxSendMsgSize(MaxMessageSize),
		},
	}
}

func (c Config) Validate() error {
	if c.Network == "" {
		return fmt.Errorf("network is required")
	}

	if c.Address == "" {
		return fmt.Errorf("address is required")
	}

	if c.Logger == nil {
		return fmt.Errorf("logger is required")
	}

	return nil
}

func (c Config) Merge(other Config) Config {
	if other.Network != "" {
		c.Network = other.Network
	}

	if other.Address != "" {
		c.Address = other.Address
	}

	if other.Logger != nil {
		c.Logger = other.Logger
	}

	if len(other.GRPCOptions) > 0 {
		c.GRPCOptions = slices.Concat(c.GRPCOptions, other.GRPCOptions)
	}
	return c
}

type FlagSet interface {
	StringVar(p *string, name string, value string, usage string)
}

type ConfigGetter func() Config

func ConfigGetterFromFlags(fs FlagSet) ConfigGetter {
	var config Config

	fs.StringVar(&config.Network, NetworkFlag, DefaultNetwork,
		"network to serve on: unix or tcp")
	fs.StringVar(&config.Address, AddressFlag, DefaultAddress,
		"address to serve on: host:port, or a socket path when --network=unix")

	return func() Config { return NewConfig().Merge(config) }
}

func ServeArgs(network, address string) []string {
	return []string{
		ServeCommand,
		"--" + NetworkFlag + "=" + network,
		"--" + AddressFlag + "=" + address,
	}
}

// ListeningLine is what a validator announces: the network and the address it bound,
// which is not what the caller asked for when it asked for port 0.
func ListeningLine(network, address string) string {
	return fmt.Sprintf(listeningFormat, network, address)
}

// ParseListeningLine reads back what ListeningLine wrote. The announcement goes
// through the validator's own logger, so by the time the caller sees it the line may
// be wrapped in JSON, logfmt or a timestamp: the prefix is looked for anywhere in the
// line and the endpoint ends where the log format resumes.
//
//nolint:nonamedreturns // named return values serve as documentation for the caller
func ParseListeningLine(line string) (network, address string, ok bool) {
	match := listeningRegexp.FindStringSubmatch(line)
	if match == nil {
		return "", "", false
	}
	return match[1], match[2], true
}
