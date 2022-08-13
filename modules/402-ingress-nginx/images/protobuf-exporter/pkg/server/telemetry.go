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
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	pio "github.com/gogo/protobuf/io"
	"github.com/gogo/protobuf/proto"
	"github.com/prometheus/common/log"
	"gopkg.in/yaml.v3"

	mproto "github.com/flant/protobuf_exporter/pkg/proto"
	"github.com/flant/protobuf_exporter/pkg/stats"
	"github.com/flant/protobuf_exporter/pkg/vault"
)

// Markers are used as first byte of message to detect metric type because lua-protobuf doesn't support oneof streaming
const (
	HistogramMarker = byte(1)
	GaugeMarker     = byte(2)
	CounterMarker   = byte(3)

	excludeLabelsFile = "/var/files/exclude_resources.yml"
)

type TelemetryServer struct {
	vault    *vault.MetricsVault
	stopChan chan struct{}

	m                  sync.RWMutex
	excludedNamespaces map[string]struct{}
	excludedIngresses  map[string]struct{}
}

func NewTelemetryServer(vault *vault.MetricsVault) *TelemetryServer {
	return &TelemetryServer{
		vault:              vault,
		excludedNamespaces: make(map[string]struct{}),
		excludedIngresses:  make(map[string]struct{}),
		stopChan:           make(chan struct{}),
	}
}

func (s *TelemetryServer) Start(address string, errorCh chan error) {
	ln, err := net.Listen("tcp", address)
	if err != nil {
		errorCh <- fmt.Errorf("unable to create TCP listener: %v", err)
		return
	}

	s.parseFileWithExcludes()
	go s.runFileWatcher()

	go func() {
		<-s.stopChan
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
	s.stopChan <- struct{}{}
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

			if s.isResourceExcluded(message.NamespacedIngress) {
				fmt.Println("excluded", message.NamespacedIngress)
				continue
			}
			fmt.Println("is ok", message.NamespacedIngress)

			err := s.vault.StoreCounter(int(message.MappingIndex), message.Labels, message.Value)
			if err != nil {
				stats.Errors.WithLabelValues("wrong-mapping").Inc()
			} else {
				stats.Messages.WithLabelValues("counter").Inc()
			}
		case GaugeMarker:
			var message mproto.GaugeMessage
			readMessage(readerCloser, &message)

			if s.isResourceExcluded(message.NamespacedIngress) {
				fmt.Println("excluded", message.NamespacedIngress)
				continue
			}
			fmt.Println("is ok", message.NamespacedIngress)

			err := s.vault.StoreGauge(int(message.MappingIndex), message.Labels, message.Value)
			if err != nil {
				stats.Errors.WithLabelValues("wrong-mapping").Inc()
			} else {
				stats.Messages.WithLabelValues("gauge").Inc()
			}
		case HistogramMarker:
			var message mproto.HistogramMessage
			readMessage(readerCloser, &message)

			if s.isResourceExcluded(message.NamespacedIngress) {
				fmt.Println("excluded", message.NamespacedIngress)
				continue
			}
			fmt.Println("is ok", message.NamespacedIngress)

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

func (s *TelemetryServer) isResourceExcluded(namespacedIngress string) bool {
	pair := strings.Split(namespacedIngress, ":")
	if len(pair) != 2 {
		return false
	}
	ns := pair[0]

	s.m.RLock()
	defer s.m.RUnlock()

	fmt.Println("EX ING", s.excludedIngresses)
	if len(s.excludedIngresses) > 0 {
		if _, ok := s.excludedIngresses[namespacedIngress]; ok {
			fmt.Println("IS ING", namespacedIngress)
			return true
		}
	}

	fmt.Println("EX NS", s.excludedNamespaces)
	if len(s.excludedNamespaces) > 0 {
		if _, ok := s.excludedNamespaces[ns]; ok {
			fmt.Println("IS NS", ns)
			return true
		}
	}

	return false
}

func (s *TelemetryServer) runFileWatcher() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("start file watcher failed: %s", err)
	}
	defer watcher.Close()

	err = watcher.Add(excludeLabelsFile)
	if err != nil {
		log.Fatalf("add watcher for file failed: %s", err)
	}

	for {
		select {
		case event := <-watcher.Events:
			if event.Op == fsnotify.Remove {
				// k8s configmaps use symlinks,
				// old file is deleted and a new link with the same name is created
				_ = watcher.Remove(event.Name)
				err = watcher.Add(event.Name)
				if err != nil {
					log.Fatal(err)
				}
				switch event.Name {
				case excludeLabelsFile:
					go s.parseFileWithExcludes()
				}
			}

		case err := <-watcher.Errors:
			log.Errorf("watch files error: %s", err)

		case <-s.stopChan:
			return
		}
	}
}

func (s *TelemetryServer) parseFileWithExcludes() {
	f, err := os.Open(excludeLabelsFile) // reader
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	var exc excludes

	err = yaml.NewDecoder(f).Decode(&exc)
	if err != nil && err != io.EOF {
		log.Fatal(err)
	}

	log.Infof("Exclude metrics from namespaces: %v", exc.Namespaces)
	log.Infof("Exclude metrics from ingresses: %v", exc.Ingresses)

	s.m.Lock()
	if len(exc.Namespaces) == 0 {
		s.excludedNamespaces = make(map[string]struct{})
	} else {
		for _, ns := range exc.Namespaces {
			s.excludedNamespaces[ns] = struct{}{}
		}
	}

	if len(exc.Ingresses) == 0 {
		s.excludedIngresses = make(map[string]struct{})
	} else {
		for _, ing := range exc.Ingresses {
			s.excludedIngresses[ing] = struct{}{}
		}
	}
	s.m.Unlock()
}

type excludes struct {
	Namespaces []string `json:"namespaces" yaml:"namespaces"`
	Ingresses  []string `json:"ingresses" yaml:"ingresses"`
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
