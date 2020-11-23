package sender

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/probe/types"
)

const MaxBatchSize = 64

type Sender struct {
	DowntimeEpisodesCh chan []types.DowntimeEpisode
	BufferLock         sync.RWMutex
	Buffer             []types.DowntimeEpisode
	Client             *UpmeterClient

	ctx    context.Context
	cancel context.CancelFunc
}

func NewSender(ctx context.Context) *Sender {
	s := &Sender{
		DowntimeEpisodesCh: make(chan []types.DowntimeEpisode),
		BufferLock:         sync.RWMutex{},
		Buffer:             make([]types.DowntimeEpisode, 0),
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	return s
}

func (s *Sender) WithUpmeterClient(client *UpmeterClient) {
	s.Client = client
}

// Start runs two go routines to behave as a naive WAL:
// 1. All results are placed in a Buffer.
// 2. The result is deleted from the buffer after successful submission to Upmeter.
func (s *Sender) Start() {
	// buffer writer
	go func() {
		for {
			select {
			case episodes := <-s.DowntimeEpisodesCh:
				//log.Infof("sender got episodes via chan: %+v", episodes)
				s.BufferLock.Lock()
				//log.Infof("SENDER append %d downtime episodes from scraper", len(episodes))
				// TODO buffer should be persistent — sqlite or something
				s.Buffer = append(s.Buffer, episodes...)
				s.BufferLock.Unlock()
			case <-s.ctx.Done():
				log.Info("Sender: stop episode receiver")
				return
			}
		}
	}()

	// A buffer reader and sender.
	// Try to send a Buffer every 5 sec.
	go func() {
		sendTimer := time.NewTicker(5 * time.Second)

		for {
			select {
			case <-sendTimer.C:
				now := time.Now().Unix()
				//log.Infof("SENDER Send downtime episodes at %d", now)
				// TODO return len and catch error. Stop if error, send next batch if len > 0.
				s.SendEpisodes(now)
			case <-s.ctx.Done():
				log.Info("Sender: stop episodes sender")
				return
			}
		}
	}()
}

func (s *Sender) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// Naive WAL implementation
// Check Buffer and send no more than 64 items at once.
func (s *Sender) SendEpisodes(now int64) {
	s.BufferLock.RLock()
	var bufferLen = len(s.Buffer)
	if bufferLen == 0 {
		s.BufferLock.RUnlock()
		return
	}

	// copy results to not lock a buffer for http writing
	var batchSize = MaxBatchSize
	if bufferLen < MaxBatchSize {
		batchSize = bufferLen
	}

	var batchBuf = make([]types.DowntimeEpisode, batchSize)
	for i := 0; i < batchSize; i++ {
		batchBuf[i] = s.Buffer[i]
	}

	// unlock a buffer after copy
	s.BufferLock.RUnlock()

	err := s.Client.Send(batchBuf)
	if err != nil {
		log.Errorf("Fail sending batch of len=%d to '%s' at %d: %v", len(batchBuf), s.Client.Ip, now, err)
		return
	}
	log.Debugf("Send batch of len=%d (%d) to '%s' at %d ts", len(batchBuf), len(s.Buffer), s.Client.Ip, now)

	// TODO add loop to send a whole buffer, not just maximum batch size.

	// Get write lock and remove batchSize items from Buffer
	s.BufferLock.Lock()
	if len(s.Buffer) == batchSize {
		// Recreate a Buffer as a "fresh" array if all items were sent
		s.Buffer = make([]types.DowntimeEpisode, 0)
	} else {
		s.Buffer = s.Buffer[batchSize:len(s.Buffer)]
	}
	s.BufferLock.Unlock()
}
