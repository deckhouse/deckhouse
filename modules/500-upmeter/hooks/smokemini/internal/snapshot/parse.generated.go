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

// DO NOT EDIT
// This file was generated automatically with
// 	go run gen_parse.go -type Node,StatefulSet,Pod,StorageClass,PodPhase,PvcTermination
//
// It is used to cast slices of snapshot types. See file types.go

package snapshot

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
)

// ParseNodeSlice parses Node slice from snapshots
func ParseNodeSlice(rs []go_hook.FilterResult) []Node {
	ret := make([]Node, len(rs))
	for i, r := range rs {
		ret[i] = r.(Node)
	}
	return ret
}

// ParseStatefulSetSlice parses StatefulSet slice from snapshots
func ParseStatefulSetSlice(rs []go_hook.FilterResult) []StatefulSet {
	ret := make([]StatefulSet, len(rs))
	for i, r := range rs {
		ret[i] = r.(StatefulSet)
	}
	return ret
}

// ParsePodSlice parses Pod slice from snapshots
func ParsePodSlice(rs []go_hook.FilterResult) []Pod {
	ret := make([]Pod, len(rs))
	for i, r := range rs {
		ret[i] = r.(Pod)
	}
	return ret
}

// ParseStorageClassSlice parses StorageClass slice from snapshots
func ParseStorageClassSlice(rs []go_hook.FilterResult) []StorageClass {
	ret := make([]StorageClass, len(rs))
	for i, r := range rs {
		ret[i] = r.(StorageClass)
	}
	return ret
}

// ParsePodPhaseSlice parses PodPhase slice from snapshots
func ParsePodPhaseSlice(rs []go_hook.FilterResult) []PodPhase {
	ret := make([]PodPhase, len(rs))
	for i, r := range rs {
		ret[i] = r.(PodPhase)
	}
	return ret
}

// ParsePvcTerminationSlice parses PvcTermination slice from snapshots
func ParsePvcTerminationSlice(rs []go_hook.FilterResult) []PvcTermination {
	ret := make([]PvcTermination, len(rs))
	for i, r := range rs {
		ret[i] = r.(PvcTermination)
	}
	return ret
}
