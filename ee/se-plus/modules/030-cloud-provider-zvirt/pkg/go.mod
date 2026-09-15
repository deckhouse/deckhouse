module github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg

go 1.25.11

require (
	github.com/deckhouse/deckhouse/go_lib/cloud-provider v0.0.0-00010101000000-000000000000
	k8s.io/apimachinery v0.34.8
)

require (
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
	k8s.io/utils v0.0.0-20260210185600-b8788abfbbc2 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.0 // indirect
)

replace (
	github.com/deckhouse/deckhouse/go_lib/cloud-provider => /deckhouse/go_lib/cloud-provider
	github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol => /deckhouse/go_lib/dhctl-provider-protocol
	github.com/deckhouse/deckhouse/pkg/log => /deckhouse/pkg/log
)
