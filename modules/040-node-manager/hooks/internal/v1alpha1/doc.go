package v1alpha1

// +k8s:deepcopy-gen=package

//go:generate deepcopy-gen --input-dirs github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1alpha1/ -O nodeuser_generated.deepcopy --bounding-dirs github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1alpha1/ --go-header-file boilerplate.go.txt --output-base /tmp
//go:generate cp /tmp/github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1alpha1/nodeuser_generated.deepcopy.go ./
