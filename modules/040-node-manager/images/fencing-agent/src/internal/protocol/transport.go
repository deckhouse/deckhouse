/*
Copyright 2026 Flant JSC

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

package protocol

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	hcml "github.com/hashicorp/memberlist"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

// BootIDPath is the file with the boot ID of the node.
const BootIDPath = "/proc/sys/kernel/random/boot_id"

const (
	// receiveQueue is the HandoffQueueDepth of memberlist. A dropped answer
	// makes a peer silent, so the queue must not be smaller.
	receiveQueue = 8192
	// metaPublishTimeout is the longest wait of one NodeMeta broadcast.
	metaPublishTimeout = time.Second
)

var errNotStarted = errors.New("memberlist is not started")

// Transport sends and receives the protocol messages through the current
// memberlist instance of one network, and publishes the NodeMeta. Each new
// instance starts a new epoch: messages of the previous instances and answers
// to their questions do not count.
type Transport struct {
	self     string
	network  domain.Network
	keys     *Keyring
	logger   *log.Logger
	received chan domain.Message
	publish  chan struct{}
	polls    polls
	// broadcast allows one NodeMeta broadcast at a time: a new broadcast
	// ends the wait for the previous one. It is a channel, so that synctest
	// can wait on it.
	broadcast chan struct{}

	mu       sync.Mutex
	list     *hcml.Memberlist
	port     uint16
	epoch    uint64
	meta     domain.NodeMeta
	metaData []byte
}

type Config struct {
	NodeName string
	Network  domain.Network
	// BootID is from ReadBootID.
	BootID string
	Keys   *Keyring
}

func NewTransport(cfg Config, logger *log.Logger) *Transport {
	meta := domain.NodeMeta{BootID: cfg.BootID, ProtocolVersion: domain.ProtocolVersion}

	return &Transport{
		self:     cfg.NodeName,
		network:  cfg.Network,
		keys:     cfg.Keys,
		logger:   logger,
		received: make(chan domain.Message, receiveQueue),
		publish:  make(chan struct{}, 1),
		meta:     meta,
		metaData: EncodeMeta(meta),

		broadcast: make(chan struct{}, 1),
	}
}

// ReadBootID reads the boot ID of the node from path, usually BootIDPath.
func ReadBootID(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read boot id: %w", err)
	}

	return strings.TrimSpace(string(b)), nil
}

// Configure prepares cfg for a new memberlist instance and starts a new epoch.
// It sets the delegate, the keyring and the encryption checks. It does not
// set SecretKey: memberlist makes SecretKey the primary key at Create, and
// this can undo a switch of the primary key. Call Attach with the instance
// that memberlist.Create returns.
func (t *Transport) Configure(cfg *hcml.Config) error {
	ring := t.keys.Ring()
	if ring == nil {
		return errors.New("the memberlist key is not loaded")
	}

	t.mu.Lock()
	t.list = nil
	t.epoch++
	epoch := t.epoch
	t.mu.Unlock()

	t.polls.reset()
	t.UpdateMeta(func(m *domain.NodeMeta) { m.KeyID = t.keys.KeyID() })

	cfg.Delegate = &delegate{t: t, epoch: epoch}
	cfg.Keyring = ring
	cfg.GossipVerifyIncoming = true
	cfg.GossipVerifyOutgoing = true

	return nil
}

// Attach makes list the instance for Send. A nil list stops Send. Send uses
// the port of list: Configure runs before the port is set in the config.
func (t *Transport) Attach(list *hcml.Memberlist) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.list = list
	if list != nil {
		t.port = list.LocalNode().Port
	}
}

// Send sends msg to the peer by direct UDP. Send sets the header. In a
// Question it also sets the epoch, the network and the name and the address
// of the peer, and registers the question. In an Answer it sets the network;
// the asker name, the pollID and the epoch come from the question.
func (t *Transport) Send(to domain.Peer, msg domain.Message) error {
	t.mu.Lock()
	list, port, epoch := t.list, t.port, t.epoch
	t.mu.Unlock()

	if list == nil {
		return errNotStarted
	}

	switch m := msg.(type) {
	case domain.Question:
		m.Epoch, m.Network, m.To, m.Addr = epoch, t.network, to.Name, to.Addr
		t.polls.ask(m)
		msg = m
	case domain.Answer:
		m.Network = t.network
		msg = m
	}

	b, err := Encode(domain.Header{Version: domain.ProtocolVersion, From: t.self}, msg)
	if err != nil {
		return err
	}

	return list.SendBestEffort(&hcml.Node{Name: to.Name, Addr: to.Addr.AsSlice(), Port: port}, b)
}

// Received delivers the messages from the peers. Answers come only when they
// match a question of the current poll. Messages of unknown types are
// ignored.
func (t *Transport) Received() <-chan domain.Message {
	return t.received
}

// UpdateMeta changes the NodeMeta at once and publishes it in the background.
// It does not wait for the peers. When the new NodeMeta is larger than
// MetaMaxSize, the previous NodeMeta stays.
func (t *Transport) UpdateMeta(change func(m *domain.NodeMeta)) {
	if !t.setMeta(change) {
		return
	}

	select {
	case t.publish <- struct{}{}:
	default:
	}
}

// PublishMeta changes the NodeMeta and waits up to 1 s until memberlist sends
// it to the peers. Use it when the next step stops the instance.
func (t *Transport) PublishMeta(change func(m *domain.NodeMeta)) error {
	t.setMeta(change)

	return t.updateNode()
}

func (t *Transport) setMeta(change func(m *domain.NodeMeta)) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	m := t.meta
	change(&m)
	b := EncodeMeta(m)
	if len(b) > domain.MetaMaxSize {
		t.logger.Error("the NodeMeta is too large, the previous NodeMeta stays", "size", len(b))

		return false
	}
	t.meta, t.metaData = m, b

	return true
}

// updateNode broadcasts the NodeMeta and waits up to metaPublishTimeout until
// memberlist sends it.
func (t *Transport) updateNode() error {
	t.broadcast <- struct{}{}
	defer func() { <-t.broadcast }()

	list := t.current()
	if list == nil {
		return errNotStarted
	}

	return list.UpdateNode(metaPublishTimeout)
}

// Run publishes the NodeMeta and keeps the keyring in sync with the key
// volume until ctx ends.
func (t *Transport) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Go(func() {
		t.publishLoop(ctx, func() error {
			// A new instance takes the NodeMeta at Create.
			if err := t.updateNode(); !errors.Is(err, errNotStarted) {
				return err
			}

			return nil
		})
	})

	ticker := time.NewTicker(keyReloadInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		if t.keys.reload() {
			t.UpdateMeta(func(m *domain.NodeMeta) { m.KeyID = t.keys.KeyID() })
		}
	}
}

// publishLoop calls update after each change of the NodeMeta. Changes during
// a slow update give one more update.
func (t *Transport) publishLoop(ctx context.Context, update func() error) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.publish:
		}

		if err := update(); err != nil {
			t.logger.Warn("publish the NodeMeta", "error", err)
		}
	}
}

func (t *Transport) current() *hcml.Memberlist {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.list
}

func (t *Transport) receive(epoch uint64, b []byte) {
	t.mu.Lock()
	current := t.epoch
	t.mu.Unlock()

	if epoch != current {
		return
	}

	msg, err := Decode(b)
	if errors.Is(err, errUnknownType) {
		return
	}
	if err != nil {
		t.logger.Debug("drop a message that is not valid", "error", err)
		return
	}

	if a, ok := msg.(domain.Answer); ok && (a.To != t.self || !t.polls.match(a)) {
		return
	}

	select {
	case t.received <- msg:
	default:
		t.logger.Warn("drop a message, the receive queue is full")
	}
}

// delegate binds one memberlist instance to the transport.
type delegate struct {
	t     *Transport
	epoch uint64
}

func (d *delegate) NodeMeta(int) []byte {
	d.t.mu.Lock()
	defer d.t.mu.Unlock()

	return d.t.metaData
}

// NotifyMsg must not block: memberlist calls it from its receive loop.
func (d *delegate) NotifyMsg(b []byte) {
	d.t.receive(d.epoch, b)
}

func (d *delegate) GetBroadcasts(int, int) [][]byte { return nil }

func (d *delegate) LocalState(bool) []byte { return nil }

func (d *delegate) MergeRemoteState([]byte, bool) {}
