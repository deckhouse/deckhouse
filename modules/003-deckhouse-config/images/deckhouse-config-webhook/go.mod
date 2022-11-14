module deckhouse-config-webhook

go 1.16

require (
	github.com/deckhouse/deckhouse v0.0.0
	github.com/onsi/gomega v1.19.0
	github.com/sirupsen/logrus v1.8.1
	github.com/slok/kubewebhook/v2 v2.2.0
	k8s.io/api v0.22.4
	k8s.io/apimachinery v0.22.4
	k8s.io/kube-openapi v0.0.0 // indirect
	sigs.k8s.io/yaml v1.3.0
)

// Replacements to successful compilation with kubewebhook and addon-operator.

// Remove 'in body' from errors, fix for Go 1.16 (https://github.com/go-openapi/validate/pull/138).
replace github.com/go-openapi/validate => github.com/flant/go-openapi-validate v0.19.12-flant.0

// Due to Helm3 lib problems
replace k8s.io/client-go => k8s.io/client-go v0.19.11

replace k8s.io/api => k8s.io/api v0.19.11

replace k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7 // indirect

replace github.com/deckhouse/deckhouse => ../../../..

// Needed for github.com/deckhouse/deckhouse/go.mod
replace github.com/deckhouse/deckhouse/dhctl => ../../../../dhctl
