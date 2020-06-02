package server

import (
	"fmt"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
)

type MetricsServer struct {
	srv *http.Server
}

func NewMetricsServer() *MetricsServer {
	return &MetricsServer{srv: &http.Server{}}
}

func (m *MetricsServer) Start(address string, errorCh chan error) {
	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, `<!DOCTYPE html>
			<title>Protobuf Exporter</title>
			<h1>Protobuf Exporter</h1>
			<p><a href=%q>Metrics</a></p>`,
			"/metrics")
		if err != nil {
			log.Warnf("Error while sending a response for the '/' path: %v", err)
			return
		}
	})

	listener, err := net.Listen("tcp", address)
	if err != nil {
		errorCh <- err
		return
	}

	log.Infof("Start exporting metrics on %q", address)
	errorCh <- m.srv.Serve(listener)
}

func (m *MetricsServer) Close() {
	_ = m.srv.Close()
}
