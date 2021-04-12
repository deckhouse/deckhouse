package main

import (
	_ "github.com/flant/addon-operator/sdk"

	_ "github.com/deckhouse/deckhouse/global-hooks/discovery"
	_ "github.com/deckhouse/deckhouse/modules/020-deckhouse/hooks"
	_ "github.com/deckhouse/deckhouse/modules/041-kube-proxy/hooks"
	_ "github.com/deckhouse/deckhouse/modules/300-prometheus/hooks"
)
