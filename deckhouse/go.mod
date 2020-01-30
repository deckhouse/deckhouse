module flant/deckhouse

go 1.12

require (
	github.com/aws/aws-sdk-go v1.15.90
	github.com/flant/addon-operator v1.0.0-beta.5.0.20200130072151-849b4ff0222b // branch: master
	github.com/flant/libjq-go v1.0.0
	github.com/flant/shell-operator v1.0.0-beta.7.0.20200130065049-508e02717e2e // branch: master
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
