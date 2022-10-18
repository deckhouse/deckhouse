module d8.io/upmeter

go 1.16

require (
	github.com/flant/kube-client v0.0.6
	github.com/flant/shell-operator v1.0.11
	github.com/gogo/protobuf v1.3.2
	github.com/golang-migrate/migrate/v4 v4.14.1
	github.com/golang/snappy v0.0.3
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/mattn/go-sqlite3 v1.14.6
	github.com/onsi/gomega v1.19.0
	github.com/prometheus/prometheus v2.5.0+incompatible
	github.com/sirupsen/logrus v1.8.1
	github.com/spaolacci/murmur3 v1.1.0
	github.com/stretchr/testify v1.7.0
	github.com/tidwall/gjson v1.6.8
	go.opentelemetry.io/contrib/exporters/metric/cortex v0.17.0
	go.uber.org/goleak v1.1.12
	google.golang.org/genproto v0.0.0-20210226172003-ab064af71705 // indirect
	google.golang.org/grpc v1.36.0 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/api v0.19.11
	k8s.io/apimachinery v0.19.11
	k8s.io/client-go v0.19.11
	sigs.k8s.io/yaml v1.2.0
)
