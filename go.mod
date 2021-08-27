module github.com/deckhouse/deckhouse

go 1.15

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/aws/aws-sdk-go v1.15.90
	github.com/benjamintf1/unmarshalledmatchers v0.0.0-20190408201839-bb1c1f34eaea
	github.com/blang/semver v3.5.1+incompatible
	github.com/clarketm/json v1.15.7
	github.com/cloudflare/cfssl v1.5.0
	github.com/davecgh/go-spew v1.1.1
	github.com/deckhouse/deckhouse/dhctl v0.0.0 // use non-existent version for replace
	github.com/fatih/color v1.9.0
	github.com/flant/addon-operator v1.0.0-rc.1.0.20210817112230-c76331cb4fd1 // branch: ref_object_patcher
	github.com/flant/kube-client v0.0.6
	github.com/flant/shell-operator v1.0.4-0.20210826102415-6e7c5922a71f // branch: wait-for-informers-cache-sync
	github.com/gammazero/deque v0.0.0-20190521012701-46e4ffb7a622
	github.com/ghodss/yaml v1.0.0
	github.com/go-openapi/spec v0.19.3
	github.com/gojuno/minimock/v3 v3.0.8
	github.com/google/go-cmp v0.5.2
	github.com/google/go-containerregistry v0.0.0-20191002200252-ff1ac7f97758
	github.com/gophercloud/gophercloud v0.12.0
	github.com/hashicorp/go-multierror v1.0.0
	github.com/iancoleman/strcase v0.0.0-20191112232945-16388991a334
	github.com/imdario/mergo v0.3.8
	github.com/kr/pretty v0.2.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/kyokomi/emoji v2.1.0+incompatible
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/otiai10/copy v1.0.2
	github.com/pkg/errors v0.9.1
	github.com/prometheus/procfs v0.2.0 // indirect
	github.com/sirupsen/logrus v1.7.0
	github.com/spaolacci/murmur3 v1.1.0
	github.com/square/go-jose/v3 v3.0.0-20200630053402-0a67ce9b0693
	github.com/stretchr/testify v1.7.0
	github.com/tidwall/gjson v1.7.5
	github.com/tidwall/sjson v1.1.6
	github.com/vmware/govmomi v0.24.1
	go.etcd.io/etcd/api/v3 v3.5.0-alpha.0
	go.etcd.io/etcd/client/v3 v3.5.0-alpha.0
	golang.org/x/crypto v0.0.0-20210421170649-83a5a9bb288b // indirect
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	golang.org/x/mod v0.4.1 // indirect
	golang.org/x/tools v0.0.0-20210114065538-d78b04bdf963 // indirect
	google.golang.org/grpc v1.32.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v2 v2.3.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
	helm.sh/helm/v3 v3.2.4
	k8s.io/api v0.19.11
	k8s.io/apiextensions-apiserver v0.19.11
	k8s.io/apimachinery v0.20.5
	k8s.io/apiserver v0.19.11
	k8s.io/client-go v0.19.11
	k8s.io/utils v0.0.0-20200729134348-d5654de09c73
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/deckhouse/deckhouse/dhctl => ./dhctl

// TODO uncomment when shell-operator migrates to client-go 0.20.0
// TODO remove when https://github.com/helm/helm/pull/8371 will be merged and released.
//replace helm.sh/helm/v3 => github.com/diafour/helm/v3 v3.2.5-0.20200630114452-b734742e3342 // branch: fix_tpl_performance_3_2_4

// TODO remove replaces below when shell-operator migrates to client-go 0.20.0
// TODO remove ./helm-mod directory as well!
replace helm.sh/helm/v3 => ./helm-mod
