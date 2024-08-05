module dynamix-csi-driver

go 1.22.3

require (
	dynamix-common v0.0.0-00010101000000-000000000000
	github.com/container-storage-interface/spec v1.10.0
	github.com/golang/protobuf v1.5.4
	github.com/kubernetes-csi/csi-lib-utils v0.18.1
	google.golang.org/grpc v1.59.0
	k8s.io/klog/v2 v2.130.1
	k8s.io/utils v0.0.0-20240711033017-18e509b52bc8
	repository.basistech.ru/BASIS/decort-golang-sdk v1.8.2
)

require (
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.11.2 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/leodido/go-urn v1.2.1 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	golang.org/x/net v0.23.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231106174013-bbf56f31fb17 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/klog v1.0.0 // indirect
)

replace dynamix-common => ../../dynamix-common
