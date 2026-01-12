package fencing_config

import (
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	WatchdogConfig   WatchdogConfig
	MemberlistConfig MemberlistConfig
	// TODO decide: create structures for kube-api and node
	KubernetesAPICheckInterval time.Duration `env:"KUBERNETES_API_CHECK_INTERVAL" env-default:"5s"`
	KubernetesAPITimeout       time.Duration `env:"KUBERNETES_API_TIMEOUT" env-default:"10s"`
	APIIsAvailableMsgInterval  time.Duration `env:"API_IS_AVAILABLE_MSG_INTERVAL" env-default:"90s"`
	HealthProbeBindAddress     string        `env:"HEALTH_PROBE_BIND_ADDRESS"  env-default:":8081"`
	NodeName                   string        `env:"NODE_NAME"`
	NodeGroup                  string        `env:"NODE_GROUP"`
}

type WatchdogConfig struct {
	WatchdogDevice       string        `env:"WATCHDOG_DEVICE" env-default:"/dev/watchdog"`
	WatchdogFeedInterval time.Duration `env:"WATCHDOG_FEED_INTERVAL" env-default:"5s"`
}

type MemberlistConfig struct {
	MemberListPort       int           `env:"MEMBERLIST_PORT"`
	ProbeInterval        time.Duration `env:"PROBE_INTERVAL"`
	ProbeTimeout         time.Duration `env:"PROBE_TIMEOUT"`
	SuspicionMult        int           `env:"SUSPICION_MULT"`
	IndirectChecks       int           `env:"INDIRECT_CHECKS"`
	GossipInterval       time.Duration `env:"GOSSIP_INTERVAL"`
	RetransmitMult       int           `env:"RETRANSMIT_MULT"`
	GossipToTheDeadTime  time.Duration `env:"GOSSIP_TO_THE_DEAD_TIME"`
	MinEventIntervalJoin time.Duration `env:"MIN_EVENT_INTERVAL_JOIN"`
	MinEventIntervalLeft time.Duration `env:"MIN_EVENT_INTERVAL_LEFT"`
}

func (c *Config) Load() error {
	err := cleanenv.ReadEnv(c)
	if err != nil {
		return err
	}
	return nil
}
