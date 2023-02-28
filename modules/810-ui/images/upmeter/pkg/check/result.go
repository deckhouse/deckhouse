/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package check

type Status int64

// We rely on this order: Fail < Success < Unknown, nodata is the inevitable zero value for initialized
// status series that is not meant to be used outside of the package.
const (
	nodata Status = iota
	Down
	Up
	Unknown
)

func (s Status) String() string {
	switch s {
	case nodata:
		return "NoData"
	case Down:
		return "Down"
	case Up:
		return "Up"
	case Unknown:
		return "Unknown"
	default:
		return "(unknown)"
	}
}

// Result represents check result
type Result struct {
	ProbeRef  *ProbeRef
	CheckName string
	Status    Status
}

// NewResult creates result struct for a check
func NewResult(ref ProbeRef, checkName string, status Status) Result {
	return Result{
		ProbeRef:  &ref,
		CheckName: checkName,
		Status:    status,
	}
}

// ProbeResult represents multiple checks results and deduces the common one.
type ProbeResult struct {
	ref      *ProbeRef
	statuses map[string]Status
}

func NewProbeResult(ref ProbeRef) *ProbeResult {
	return &ProbeResult{
		ref:      &ref,
		statuses: make(map[string]Status),
	}
}

func (a *ProbeResult) Add(r Result) {
	a.statuses[r.CheckName] = r.Status
}

func (a *ProbeResult) Status() Status {
	var acc Status
	for _, s := range a.statuses {
		acc = mergeStrategy(acc, s)
	}
	return acc
}

func (a *ProbeResult) ProbeRef() ProbeRef {
	return *a.ref
}
