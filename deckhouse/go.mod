module flant/deckhouse

go 1.12

require (
	github.com/flant/addon-operator v1.0.0-beta.5.0.20191114120048-4f1260b1aa5d // branch: fix_synchronization
	github.com/flant/shell-operator v1.0.0-beta.5.0.20191112094847-62345f773937 // branch: fix_synchronization
	github.com/google/go-containerregistry v0.0.0-20191002200252-ff1ac7f97758
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.4.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190409092523-d687e77c8ae9
	k8s.io/apimachinery v0.0.0-20190409092423-760d1845f48b
)
