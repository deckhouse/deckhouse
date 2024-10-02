/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

import "regexp"

var systemNamespaces = []string{
	"kube-.*",
	"d8-.*",
	"default",
	// legacy
	"antiopa",
	"loghouse",
}

var systemNamespacesRegex []*regexp.Regexp

func init() {
	for _, systemNamespace := range systemNamespaces {
		r, _ := regexp.Compile("^" + systemNamespace + "$")
		systemNamespacesRegex = append(systemNamespacesRegex, r)
	}
}
