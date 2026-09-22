module github.com/deckhouse/deckhouse/go_lib/dependency/cr

go 1.26.4

require (
	github.com/gojuno/minimock/v3 v3.4.7
	github.com/google/go-containerregistry v0.21.3 // v0.21.4-v0.21.6 issues: realm handling fixed in v0.21.6 (https://github.com/google/go-containerregistry/pull/2243), but pull hang introduced in this version and still not fixed (https://github.com/google/go-containerregistry/issues/2341).
	github.com/stretchr/testify v1.11.1
	github.com/sylabs/oci-tools v0.19.0
	github.com/tidwall/gjson v1.19.0
	go.opentelemetry.io/otel v1.44.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.18.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/cli v29.3.0+incompatible // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.9.3 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/klauspost/compress v1.18.4 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/vbatts/tar-split v0.12.2 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/metric v1.44.0 // indirect
	go.opentelemetry.io/otel/trace v1.44.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	gotest.tools/v3 v3.5.2 // indirect
)
