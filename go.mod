module github.com/deckhouse/deckhouse

go 1.15

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/benjamintf1/unmarshalledmatchers v0.0.0-20190408201839-bb1c1f34eaea
	github.com/clarketm/json v1.15.7
	github.com/cloudflare/cfssl v1.5.0
	github.com/davecgh/go-spew v1.1.1
	github.com/deckhouse/deckhouse/dhctl v0.0.0 // use non-existent version for replace
	github.com/fatih/color v1.9.0
	github.com/flant/addon-operator v1.0.7-0.20221108074825-ba5d9983d012 // branch: main
	github.com/flant/kube-client v0.0.6
	github.com/flant/shell-operator v1.0.13-0.20221018121846-032ccd06522c // branch: main
	github.com/gammazero/deque v0.0.0-20190521012701-46e4ffb7a622
	github.com/ghodss/yaml v1.0.0
	github.com/go-openapi/spec v0.19.8
	github.com/go-sql-driver/mysql v1.5.0 // indirect
	github.com/gojuno/minimock/v3 v3.0.8
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.8
	github.com/google/go-containerregistry v0.0.0-20191002200252-ff1ac7f97758
	github.com/google/uuid v1.1.2
	github.com/gophercloud/gophercloud v0.20.0
	github.com/gophercloud/utils v0.0.0-20210823151123-bfd010397530
	github.com/hashicorp/go-multierror v1.1.1
	github.com/iancoleman/strcase v0.0.0-20191112232945-16388991a334
	github.com/imdario/mergo v0.3.11
	github.com/kyokomi/emoji v2.1.0+incompatible
	github.com/mitchellh/hashstructure/v2 v2.0.2
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.20.2
	github.com/otiai10/copy v1.0.2
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.8.1
	github.com/spaolacci/murmur3 v1.1.0
	github.com/square/go-jose/v3 v3.0.0-20200630053402-0a67ce9b0693
	github.com/stretchr/testify v1.7.2
	github.com/tidwall/gjson v1.12.1
	github.com/tidwall/sjson v1.2.3
	github.com/vmware/govmomi v0.24.1
	go.etcd.io/etcd/api/v3 v3.5.0-alpha.0
	go.etcd.io/etcd/client/v3 v3.5.0-alpha.0
	google.golang.org/grpc v1.32.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
	helm.sh/helm/v3 v3.5.1
	k8s.io/api v0.21.4
	k8s.io/apiextensions-apiserver v0.20.5
	k8s.io/apimachinery v0.21.4
	k8s.io/apiserver v0.20.5
	k8s.io/client-go v0.21.4
	k8s.io/utils v0.0.0-20220210201930-3a6ce19ff2f9
	sigs.k8s.io/yaml v1.3.0
)

replace github.com/deckhouse/deckhouse/dhctl => ./dhctl

// Remove 'in body' from errors, fix for Go 1.16 (https://github.com/go-openapi/validate/pull/138).
replace github.com/go-openapi/validate => github.com/flant/go-openapi-validate v0.19.12-flant.0

// Due to Helm3 lib problems
replace k8s.io/client-go => k8s.io/client-go v0.19.11

replace k8s.io/api => k8s.io/api v0.19.11
