module flant/deckhouse-controller

go 1.12

require (
	github.com/deckhouse/deckhouse v0.0.0
	github.com/deckhouse/deckhouse/candictl v0.0.0 // use non-existent version for replace
	github.com/aws/aws-sdk-go v1.15.90
	github.com/blang/semver v3.5.0+incompatible
	github.com/coreos/etcd v3.3.22+incompatible // indirect
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf // indirect
	github.com/flant/addon-operator v1.0.0-rc.1.0.20210402062614-5a3d3d440037 // branch: master
	github.com/flant/shell-operator v1.0.1-0.20210402061915-a583609302e9 // branch: master
	github.com/google/go-containerregistry v0.0.0-20191002200252-ff1ac7f97758
	github.com/gophercloud/gophercloud v0.12.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spaolacci/murmur3 v1.1.0
	github.com/stretchr/testify v1.6.1
	github.com/vmware/govmomi v0.21.0
	go.etcd.io/etcd v3.3.22+incompatible
	go.uber.org/zap v1.15.0 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.18.0
	k8s.io/apimachinery v0.18.0
	k8s.io/kubectl v0.18.0
)

//replace github.com/go-openapi/validate => github.com/flant/go-openapi-validate v0.19.4-0.20200313141509-0c0fba4d39e1 // branch: fix_in_body_0_19_7

// For local development, do not remove these 2 lines, please.
//replace github.com/flant/shell-operator => ../../shell-operator
//replace github.com/flant/addon-operator => ../../addon-operator

replace github.com/deckhouse/deckhouse/candictl => ../candictl

replace github.com/deckhouse/deckhouse => ../

// These are important until upgrade to v0.20
replace k8s.io/api => k8s.io/api v0.17.0

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.0

replace k8s.io/apimachinery => k8s.io/apimachinery v0.17.0

replace k8s.io/client-go => k8s.io/client-go v0.17.0

replace google.golang.org/grpc => google.golang.org/grpc v1.23.0
