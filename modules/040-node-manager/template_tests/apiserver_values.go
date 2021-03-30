package template_tests

import (
	"github.com/deckhouse/deckhouse/testing/helm"
)

const (
	bashibleAPIServerCA  = "meapiserverca"
	bashibleAPIServerCrt = "meapiservercrt"
	bashibleAPIServerKey = "meapiserverprivkey"
)

func setBashibleAPIServerTLSValues(f *helm.Config) {
	f.ValuesSet("nodeManager.internal.bashibleApiServerCA", bashibleAPIServerCA)
	f.ValuesSet("nodeManager.internal.bashibleApiServerCrt", bashibleAPIServerCrt)
	f.ValuesSet("nodeManager.internal.bashibleApiServerKey", bashibleAPIServerKey)
}
