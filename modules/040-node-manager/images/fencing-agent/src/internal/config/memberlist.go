package fencingconfig

import (
	"errors"
	"time"
)

type MemberlistConfig struct {
	MemberListPort       uint          `env:"MEMBERLIST_PORT" env-required:"true"`
	ProbeInterval        time.Duration `env:"PROBE_INTERVAL" env-default:"500ms"`
	ProbeTimeout         time.Duration `env:"PROBE_TIMEOUT" env-default:"200ms"`
	SuspicionMult        uint          `env:"SUSPICION_MULT" env-default:"2"`
	IndirectChecks       uint          `env:"INDIRECT_CHECKS" env-default:"3"`
	GossipInterval       time.Duration `env:"GOSSIP_INTERVAL" env-default:"200ms"`
	RetransmitMult       uint          `env:"RETRANSMIT_MULT" env-default:"4"`
	GossipToTheDeadTime  time.Duration `env:"GOSSIP_TO_THE_DEAD_TIME" env-default:"2s"`
	MinEventIntervalJoin time.Duration `env:"MIN_EVENT_INTERVAL_JOIN" env-default:"200ms"`
	MinEventIntervalLeft time.Duration `env:"MIN_EVENT_INTERVAL_LEFT" env-default:"600ms"`
}

func (mlc *MemberlistConfig) validate() error {
	if mlc.MemberListPort == 0 {
		return errors.New("MEMBERLIST_PORT env var is empty")
	}

	if mlc.SuspicionMult == 0 {
		return errors.New("SUSPICION_MULT env var must be greater than zero")
	}

	if mlc.ProbeTimeout >= mlc.ProbeInterval {
		return errors.New("PROBE_TIMEOUT env var must be less than probe interval")
	}
	return nil
}
