/*
Copyright 2021 Flant JSC

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

package snapshot

import (
	"fmt"
	"strings"
)

// Index is a smoke-mini index with useful methods ("a", "b", "c", "d", or "e")
type Index string

func (x Index) String() string {
	return string(x)
}

func (x Index) StatefulSetName() string {
	return fmt.Sprintf("smoke-mini-%s", x)
}

func (x Index) PodName() string {
	return x.StatefulSetName() + "-0"
}

func (x Index) PersistenceVolumeClaimName() string {
	return "disk-" + x.PodName()
}

func IndexFromStatefulSetName(podName string) Index {
	// "smoke-mini-x"   => "x"
	// "smoke-mini-x-0" => "x"
	x := strings.Split(podName, "-")[2]
	return Index(x)
}

func IndexFromPodName(podName string) Index {
	// "smoke-mini-x"   => "x"
	// "smoke-mini-x-0" => "x"
	x := strings.Split(podName, "-")[2]
	return Index(x)
}

func IndexFromPVCName(pvcName string) Index {
	// "disk-smoke-mini-x-0" => "x"
	return IndexFromPodName(PodNameFromPVC(pvcName))
}

func PodNameFromPVC(pvcName string) string {
	return strings.TrimPrefix(pvcName, "disk-")
}
