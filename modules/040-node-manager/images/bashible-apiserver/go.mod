// This is a generated file. Do not edit directly.

module d8.io/bashible

go 1.15

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/Masterminds/sprig/v3 v3.2.0
	github.com/evanphx/json-patch v4.11.0+incompatible // indirect
	github.com/flant/kube-client v0.0.6
	github.com/fsnotify/fsnotify v1.5.1
	github.com/go-openapi/spec v0.20.2
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.17.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/cobra v1.1.1
	github.com/stretchr/testify v1.7.0 // indirect
	go.uber.org/zap v1.13.0 // indirect
	golang.org/x/tools v0.1.5 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/api v0.20.5
	k8s.io/apiextensions-apiserver v0.20.1 // indirect
	k8s.io/apimachinery v0.20.5
	k8s.io/apiserver v0.20.2
	k8s.io/client-go v0.20.5
	k8s.io/code-generator v0.20.2
	k8s.io/component-base v0.20.2
	k8s.io/klog/v2 v2.4.0
	k8s.io/kube-openapi v0.0.0-20210113233702-8566a335510f
	sigs.k8s.io/yaml v1.3.0
)

// replace for flant/kube-client
replace k8s.io/client-go => k8s.io/client-go v0.20.2
