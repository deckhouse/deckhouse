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
)

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

	return func() Config { return config }
}

func ServeArgs(network, address string) []string {
	return []string{
		ServeCommand,
		"--" + NetworkFlag + "=" + network,
		"--" + AddressFlag + "=" + address,
	}
}
