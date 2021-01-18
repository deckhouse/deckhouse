package process

import (
	"sync"

	"github.com/deckhouse/deckhouse/candictl/pkg/log"
)

var DefaultSession *Session

func init() {
	DefaultSession = NewSession()
}

type Stopable interface {
	Stop()
}

type Session struct {
	Stopables []Stopable
}

func NewSession() *Session {
	return &Session{
		Stopables: make([]Stopable, 0),
	}
}

func (s *Session) Stop() {
	if s == nil {
		return
	}
	var wg sync.WaitGroup
	count := 0
	for _, stopable := range s.Stopables {
		if stopable == nil {
			continue
		}
		wg.Add(1)
		count++
		go func(s Stopable) {
			defer wg.Done()
			s.Stop()
		}(stopable)
	}
	log.DebugF("Wait while %d processes stops\n", count)
	wg.Wait()
}

func (s *Session) RegisterStoppable(stopable Stopable) {
	s.Stopables = append(s.Stopables, stopable)
}
