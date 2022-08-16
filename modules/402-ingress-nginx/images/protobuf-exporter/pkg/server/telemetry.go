/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	mproto "github.com/flant/protobuf_exporter/pkg/proto"
	"github.com/flant/protobuf_exporter/pkg/stats"
	"github.com/flant/protobuf_exporter/pkg/vault"
	pio "github.com/gogo/protobuf/io"
	"github.com/gogo/protobuf/proto"
	"github.com/prometheus/common/log"
)

// Markers are used as first byte of message to detect metric type because lua-protobuf doesn't support oneof streaming
const (
	HistogramMarker = byte(1)
	GaugeMarker     = byte(2)
	CounterMarker   = byte(3)
)

type TelemetryServer struct {
	ctx      context.Context
	stopFunc context.CancelFunc
	vault    *vault.MetricsVault

	messageProcessor *telemetryMessageProcessor
}

func NewTelemetryServer(vault *vault.MetricsVault) *TelemetryServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &TelemetryServer{
		ctx:              ctx,
		stopFunc:         cancel,
		vault:            vault,
		messageProcessor: newTelemetryMessageProcessor(),
	}
}

func (s *TelemetryServer) Start(address string, errorCh chan error) {
	err := s.messageProcessor.LoadConfig(s.ctx)
	if err != nil {
		errorCh <- fmt.Errorf("unable to Load config: %v", err)
		return
	}

	ln, err := net.Listen("tcp", address)
	if err != nil {
		errorCh <- fmt.Errorf("unable to create TCP listener: %v", err)
		return
	}

	go func() {
		<-s.ctx.Done()
		_ = ln.Close()
	}()
	log.Infof("Start listening telemetry on %q", address)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if strings.HasSuffix(err.Error(), "use of closed network connection") {
				return
			}
			errorCh <- fmt.Errorf("acceptTCP failed: %v", err)
		}
		go s.handleConn(conn.(*net.TCPConn))
	}
}

func (s *TelemetryServer) Close() {
	s.stopFunc()
}

func (s *TelemetryServer) handleConn(c *net.TCPConn) {
	defer c.Close()

	r := bufio.NewReader(c)
	readerCloser := pio.NewDelimitedReader(r, 64000)

	for {
		marker, err := r.ReadByte()
		if err != nil {
			if err != io.EOF {
				log.Warnf("can't read the first byte (marker): %v", err)
				stats.Errors.WithLabelValues("read-marker").Inc()
			}
			break
		}

		switch marker {
		case CounterMarker:
			var message mproto.CounterMessage
			readMessage(readerCloser, &message)

			if s.isMessagedDiscarded(message.Annotations) {
				continue
			}

			err := s.vault.StoreCounter(int(message.MappingIndex), message.Labels, message.Value)
			if err != nil {
				stats.Errors.WithLabelValues("wrong-mapping").Inc()
			} else {
				stats.Messages.WithLabelValues("counter").Inc()
			}
		case GaugeMarker:
			var message mproto.GaugeMessage
			readMessage(readerCloser, &message)

			if s.isMessagedDiscarded(message.Annotations) {
				continue
			}

			err := s.vault.StoreGauge(int(message.MappingIndex), message.Labels, message.Value)
			if err != nil {
				stats.Errors.WithLabelValues("wrong-mapping").Inc()
			} else {
				stats.Messages.WithLabelValues("gauge").Inc()
			}
		case HistogramMarker:
			var message mproto.HistogramMessage
			readMessage(readerCloser, &message)

			if s.isMessagedDiscarded(message.Annotations) {
				continue
			}

			buckets := make(map[float64]uint64, len(message.Buckets))
			for key, value := range message.Buckets {
				bucketNumber, err := strconv.ParseFloat(key, 64)
				if err != nil {
					log.Warnf("Wrong bucket value: %s %v", key, err)
					stats.Errors.WithLabelValues("wrong-bucket-value").Inc()
					return
				}
				buckets[bucketNumber] = value
			}

			err = s.vault.StoreHistogram(int(message.MappingIndex), message.Labels, message.Count, message.Sum, buckets)
			if err != nil {
				stats.Errors.WithLabelValues("wrong-mapping").Inc()
			} else {
				stats.Messages.WithLabelValues("histogram").Inc()
			}
		default:
			log.Warnf("protocol error: unknown metric marker: %v", marker)
			stats.Errors.WithLabelValues("unknown-marker").Inc()
			return
		}
	}
}

func (s *TelemetryServer) isMessagedDiscarded(annotation map[string]string) bool {
	return s.messageProcessor.discardProcessor.IsDiscarded(annotation)
}

func readMessage(closer pio.Reader, message proto.Message) {
	err := closer.ReadMsg(message)
	if err != nil {
		if err != io.EOF {
			log.Warnln(err)
			stats.Errors.WithLabelValues("read-message").Inc()
		}
		return
	}
	if len(message.String()) == 0 {
		log.Warnln("empty message received")
		stats.Errors.WithLabelValues("empty-message").Inc()
	}
	log.Debug(message.String())
}
