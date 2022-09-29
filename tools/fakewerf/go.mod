module fakewerf

go 1.18

require (
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/gobwas/glob v0.2.3
	github.com/werf/werf v1.2.170-0.20220907163711-43d1ca59d5d0
	github.com/yargevad/filepathx v1.0.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/semver/v3 v3.1.1 // indirect
	github.com/avelino/slugify v0.0.0-20180501145920-855f152bd774 // indirect
	github.com/bmatcuk/doublestar v1.1.5 // indirect
	github.com/djherbis/buffer v1.1.0 // indirect
	github.com/djherbis/nio/v3 v3.0.1 // indirect
	github.com/docker/docker v20.10.14+incompatible // indirect
	github.com/go-git/go-git/v5 v5.4.2 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gookit/color v1.3.7 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/shopspring/decimal v1.2.0 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/werf/logboek v0.5.4 // indirect
	golang.org/x/crypto v0.0.0-20220411220226-7b82a4e95df4 // indirect
	golang.org/x/net v0.0.0-20220728211354-c7608f3a8462 // indirect
	golang.org/x/sys v0.0.0-20220731174439-a90be440212d // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	golang.org/x/text v0.3.7 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305

replace k8s.io/helm => github.com/werf/helm v0.0.0-20210202111118-81e74d46da0f

replace github.com/deislabs/oras => github.com/werf/third-party-oras v0.9.1-0.20210927171747-6d045506f4c8

replace helm.sh/helm/v3 => github.com/werf/3p-helm/v3 v3.0.0-20220902145201-6265178e3c32

replace github.com/go-git/go-git/v5 => github.com/ZauberNerd/go-git/v5 v5.4.3-0.20220315170230-29ec1bc1e5db
