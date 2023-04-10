/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// https://github.com/aquasecurity/trivy/blob/e6ab389f9eafa5bd1f7879f60c1cc1a14f71e106/pkg/compliance/spec/compliance.go

package apis

type ControlStatus string

const (
	FailStatus ControlStatus = "FAIL"
	PassStatus ControlStatus = "PASS"
	WarnStatus ControlStatus = "WARN"
)
