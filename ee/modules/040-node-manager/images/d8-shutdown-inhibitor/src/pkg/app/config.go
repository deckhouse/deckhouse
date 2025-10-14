/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package app

import (
	"time"
)

// Default configuration
const (
	InhibitNodeShutdownLabel = "pod.deckhouse.io/inhibit-node-shutdown"
	InhibitDelayMaxSec       = 3 * 24 * time.Hour // 3 days
	WallBroadcastInterval    = 42 * time.Second
	PodsCheckingInterval     = 15 * time.Second
)

type AppConfig struct {
	InhibitDelayMax       time.Duration
	WallBroadcastInterval time.Duration
	PodsCheckingInterval  time.Duration
	PodLabel              string
	NodeName              string
}
