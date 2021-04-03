package main

import (
	_ "github.com/flant/addon-operator/sdk/registry"

	_ "github.com/deckhouse/deckhouse/global-hooks/discovery"
	_ "github.com/deckhouse/deckhouse/modules/300-prometheus/hooks"
)
