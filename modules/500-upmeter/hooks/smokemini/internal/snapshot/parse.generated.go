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
	"fmt"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// ParseNodeSlice parses Node slice from snapshots
func ParseNodeSlice(rs []sdkpkg.Snapshot) ([]Node, error) {
	ret := make([]Node, len(rs))
	for node, err := range sdkobjectpatch.SnapshotIter[Node](rs) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over snapshots - failed to parse node: %w", err)
		}
		ret = append(ret, node)
	}
	return ret, nil
}

// ParseBoolSlice parses bool slice from snapshots
func ParseBoolSlice(rs []sdkpkg.Snapshot) ([]bool, error) {
	ret := make([]bool, len(rs))
	for bool, err := range sdkobjectpatch.SnapshotIter[bool](rs) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over snapshots - failed to parse bool: %w", err)
		}
		ret = append(ret, bool)
	}
	return ret, nil
}

// ParseStatefulSetSlice parses StatefulSet slice from snapshots
func ParseStatefulSetSlice(rs []sdkpkg.Snapshot) ([]StatefulSet, error) {
	ret := make([]StatefulSet, len(rs))
	for statefulSet, err := range sdkobjectpatch.SnapshotIter[StatefulSet](rs) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over snapshots - failed to parse statefulset: %w", err)
		}
		ret = append(ret, statefulSet)
	}
	return ret, nil
}

// ParsePodSlice parses Pod slice from snapshots
func ParsePodSlice(rs []sdkpkg.Snapshot) ([]Pod, error) {
	ret := make([]Pod, len(rs))
	for pod, err := range sdkobjectpatch.SnapshotIter[Pod](rs) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over snapshots - failed to parse pod: %w", err)
		}
		ret = append(ret, pod)
	}
	return ret, nil
}

// ParseStorageClassSlice parses StorageClass slice from snapshots
func ParseStorageClassSlice(rs []sdkpkg.Snapshot) ([]StorageClass, error) {
	ret := make([]StorageClass, len(rs))
	for storageClass, err := range sdkobjectpatch.SnapshotIter[StorageClass](rs) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over snapshots - failed to parse storageclass: %w", err)
		}
		ret = append(ret, storageClass)
	}
	return ret, nil
}

// ParsePodPhaseSlice parses PodPhase slice from snapshots
func ParsePodPhaseSlice(rs []sdkpkg.Snapshot) ([]PodPhase, error) {
	ret := make([]PodPhase, len(rs))
	for podPhase, err := range sdkobjectpatch.SnapshotIter[PodPhase](rs) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over snapshots - failed to parse podphase: %w", err)
		}
		ret = append(ret, podPhase)
	}
	return ret, nil
}

// ParsePvcTerminationSlice parses PvcTermination slice from snapshots
func ParsePvcTerminationSlice(rs []sdkpkg.Snapshot) ([]PvcTermination, error) {
	ret := make([]PvcTermination, len(rs))
	for pvcTermination, err := range sdkobjectpatch.SnapshotIter[PvcTermination](rs) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over snapshots - failed to parse pvctermination: %w", err)
		}
		ret = append(ret, pvcTermination)
	}
	return ret, nil
}
