module flant/deckhouse

go 1.12

require (
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/flant/addon-operator v1.0.0-beta.5.0.20190923141242-4955bcf490b1 // merged branch: tiller_sidecar
	github.com/flant/shell-operator v1.0.0-beta.5.0.20190923140739-5f7d9cca9885 // branch: release-1.0
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/google/btree v1.0.0 // indirect
	github.com/google/go-containerregistry v0.0.0-20191002200252-ff1ac7f97758
	github.com/gorilla/mux v1.7.2 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/magiconair/properties v1.8.1 // indirect
	github.com/otiai10/curr v0.0.0-20190513014714-f5a3d24e5776 // indirect
	github.com/romana/rlog v0.0.0-20171115192701-f018bc92e7d7
	github.com/sirupsen/logrus v1.4.2 // indirect
	github.com/stretchr/testify v1.4.0
	golang.org/x/tools v0.0.0-20191001184121-329c8d646ebe
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/satori/go.uuid.v1 v1.2.0
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190409092523-d687e77c8ae9
	k8s.io/apimachinery v0.0.0-20190409092423-760d1845f48b
	k8s.io/client-go v0.0.0-20190411052641-7a6b4715b709
)
