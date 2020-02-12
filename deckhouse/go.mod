module flant/deckhouse

go 1.12

require (
	github.com/aws/aws-sdk-go v1.15.90
	github.com/flant/addon-operator v1.0.0-beta.5.0.20200212150523-d9aa873177c6 // branch: master
	github.com/flant/shell-operator v1.0.0-beta.7.0.20200212144836-04e5d35edd9e // branch: master
	github.com/google/go-containerregistry v0.0.0-20191002200252-ff1ac7f97758
	github.com/sirupsen/logrus v1.4.2
	github.com/spaolacci/murmur3 v1.1.0
	github.com/stretchr/testify v1.4.0
	github.com/vmware/govmomi v0.21.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v2 v2.2.7
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
)

//replace github.com/flant/shell-operator => ../../shell-operator

//replace github.com/flant/addon-operator => ../../addon-operator
