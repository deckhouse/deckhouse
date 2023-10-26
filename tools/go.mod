module tools

go 1.15

require (
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/deckhouse/deckhouse v0.0.0
	github.com/flant/addon-operator v1.2.4-0.20231016154044-c0a848fc7d74 // indirect
	github.com/flant/shell-operator v1.3.3-0.20231013105726-aa38dfcd70d1 // indirect
	github.com/golangci/golangci-lint v1.40.1
	github.com/iancoleman/strcase v0.0.0-20191112232945-16388991a334
	gopkg.in/yaml.v3 v3.0.1
)

replace (
	github.com/deckhouse/deckhouse => ../
	github.com/deckhouse/deckhouse/dhctl => ../dhctl
	github.com/deckhouse/deckhouse/go_lib/cloud-data => ../go_lib/cloud-data
)
