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

import (
	"fmt"
)

type ProbeRef struct {
	Group string `json:"group"`
	Probe string `json:"probe"`
}

func (p ProbeRef) Id() string {
	return fmt.Sprintf("%s/%s", p.Group, p.Probe)
}

// ByProbeRef implements sort.Interface based on the probe reference ID.
type ByProbeRef []ProbeRef

func (a ByProbeRef) Len() int           { return len(a) }
func (a ByProbeRef) Less(i, j int) bool { return a[i].Id() < a[j].Id() }
func (a ByProbeRef) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
