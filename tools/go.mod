module tools

go 1.15

require (
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/deckhouse/deckhouse v0.0.0
	github.com/golangci/golangci-lint v1.40.1
	github.com/iancoleman/strcase v0.2.0
	gopkg.in/yaml.v3 v3.0.1
)

replace (
	github.com/deckhouse/deckhouse => ../
	github.com/deckhouse/deckhouse/dhctl => ../dhctl
	github.com/deckhouse/deckhouse/go_lib/cloud-data => ../go_lib/cloud-data
)

replace go.cypherpunks.ru/gogost/v5 v5.13.0 => github.com/flant/gogost/v5 v5.13.0
