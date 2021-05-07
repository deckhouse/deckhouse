package sender

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/probe/util"
	"d8.io/upmeter/pkg/server/api"
)

type Sender struct {
	client  *Client
	recv    chan []check.Episode
	storage *ListStorage
	period  time.Duration
}

func New(client *Client, recv chan []check.Episode, storage *ListStorage, period time.Duration) *Sender {
	s := &Sender{
		client:  client,
		recv:    recv,
		storage: storage,
		period:  period,
	}
	return s
}

func (s *Sender) Start(ctx context.Context) {
	go s.receiveLoop(ctx)
	go s.sendLoop(ctx)
	go s.cleanupLoop(ctx)
}

// buffer writer
func (s *Sender) receiveLoop(ctx context.Context) {
	for {
		select {
		case episodes := <-s.recv:
			err := s.storage.Save(episodes)
			if err != nil {
				log.Fatalf("cannot save episodes to storage: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Sender) sendLoop(ctx context.Context) {
	tick := time.NewTicker(s.period)

	for {
		select {
		case <-tick.C:
			err := s.export()
			if err != nil {
				log.Errorf("cannot export episodes: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Sender) cleanupLoop(ctx context.Context) {
	tick := time.NewTicker(s.period)
	dayBack := -24 * time.Hour

	for {
		select {
		case <-tick.C:
			deadline := time.Now().Truncate(s.period).Add(dayBack)
			err := s.storage.Clean(deadline)
			if err != nil {
				log.Errorf("cannot clean old episodes: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Sender) export() error {
	episodes, err := s.storage.List()
	if err != nil {
		return err
	}
	if episodes == nil {
		// nothing to send, it is fine
		return nil
	}

	err = s.send(episodes)
	if err != nil {
		return err
	}

	slot := episodes[0].TimeSlot
	err = s.storage.Clean(slot)
	if err != nil {
		return fmt.Errorf("cannot clean storage for slot=%v: %v", slot, err)
	}
	return nil
}

func (s *Sender) send(episodes []check.Episode) error {
	data := api.EpisodesPayload{
		Origin:   util.AgentUniqueId(),
		Episodes: episodes,
	}

	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal payload to JSON: %v", err)
	}

	err = s.client.Send(body)
	if err != nil {
		return fmt.Errorf("failed to send data: %v", err)
	}
	return nil
}
