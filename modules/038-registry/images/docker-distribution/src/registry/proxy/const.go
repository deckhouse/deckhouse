package proxy

import (
	"time"
)

const (
	schedulerStateFilePath = "/scheduler-state.json"
	schedulerDefaultTTL    = 24 * 7 * time.Hour
)
