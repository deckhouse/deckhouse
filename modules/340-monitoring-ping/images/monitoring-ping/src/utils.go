/*
Copyright 2025 Flant JSC

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

package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func Summarize(rtts []float64) (min, max, mean, std, sum float64) {
	n := float64(len(rtts))
	if n == 0 {
		return 0, 0, 0, 0, 0
	}

	min = slices.Min(rtts)
	max = slices.Max(rtts)

	for _, v := range rtts {
		sum += v
	}
	mean = sum / n

	var variance float64
	for _, v := range rtts {
		d := v - mean
		variance += d * d
	}
	std = math.Sqrt(variance / n)

	// Sanity check
	if math.IsNaN(std) {
		std = 0
	}
	return
}

// GetTargetName returns the name if it's non-empty, otherwise returns the address.
func GetTargetName(name, address string) string {
	if name == "" {
		return address
	}
	return name
}

// DiffMaps returns keys that exist in oldMap but not in newMap.
func DiffMaps(oldMap, newMap map[string]string) map[string]string {
	diff := make(map[string]string)
	for k, v := range oldMap {
		if _, exists := newMap[k]; !exists {
			diff[k] = v
		}
	}
	return diff
}

// BuildClusterMap builds a map of IP -> Name for cluster nodes, using GetTargetName.
func BuildClusterMap(targets []NodeTarget) map[string]string {
	m := make(map[string]string, len(targets))
	for _, t := range targets {
		m[t.IP] = GetTargetName(t.Name, t.IP)
	}
	return m
}

// BuildExternalMap builds a map of Host -> Name for external targets, using GetTargetName.
func BuildExternalMap(targets []ExternalTarget) map[string]string {
	m := make(map[string]string, len(targets))
	for _, t := range targets {
		m[t.Host] = GetTargetName(t.Name, t.Host)
	}
	return m
}

// TODO remove this function in future release
// Removed deprecated prometheus exporter file to avoid stale metrics in grafana.
func CleanUpDeprecatedExporterFile() {
	dirPath := "/node-exporter-textfile"
	filePattern := "monitoring-ping*.prom"
	fullPath := filepath.Join(dirPath, filePattern)

	files, err := filepath.Glob(fullPath)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to find files with pattern %s: %v", fullPath, err))
		return
	}

	for _, file := range files {
		err := os.Remove(file)
		if err != nil {
			log.Error(fmt.Sprintf("Failed to remove file %s: %v", file, err))
			continue
		}
		log.Info(fmt.Sprintf("File %s removed", file))
	}
}
