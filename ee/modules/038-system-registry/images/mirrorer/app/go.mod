module mirrorer

go 1.23.1

replace github.com/deckhouse/deckhouse/pkg/log => ../../../../../../pkg/log

require (
	github.com/deckhouse/deckhouse/pkg/log v0.0.0-00010101000000-000000000000
	github.com/go-ozzo/ozzo-validation v3.6.0+incompatible
	github.com/google/go-containerregistry v0.20.2
	golang.org/x/sync v0.10.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/DataDog/gostackparse v0.7.0 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.14.3 // indirect
	github.com/docker/cli v27.1.1+incompatible // indirect
	github.com/docker/distribution v2.8.2+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.7.0 // indirect
	github.com/klauspost/compress v1.16.5 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc3 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/sirupsen/logrus v1.9.1 // indirect
	github.com/vbatts/tar-split v0.11.3 // indirect
	golang.org/x/sys v0.15.0 // indirect
)
