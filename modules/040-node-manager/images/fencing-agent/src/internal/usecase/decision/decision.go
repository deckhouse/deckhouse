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

// Package decision decides from the control plane state and the poll of the
// peers whether the agent may feed its watchdog, and answers the peers.
package decision

import "fencing-agent/internal/domain"

// Transport sends and receives the protocol messages on the main network.
type Transport interface {
	// Peers returns the static peer list without the own node and without
	// the nodes in the hold of «leaving». The polls use it.
	Peers() []domain.Peer
	// Peer finds a node of the static peer list by name. Unlike Peers, it
	// also finds a node in the hold of «leaving»: the agent answers that node.
	Peer(name string) (domain.Peer, bool)
	// Send sends msg to the peer by direct UDP. An error means that the peer is silent.
	Send(to domain.Peer, msg domain.Message) error
	// Received delivers the decoded messages from the peers.
	Received() <-chan domain.Message
}

// ControlPlane gives the control plane state.
type ControlPlane interface {
	// Updates delivers each new read.
	Updates() <-chan domain.CPState
}

type Deps struct {
	Transport    Transport
	ControlPlane ControlPlane
	// StopFeed stops the feed of the armed watchdog and tells whether the
	// watchdog is armed. After it returns, the watchdog gets no feed.
	StopFeed func() bool
	// ResumeFeed lets the armed watchdog feed again after StopFeed. The first
	// feed comes at the next feed step. It tells whether the feed may go on.
	// It does not resume after Finalize, in the phase Proven of the own FFNS
	// and after the stop of the agent.
	ResumeFeed func() bool
	// Finalize makes the stop of the feed final.
	Finalize func()
	Metrics  domain.Metrics
	// Leaving gets the name of a peer that sent «leaving». It can be nil.
	Leaving func(node string)
	// Created gets the node of a «created» message. It can be nil.
	Created func(node string)
	// Attest gets the node of an accepted DeclareSuicide. It can be nil.
	Attest func(node string)
}
