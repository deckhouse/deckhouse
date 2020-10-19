package app

import (
	"time"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	MetricsPath   = "/metrics"
	ListenAddress = ":9101"
	CheckInterval = time.Minute
	OutputFormat  = "yaml"
)

func DefineConvergeExporterFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("metrics-path", "Path to export metrics").
		Envar(configEnvName("METRICS_PATH")).
		StringVar(&MetricsPath)
	cmd.Flag("listen-address", "Address to expose metrics").
		Envar(configEnvName("LISTEN_ADDRESS")).
		StringVar(&ListenAddress)
	cmd.Flag("check-interval", "Period to check terraform state converge").
		Envar(configEnvName("CHECK_INTERVAL")).
		DurationVar(&CheckInterval)
}

func DefineOutputFlag(cmd *kingpin.CmdClause) {
	cmd.Flag("output", "Output format").
		Envar(configEnvName("OUTPUT")).
		Short('o').
		EnumVar(&OutputFormat, "yaml", "json")
}
