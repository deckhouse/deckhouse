package fencingconfig

import (
	"errors"
	"fencing-agent/internal/config/validators"
	"strings"
)

type GRPCConfig struct {
	GRPCSocketPath string `env:"GRPC_SOCKET_PATH" env-default:"/var/run/fencing-agent.sock"`
	UnaryRPS       int    `env:"REQUEST_RPS" env-default:"10"`
	UnaryBurst     int    `env:"REQUEST_BURST" env-default:"100"`
	StreamRPS      int    `env:"STREAM_RPS" env-default:"5"`
	StreamBurst    int    `env:"STREAM_BURST" env-default:"100"`
}

func (g *GRPCConfig) validate() error {
	if unaryErr := validators.ValidateRateLimit(g.UnaryRPS, g.UnaryBurst, "Unary"); unaryErr != nil {
		return unaryErr
	}

	if streamErr := validators.ValidateRateLimit(g.StreamRPS, g.StreamBurst, "Stream"); streamErr != nil {
		return streamErr
	}

	if strings.TrimSpace(g.GRPCSocketPath) == "" {
		return errors.New("GRPC_SOCKET_PATH is empty")
	}
	return nil
}
