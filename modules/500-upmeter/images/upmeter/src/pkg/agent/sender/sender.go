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
	"sync/atomic"
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

	// maxEpisodeAgeSeconds is set from the server's ack on each successful POST.
	// 0 means "server hasn't told us yet" — the agent then keeps everything until
	// the first ack arrives. After the first ack, this drives stale-tail cutoff.
	maxEpisodeAgeSeconds atomic.Int64

	stop chan struct{}
	done chan struct{}
}

func New(client *Client, recv chan []check.Episode, storage *ListStorage, interval time.Duration) *Sender {
	s := &Sender{
		client:   client,
		recv:     recv,
		storage:  storage,
		interval: interval,

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
	episodes, err := s.storage.List()
	if err != nil {
		return err
	}
	if len(episodes) == 0 {
		// nothing to send, it is fine
		return nil
	}

	slot := episodes[0].TimeSlot

	// Drop the stale prefix of the local WAL in a single SQL call. Without this, after a long
	// outage the agent would replay every old slot one-by-one (1 slot per --export-interval
	// tick), starving fresh data from reaching the server and breaking the status API in the UI.
	// The threshold is pushed by the server in the previous POST's ack; before the first ack
	// arrives we keep everything (safe: at worst one stale slot leaks through and gets cut on
	// the server side by the per-CR maxSampleAgeSeconds in syncer).
	if maxAge := s.serverMaxEpisodeAge(); maxAge > 0 {
		ageDeadline := time.Now().Add(-maxAge)
		if slot.Before(ageDeadline) {
			log.Errorf("dropping stale episodes up to %s (older than %s, hinted by server)", ageDeadline.Format(time.RFC3339), maxAge)
			if err := s.storage.Clean(ageDeadline); err != nil {
				return fmt.Errorf("dropping stale episodes: %w", err)
			}
			return nil
		}
	}

	err = s.send(episodes)
	if err != nil {
		return err
	}

	err = s.storage.Clean(slot)
	if err != nil {
		return fmt.Errorf("cleaning send storage, slot=%v: %v", slot, err)
	}
	return nil
}

// serverMaxEpisodeAge returns the cutoff received from the server's last ack.
// Returns 0 if the server has not provided a hint yet (no ingestion happens in that case).
func (s *Sender) serverMaxEpisodeAge() time.Duration {
	secs := s.maxEpisodeAgeSeconds.Load()
	if secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

func (s *Sender) send(episodes []check.Episode) error {
	data := api.EpisodesPayload{
		Origin:   run.ID(),
		Episodes: episodes,
	}

	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshalling to JSON: %v", err)
	}

	respBody, err := s.client.Send(body)
	if err != nil {
		return err
	}

	if len(respBody) > 0 {
		var ack api.EpisodesAck
		if jerr := json.Unmarshal(respBody, &ack); jerr == nil && ack.MaxEpisodeAgeSeconds > 0 {
			s.maxEpisodeAgeSeconds.Store(ack.MaxEpisodeAgeSeconds)
		}
	}
	return nil
}

func (s *Sender) Stop() {
	close(s.stop)

	<-s.done
	<-s.done
	<-s.done
}
