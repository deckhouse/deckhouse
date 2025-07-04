/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package checker

import "time"

const (
	parallelizmPerRegistry = 3

	retryDelay     = time.Second * 10
	processTimeout = time.Second * 30

	showMaxErrItems = 5

	valuesPath            = "systemRegistry.internal.checker"
	valuesParamsPath      = valuesPath + ".params"
	valuesStatePath       = valuesPath + ".state"
	valuesInitializedPath = valuesPath + ".initialized"
)
