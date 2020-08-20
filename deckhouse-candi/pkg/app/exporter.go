package app

import (
	"time"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	MetricsPath   = "/metrics"
	ListenAddress = ":9101"
	CheckInterval = time.Minute
)

func DefineConvergeExporterFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("metrics-path", "Path to export metrics").
		StringVar(&MetricsPath)
	cmd.Flag("listen-address", "Address to expose metrics").
		StringVar(&ListenAddress)
	cmd.Flag("check-interval", "Period to check terraform state converge").
		DurationVar(&CheckInterval)
}
