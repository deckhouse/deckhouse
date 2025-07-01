package collectors_test

import (
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/collectors"
)

var (
	_ collectors.ConstCollector = (*collectors.ConstCounterCollector)(nil)
	_ collectors.ConstCollector = (*collectors.ConstGaugeCollector)(nil)
)
