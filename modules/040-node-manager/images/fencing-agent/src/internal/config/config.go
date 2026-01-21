package fencing_config

import (
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	WatchdogConfig             WatchdogConfig
	MemberlistConfig MemberlistConfig
	RLimit           RateLimit
	LogLevel         string        `env:"LOG_LEVEL" env-default:"info"`
	GRPCAddress                string        `env:"GRPC_ADDRESS" env-default:"/var/run/fencing-agent.sock"`
	KubernetesAPICheckInterval time.Duration `env:"KUBERNETES_API_CHECK_INTERVAL" env-default:"5s"`
	KubernetesAPITimeout       time.Duration `env:"KUBERNETES_API_TIMEOUT" env-default:"10s"`
	APIIsAvailableMsgInterval  time.Duration `env:"API_IS_AVAILABLE_MSG_INTERVAL" env-default:"90s"`
	HealthProbeBindAddress     string        `env:"HEALTH_PROBE_BIND_ADDRESS"  env-default:":8081"`
	NodeName                   string        `env:"NODE_NAME" env-required:"true"`
	NodeGroup                  string        `env:"NODE_GROUP" env-required:"true"`
}

// TODO discuss values
type RateLimit struct {
	UnaryRPS    int `env:"REQUEST_RPS" env-default:"10"`
	UnaryBurst  int `env:"REQUEST_BURST" env-default:"100"`
	StreamRPS   int `env:"STREAM_RPS" env-default:"5"`
	StreamBurst int `env:"STREAM_BURST" env-default:"100"`
}

type WatchdogConfig struct {
	WatchdogDevice       string        `env:"WATCHDOG_DEVICE" env-default:"/dev/watchdog"`
	WatchdogFeedInterval time.Duration `env:"WATCHDOG_FEED_INTERVAL" env-default:"5s"`
}

type MemberlistConfig struct {
	MemberListPort       int           `env:"MEMBERLIST_PORT" env-required:"true"`
	ProbeInterval        time.Duration `env:"PROBE_INTERVAL" env-default:"500ms"`
	ProbeTimeout         time.Duration `env:"PROBE_TIMEOUT" env-default:"200ms"`
	SuspicionMult        int           `env:"SUSPICION_MULT" env-default:"2"`
	IndirectChecks       int           `env:"INDIRECT_CHECKS" env-default:"3"`
	GossipInterval       time.Duration `env:"GOSSIP_INTERVAL" env-default:"200ms"`
	RetransmitMult       int           `env:"RETRANSMIT_MULT" env-default:"4"`
	GossipToTheDeadTime  time.Duration `env:"GOSSIP_TO_THE_DEAD_TIME" env-default:"2s"`
	MinEventIntervalJoin time.Duration `env:"MIN_EVENT_INTERVAL_JOIN" env-default:"200ms"`
	MinEventIntervalLeft time.Duration `env:"MIN_EVENT_INTERVAL_LEFT" env-default:"600ms"`
}

func (c *Config) MustLoad() {
	err := cleanenv.ReadEnv(c)
	if err != nil {
		panic(err)
	}
}
