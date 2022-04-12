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

package remotewrite

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/prometheus/prompb"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/exporters/metric/cortex"

	"d8.io/upmeter/pkg/check"
	v1 "d8.io/upmeter/pkg/crd/v1"
	"d8.io/upmeter/pkg/db/dao"
	"d8.io/upmeter/pkg/util"
)

var ErrSkip = fmt.Errorf("skip export")

// syncer links puller and exporter via channel in exporter
type syncer struct {
	syncID   SyncIdentifier
	slotSize time.Duration
	labels   []*prompb.Label

	storage  *storage // adds and gets episodes
	exporter *exporter

	period time.Duration // for pulling and pushing
	logger *log.Entry
	cancel context.CancelFunc
}

func newSyncer(cfg exportingConfig, period time.Duration, storage *storage, logger *log.Entry) *syncer {
	exporter := &exporter{
		config: *cfg.exporterConfig,
	}

	syncID := cfg.ID()

	syncer := &syncer{
		syncID:   syncID,
		slotSize: cfg.slotSize,
		labels:   cfg.labels,

		storage:  storage,
		exporter: exporter,

		period: period,
		logger: logger.WithField("syncID", syncID),
	}

	return syncer
}

func (s *syncer) start(ctx context.Context) error {
	if s.cancel != nil {
		return fmt.Errorf("already started")
	}

	ctx, s.cancel = context.WithCancel(ctx)

	go s.exportLoop(ctx)
	go s.cleanupLoop(ctx)

	return nil
}

func (s *syncer) stop() {
	if !s.isRunning() {
		return
	}
	s.cancel()
	s.cancel = nil
}

func (s *syncer) isRunning() bool {
	return s.cancel != nil
}

func (s *syncer) exportLoop(ctx context.Context) {
	ticker := time.NewTicker(s.period)

	for {
		select {
		case <-ticker.C:
			err := s.export(ctx)
			if err != nil && err != ErrSkip {
				s.logger.Errorln(err)
			}
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func (s *syncer) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(s.period)

	dayBack := -24 * time.Hour

	for {
		select {
		case <-ticker.C:
			deadline := time.Now().Truncate(s.period).Add(dayBack)
			err := s.storage.Delete(s.syncID, deadline)
			if err != nil {
				log.Errorf("cannot clean old episodes: %v", err)
			}
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func (s *syncer) export(ctx context.Context) error {
	// Get
	timeseries, slot, err := s.getTimeseries()
	if err == ErrSkip {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot get timeseries: %v", err)
	}

	if s.logger.Level == log.DebugLevel {
		// The logger prints "\n" symbols and thus makes the output unreadable
		fmt.Println(stringifyTimeseries(timeseries, string(s.syncID)))
	}

	// Send to the remote storage
	err = s.exporter.Export(ctx, timeseries)
	if err != nil {
		return fmt.Errorf("cannot export: %v", err)
	}

	s.logger.Debugf("exported timeseries %s", slot.Format("15:04:05"))

	// Delete from the database
	err = s.storage.Delete(s.syncID, slot)
	if err != nil {
		return fmt.Errorf("cannot delete exported episodes %v: %v", slot.Format("15:04:05"), err)
	}

	s.logger.Debugf("cleaned exported episodes %s", slot.Format("15:04:05"))

	return nil
}

func (s *syncer) getTimeseries() ([]*prompb.TimeSeries, time.Time, error) {
	var timestamp time.Time

	episodes, err := s.storage.Get(s.syncID)
	if err == dao.ErrNotFound {
		return nil, timestamp, ErrSkip
	}
	if err != nil {
		return nil, timestamp, err
	}

	// Skip incomplete slots. Send only data from two slots ago and earlier.
	//  - Current timestamp is incomplete.
	//  - One timestamp ago is also incomplete, because the last 30s are sent after it finishes.
	//  - Two slots ago should be complete.
	timestamp = episodes[0].TimeSlot
	twoSlotsAgo := time.Now().Truncate(s.slotSize).Add(-2 * s.slotSize)
	if timestamp.After(twoSlotsAgo) {
		return nil, timestamp, ErrSkip
	}
	s.logger.Debugf("got %d episodes", len(episodes))

	timeseries := convEpisodes2Timeseries(timestamp, episodes, s.labels)

	return timeseries, timestamp, nil
}

func (s *syncer) Add(origin string, episodes []*check.Episode) error {
	return s.storage.Add(s.syncID, origin, episodes)
}

// exportingConfig is the configuration of metrics exporting
type exportingConfig struct {
	exporterConfig *cortex.Config
	labels         []*prompb.Label
	slotSize       time.Duration
}

func newExportConfig(rw *v1.RemoteWrite) exportingConfig {
	var labels []*prompb.Label
	for k, v := range rw.Spec.AdditionalLabels {
		labels = append(labels, &prompb.Label{
			Name:  k,
			Value: v,
		})
	}

	return exportingConfig{
		exporterConfig: &cortex.Config{
			Name:        rw.Name,
			Endpoint:    rw.Spec.Config.Endpoint,
			BasicAuth:   rw.Spec.Config.BasicAuth,
			BearerToken: rw.Spec.Config.BearerToken,
			Headers: map[string]string{
				"User-Agent": util.ServerUserAgent,
			},
		},
		slotSize: time.Duration(rw.Spec.IntervalSeconds) * time.Second,
		labels:   labels,
	}
}

func (c *exportingConfig) ID() SyncIdentifier {
	var (
		name     = c.exporterConfig.Name
		slotSize = c.slotSize
	)

	return SyncIdentifier(name + "-" + slotSize.String())
}

// syncers manages the dynamic collection of syncers
type syncers struct {
	mu sync.RWMutex

	// Key is syncer name, not ID. This lets us change sync period which affects the ID.
	syncers map[string]*syncer

	period  time.Duration
	storage *storage

	logger *log.Entry
}

func newSyncers(storage *storage, period time.Duration, logger *log.Entry) *syncers {
	return &syncers{
		syncers: make(map[string]*syncer),
		logger:  logger,
		period:  period,
		storage: storage,
	}
}

func (sc *syncers) start(ctx context.Context) error {
	for _, s := range sc.syncers {
		if s.isRunning() {
			continue
		}

		err := s.start(ctx)
		if err != nil {
			return err
		}

	}
	return nil
}

func (sc *syncers) stop() {
	for _, s := range sc.syncers {
		s.stop()
	}
}

// Add adds syncer from exportingConfig
func (sc *syncers) Add(ctx context.Context, config exportingConfig) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	return sc.add(ctx, config)
}

// Delete removes syncer
func (sc *syncers) Delete(config exportingConfig) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	name := config.exporterConfig.Name
	sc.delete(name)
}

// add does not maintain lock
func (sc *syncers) add(ctx context.Context, config exportingConfig) error {
	name := config.exporterConfig.Name

	sc.delete(name)

	logger := sc.logger.WithField("who", "syncer").WithField("name", name)
	syncer := newSyncer(config, sc.period, sc.storage, logger)
	sc.syncers[name] = syncer

	err := syncer.start(ctx)
	if err != nil {
		return fmt.Errorf("cannot start syncer %q: %v", name, err)
	}
	return nil
}

// delete does not maintain lock
func (sc *syncers) delete(name string) {
	syncer, ok := sc.syncers[name]
	if !ok {
		return
	}
	syncer.stop()
	delete(sc.syncers, name)
}

func (sc *syncers) AddEpisodes(origin string, episodes []*check.Episode, slotSize time.Duration) error {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	for _, syncer := range sc.syncers {
		// distinguish 30s and 5m
		if syncer.slotSize != slotSize {
			continue
		}
		err := syncer.Add(origin, episodes)
		if err != nil {
			return err
		}
	}

	return nil
}
