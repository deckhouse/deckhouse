module github.com/deckhouse/deckhouse/dhctl

go 1.15

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/alessio/shellescape v1.4.1
	github.com/fatih/color v1.9.0
	github.com/flant/kube-client v0.0.6
	github.com/flant/logboek v0.3.4
	github.com/fsnotify/fsnotify v1.5.1
	github.com/go-openapi/spec v0.19.8
	github.com/go-openapi/strfmt v0.19.5
	github.com/go-openapi/validate v0.19.12
	github.com/google/go-cmp v0.5.8
	github.com/google/uuid v1.1.2
	github.com/hashicorp/go-multierror v1.1.1
	github.com/iancoleman/strcase v0.0.0-20191112232945-16388991a334
	github.com/peterbourgon/mergemap v0.0.0-20130613134717-e21c03b7a721
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/client_model v0.2.0
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.7.0
	golang.org/x/crypto v0.0.0-20220214200702-86341886e292
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/satori/go.uuid.v1 v1.2.0
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/api v0.24.1
	k8s.io/apiextensions-apiserver v0.19.11
	k8s.io/apimachinery v0.24.1
	k8s.io/client-go v0.24.1
	k8s.io/klog v1.0.0
	sigs.k8s.io/yaml v1.2.0
)

// Remove 'in body' from errors, fix for Go 1.16 (https://github.com/go-openapi/validate/pull/138).
replace github.com/go-openapi/validate => github.com/flant/go-openapi-validate v0.19.12-flant.0

// replace with master branch to work with single dash
replace gopkg.in/alecthomas/kingpin.v2 => github.com/alecthomas/kingpin v1.3.8-0.20200323085623-b6657d9477a6
