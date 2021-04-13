package sender

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/check"
	"upmeter/pkg/probe/util"
	"upmeter/pkg/upmeter/api"
)

const MaxBatchSize = 64

type Sender struct {
	client *UpmeterClient

	recv       chan []check.Episode
	bufferLock sync.RWMutex
	buffer     []check.Episode

	cancel context.CancelFunc
}

func New(client *UpmeterClient, recv chan []check.Episode) *Sender {
	s := &Sender{
		client: client,
		recv:   recv,
		buffer: make([]check.Episode, 0),
	}
	return s
}

// Start runs two go routines to behave as a naive WAL:
// 1. All results are placed in a Buffer.
// 2. The result is deleted from the buffer after successful submission to Upmeter.
func (s *Sender) Start(ctx context.Context) {
	go s.receiveLoop(ctx)
	go s.sendLoop(ctx)
}

// buffer writer
func (s *Sender) receiveLoop(ctx context.Context) {
	for {
		select {
		case episodes := <-s.recv:
			s.bufferLock.Lock()
			// TODO buffer should be persistent — sqlite or something
			s.buffer = append(s.buffer, episodes...)
			s.bufferLock.Unlock()
		case <-ctx.Done():
			log.Info("Sender: stop episode receiver")
			return
		}
	}
}

// A buffer reader and sender.
// Try to send a Buffer every 5 sec.
func (s *Sender) sendLoop(ctx context.Context) {
	tick := time.NewTicker(5 * time.Second)

	for {
		select {
		case <-tick.C:
			err := s.Send()
			if err != nil {
				log.Errorf("sending failed: %v", err)
			}
		case <-ctx.Done():
			log.Info("Sender: stop episodes sender")
			return
		}
	}
}

// Naive WAL implementation
// Check Buffer and send no more than 64 items at once.
func (s *Sender) Send() error {
	// TODO add loop to send a whole buffer, not just maximum batch size.

	batch := s.getBatch()
	if batch == nil {
		// nothing to send
		return nil
	}

	err := s.sendBatch(batch)
	if err != nil {
		return err
	}

	s.cleanBuffer(len(batch))

	return nil
}

func (s *Sender) sendBatch(batch []check.Episode) error {
	data := api.EpisodesPayload{
		Origin:   util.AgentUniqueId(),
		Episodes: batch,
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

func (s *Sender) getBatch() []check.Episode {
	s.bufferLock.RLock()
	defer s.bufferLock.RUnlock()

	// decide of batch size
	var bufSize = len(s.buffer)
	if len(s.buffer) == 0 {
		return nil
	}
	var batchSize = MaxBatchSize
	if bufSize < MaxBatchSize {
		batchSize = bufSize
	}

	// copy results to not lock a buffer for http writing
	var batch = make([]check.Episode, batchSize)
	for i := 0; i < batchSize; i++ {
		batch[i] = s.buffer[i]
	}

	return batch
}

// cleanBuffer removes batch items from Buffer
func (s *Sender) cleanBuffer(size int) {
	s.bufferLock.Lock()
	defer s.bufferLock.Unlock()

	if len(s.buffer) == size {
		// Recreate a Buffer as a "fresh" array if all items were sent
		s.buffer = make([]check.Episode, 0)
	} else {
		// We don't expect to have argument "size" bigger than the buffer size
		s.buffer = s.buffer[size:len(s.buffer)]
	}
}
