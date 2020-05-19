module flant/deckhouse-controller

go 1.12

require (
	flant/deckhouse-candi v0.0.0 // use non-existent version for replace
	github.com/aws/aws-sdk-go v1.15.90
	github.com/flant/addon-operator v1.0.0-beta.6.0.20200521120002-acde9c665e5a // branch: feat_parallel_synchronization
	github.com/flant/shell-operator v1.0.0-beta.9.0.20200519120632-a644bd3f8ab3 // branch: master
	github.com/golang/protobuf v1.3.2
	github.com/google/go-containerregistry v0.0.0-20191002200252-ff1ac7f97758
	github.com/gophercloud/gophercloud v0.1.0
	github.com/helm/helm v2.16.6+incompatible
	github.com/sirupsen/logrus v1.4.2
	github.com/spaolacci/murmur3 v1.1.0
	github.com/stretchr/testify v1.4.0
	github.com/vmware/govmomi v0.21.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
	k8s.io/helm v2.16.6+incompatible
	k8s.io/kubectl v0.17.0
)

//replace github.com/go-openapi/validate => github.com/flant/go-openapi-validate v0.19.4-0.20200313141509-0c0fba4d39e1 // branch: fix_in_body_0_19_7

//replace github.com/flant/shell-operator => ../../shell-operator

//replace github.com/flant/addon-operator => ../../addon-operator

replace flant/deckhouse-candi => ../deckhouse-candi
