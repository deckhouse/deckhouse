module flant/deckhouse

go 1.12

require (
	github.com/aws/aws-sdk-go v1.15.90
	github.com/flant/addon-operator v1.0.0-beta.5.0.20200116181100-1881269dd216 // branch: feat_named_queues, +feat_include_snapshots
	github.com/flant/libjq-go v0.0.0-20191126154400-1afb898d97a3
	github.com/flant/shell-operator v1.0.0-beta.5.0.20200116180311-86c4055da42a // branch: feat_named_queues, +feat_kubernetes_binding_mode
	github.com/google/go-containerregistry v0.0.0-20191002200252-ff1ac7f97758
	github.com/sirupsen/logrus v1.4.2
	github.com/spaolacci/murmur3 v1.1.0
	github.com/stretchr/testify v1.4.0
	github.com/vmware/govmomi v0.21.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v2 v2.2.7
	k8s.io/api v0.0.0-20190409092523-d687e77c8ae9
	k8s.io/apimachinery v0.0.0-20190409092423-760d1845f48b
)

//replace github.com/flant/shell-operator => ../../shell-operator

//replace github.com/flant/addon-operator => ../../addon-operator
