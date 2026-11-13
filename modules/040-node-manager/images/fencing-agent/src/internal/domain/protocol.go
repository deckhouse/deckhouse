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

package domain

import (
	"net/netip"
	"time"
)

// ProtocolVersion is the version of the peer protocol of this agent.
const ProtocolVersion uint32 = 1

// MetaMaxSize is the largest encoded NodeMeta that memberlist accepts.
const MetaMaxSize = 512

// Network is the memberlist network of a message.
type Network string

const (
	NetworkMain   Network = "main"
	NetworkSystem Network = "system"
)

// Verdict is the answer of a peer about its own view of the control plane.
type Verdict uint8

const (
	// VerdictUnknown is a verdict that this protocol version does not know.
	// The asker counts it as support.
	VerdictUnknown Verdict = iota
	VerdictSee
	VerdictNotSee
	// VerdictDying comes only from an older agent. This agent does not send
	// it and takes it as NOT SEE.
	VerdictDying
)

// Message is one of the protocol messages of this file.
type Message interface {
	isMessage()
}

// Header is in all protocol messages.
type Header struct {
	Version uint32
	// From is the name of the sender node.
	From string
}

func (Header) isMessage() {}

// Question asks a peer about its view of the control plane.
type Question struct {
	Header
	PollID  uint64
	Network Network
	// Epoch is the generation of the memberlist instance of the asker.
	Epoch uint64
	// To is the name of the peer that the asker expects at Addr.
	To   string
	Addr netip.Addr
	// LastReadyAgo is the lastReadyAgo of the asker when it sends the question.
	LastReadyAgo time.Duration
}

// Answer is the reply to a Question. From is the name of the peer that answers.
type Answer struct {
	Header
	// To is the name of the asker.
	To      string
	PollID  uint64
	Epoch   uint64
	Network Network
	Verdict Verdict
	// LastReadyAgo is the lastReadyAgo of the peer that answers.
	LastReadyAgo time.Duration
	// UnknownFields is true when the decoder found fields that this protocol
	// version does not know. The asker counts such an answer as support.
	UnknownFields bool
}

// DeclareSuicide tells a witness that the sender stopped feeding its watchdog.
type DeclareSuicide struct {
	Header
	// PollID is the poll of the answer from the witness.
	PollID uint64
}

// SeeCP comes only from an older agent: the sender sees the control plane
// again. This agent does not send it and takes no action on it.
type SeeCP struct {
	Header
}

// Leaving tells the peers that the sender stops on purpose.
type Leaving struct {
	Header
}

// Asked forwards to the writers of Node that Node asked the sender at At.
type Asked struct {
	Header
	Node string
	At   time.Time
}

// FFNSCreated tells the writers that the FencingFailedNodeState of Node exists.
type FFNSCreated struct {
	Header
	Node string
}

// NodeMeta is the memberlist metadata of the agent.
type NodeMeta struct {
	// BootID is from /proc/sys/kernel/random/boot_id.
	BootID string
	Armed  bool
	// NodeUID is the UID of the own Node.
	NodeUID string
	// KeyID identifies the key that the agent encrypts with.
	KeyID           string
	Leaving         bool
	Maintenance     bool
	ProtocolVersion uint32
	// NodeGroup is the node group of the own Node. Profile is the name of
	// the FencingSLAProfile that the agent applies.
	NodeGroup string
	Profile   string
}
