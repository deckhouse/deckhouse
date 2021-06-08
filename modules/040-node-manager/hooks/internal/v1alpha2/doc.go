package v1alpha2

// +k8s:deepcopy-gen=package

//go:generate deepcopy-gen --input-dirs github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1alpha2/ -O nodegroup_generated.deepcopy --bounding-dirs github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1alpha2/ --go-header-file boilerplate.go.txt --output-base /tmp
//go:generate cp /tmp/github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1alpha2/nodegroup_generated.deepcopy.go ./
