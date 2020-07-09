module flant/deckhouse-controller

go 1.12

require (
	flant/deckhouse-candi v0.0.0 // use non-existent version for replace
	github.com/aws/aws-sdk-go v1.15.90
	github.com/coreos/etcd v3.3.22+incompatible // indirect
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/flant/addon-operator v1.0.0-beta.6.0.20200708095832-aa722bddf952 // branch: master
	github.com/flant/shell-operator v1.0.0-beta.10.0.20200708094531-126d47838bc0 // branch: master
	github.com/google/go-containerregistry v0.0.0-20191002200252-ff1ac7f97758
	github.com/gophercloud/gophercloud v0.1.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spaolacci/murmur3 v1.1.0
	github.com/stretchr/testify v1.5.1
	github.com/vmware/govmomi v0.21.0
	go.etcd.io/etcd v3.3.22+incompatible
	go.uber.org/zap v1.15.0 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
	k8s.io/kubectl v0.17.0
)

//replace github.com/go-openapi/validate => github.com/flant/go-openapi-validate v0.19.4-0.20200313141509-0c0fba4d39e1 // branch: fix_in_body_0_19_7

//replace github.com/flant/shell-operator => ../../shell-operator

//replace github.com/flant/addon-operator => ../../addon-operator

replace flant/deckhouse-candi => ../deckhouse-candi
