package main

import (
	"bytes"
	"encoding/gob"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"

	"github.com/deckhouse/deckhouse/modules/500-operator-trivy/images/bdu-exporter/types"
)

type CVEToBDUs map[string][]string

type metricsCollector struct {
	vulns []prometheus.Metric
}

func newMetricsSlice(vulns *types.Vulnerabilities) ([]byte, error) {
	var (
		ctbs    = make(CVEToBDUs)
		metrics []prometheus.Metric
	)

	for _, v := range vulns.Vul {
		for _, id := range v.Identifiers {
			if id.Type != "CVE" {
				continue
			}

			if bdus, ok := ctbs[id.Identifier]; ok {
				bdus = append(bdus, v.Identifier)
			} else {
				ctbs[id.Identifier] = []string{v.Identifier}
			}
		}
	}

	for cve, bdus := range ctbs {
		for _, bdu := range bdus {
			metrics = append(metrics, prometheus.MustNewConstMetric(
				prometheus.NewDesc("bdu_exporter_cve", "", nil, map[string]string{
					"vuln_id": cve, "bdu_id": bdu,
				}),
				prometheus.GaugeValue, 1,
			))
		}
	}

	mc := metricsCollector{vulns: metrics}

	tempRegistry := prometheus.NewRegistry()
	tempRegistry.MustRegister(mc)
	mfs, err := tempRegistry.Gather()
	if err != nil {
		return nil, err
	}

	var metricsBuffer bytes.Buffer
	for _, mf := range mfs {
		if _, err := expfmt.MetricFamilyToText(&metricsBuffer, mf); err != nil {
			return nil, err
		}
	}

	return metricsBuffer.Bytes(), nil
}

func (m metricsCollector) Describe(_ chan<- *prometheus.Desc) {}

func (m metricsCollector) Collect(metrics chan<- prometheus.Metric) {
	for _, metric := range m.vulns {
		metrics <- metric
	}
}

func readIntoMemory(r io.Reader) (*types.Vulnerabilities, error) {
	var vulns types.Vulnerabilities

	decoder := gob.NewDecoder(r)

	err := decoder.Decode(&vulns)
	if err != nil {
		return nil, err
	}

	return &vulns, nil
}

func main() {
	file, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	vulns, err := readIntoMemory(file)
	if err != nil {
		log.Fatal(err)
	}

	_ = file.Close()

	metrics, err := newMetricsSlice(vulns)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write(metrics)
		if err != nil {
			log.Printf("failed to write metrics: %s", err)
		}
	})

	log.Fatal(http.ListenAndServe("127.0.0.1:5000", nil))
}
