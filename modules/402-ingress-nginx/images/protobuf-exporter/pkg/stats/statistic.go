package stats

import "github.com/prometheus/client_golang/prometheus"

var (
	Messages = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "protobuf_exporter_messages_total",
			Help: "The total number of metric messages seen.",
		},
		[]string{"type"},
	)
	Errors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "protobuf_exporter_errors_total",
			Help: "The number of errors encountered.",
		},
		[]string{"type"},
	)
)

func init() {
	prometheus.MustRegister(Messages)
	prometheus.MustRegister(Errors)
}
