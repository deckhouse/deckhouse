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

package sender

import (
	"encoding/json"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/probe/run"
	"d8.io/upmeter/pkg/server/api"
)

type Sender struct {
	client   *Client
	recv     chan []check.Episode
	storage  *ListStorage
	interval time.Duration

	// batchSlots limits how many earliest time slots are drained from the WAL and sent in a single
	// request. It lets the agent quickly catch up after a server downtime instead of sending one
	// 30s slot per tick. In steady state there is at most one slot pending, so a single slot is sent.
	batchSlots int

	stop chan struct{}
	done chan struct{}
}

func New(client *Client, recv chan []check.Episode, storage *ListStorage, interval time.Duration, batchSlots int) *Sender {
	if batchSlots < 1 {
		batchSlots = 1
	}

	s := &Sender{
		client:     client,
		recv:       recv,
		storage:    storage,
		interval:   interval,
		batchSlots: batchSlots,

		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	return s
}

func (s *Sender) Start() {
	go s.receiveLoop()
	go s.sendLoop()
	go s.cleanupLoop()
}

// buffer writer
func (s *Sender) receiveLoop() {
	for {
		select {
		case episodes := <-s.recv:
			err := s.storage.Save(episodes)
			if err != nil {
				log.Fatalf("cannot save episodes to storage: %v", err)
			}
		case <-s.stop:
			s.done <- struct{}{}
			return
		}
	}
}

func (s *Sender) sendLoop() {
	ticker := time.NewTicker(s.interval)

	for {
		select {
		case <-ticker.C:
			err := s.export()
			if err != nil {
				log.Errorf("sendLoop: %v", err)
			}
		case <-s.stop:
			ticker.Stop()
			s.done <- struct{}{}
			return
		}
	}
}

func (s *Sender) cleanupLoop() {
	ticker := time.NewTicker(s.interval)

	dayBack := -24 * time.Hour

	for {
		select {
		case <-ticker.C:
			deadline := time.Now().Truncate(s.interval).Add(dayBack)
			err := s.storage.Clean(deadline)
			if err != nil {
				log.Errorf("cannot clean old episodes: %v", err)
			}
		case <-s.stop:
			ticker.Stop()
			s.done <- struct{}{}
			return
		}
	}
}

func (s *Sender) export() error {
	listStart := time.Now()
	episodes, err := s.storage.List(s.batchSlots)
	listDur := time.Since(listStart)
	if err != nil {
		return err
	}
	if len(episodes) == 0 {
		// nothing to send, it is fine
		return nil
	}

	earliest, latest := slotRange(episodes)
	slots := countSlots(episodes)

	log.Infof("export batch start: episodes=%d slots=%d earliest=%s latest=%s listDur=%s",
		len(episodes), slots, fmtSlot(earliest), fmtSlot(latest), listDur)

	// The batch may span several slots, so clean up to the latest one sent.
	sendStart := time.Now()
	sentBytes, err := s.send(episodes)
	sendDur := time.Since(sendStart)
	if err != nil {
		log.Errorf("export batch failed: episodes=%d slots=%d earliest=%s latest=%s bytes=%d sendDur=%s: %v",
			len(episodes), slots, fmtSlot(earliest), fmtSlot(latest), sentBytes, sendDur, err)
		return err
	}

	cleanStart := time.Now()
	err = s.storage.Clean(latest)
	cleanDur := time.Since(cleanStart)
	if err != nil {
		return fmt.Errorf("cleaning send storage, slot=%v: %v", latest, err)
	}

	remaining, cntErr := s.storage.CountSlots()
	if cntErr != nil {
		log.Errorf("cannot count remaining backlog: %v", cntErr)
		remaining = -1
	}

	log.Infof("export batch ok: episodes=%d slots=%d earliest=%s latest=%s bytes=%d backlogSlotsRemaining=%d listDur=%s sendDur=%s cleanDur=%s totalDur=%s",
		len(episodes), slots, fmtSlot(earliest), fmtSlot(latest), sentBytes, remaining,
		listDur, sendDur, cleanDur, time.Since(listStart))
	return nil
}

// slotRange returns the earliest and the latest time slot found in the batch.
func slotRange(episodes []check.Episode) (earliest, latest time.Time) {
	earliest = episodes[0].TimeSlot
	latest = episodes[0].TimeSlot
	for _, ep := range episodes[1:] {
		if ep.TimeSlot.Before(earliest) {
			earliest = ep.TimeSlot
		}
		if ep.TimeSlot.After(latest) {
			latest = ep.TimeSlot
		}
	}
	return earliest, latest
}

// countSlots counts how many distinct time slots the batch spans.
func countSlots(episodes []check.Episode) int {
	seen := make(map[int64]struct{}, len(episodes))
	for _, ep := range episodes {
		seen[ep.TimeSlot.Unix()] = struct{}{}
	}
	return len(seen)
}

func fmtSlot(t time.Time) string {
	return t.Format("15:04:05")
}

func (s *Sender) send(episodes []check.Episode) (int, error) {
	data := api.EpisodesPayload{
		Origin:   run.ID(),
		Episodes: episodes,
	}

	body, err := json.Marshal(data)
	if err != nil {
		return 0, fmt.Errorf("marshalling to JSON: %v", err)
	}

	return len(body), s.client.Send(body)
}

func (s *Sender) Stop() {
	close(s.stop)

	<-s.done
	<-s.done
	<-s.done
}
