/*
Copyright 2026 Flant JSC

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
	"math/rand"
	"slices"
	"time"
)

func main() {
	repoSizes := map[string]int{
		"system/deckhouse":                              50,
		"system/deckhouse/install":                      30,
		"system/deckhouse/install-standalone":           25,
		"system/deckhouse/installer":                    20,
		"system/deckhouse/modules":                      40,
		"system/deckhouse/modules/console":              35,
		"system/deckhouse/modules/console/release":      28,
		"system/deckhouse/modules/pod-reloader":         22,
		"system/deckhouse/modules/pod-reloader/release": 18,
		"system/deckhouse/modules/prompp":               15,
		"system/deckhouse/modules/prompp/release":       12,
		"system/deckhouse/release-channel":              10,
		"system/deckhouse/security/trivy-bdu":           8,
		"system/deckhouse/security/trivy-checks":        6,
		"system/deckhouse/security/trivy-db":            5,
		"system/deckhouse/security/trivy-java-db":       4,
	}

	repos := make([]string, 0, len(repoSizes))
	for repo := range repoSizes {
		repos = append(repos, repo)
	}
	slices.Sort(repos)

	total := 0
	current := 1

	for _, size := range repoSizes {
		total += size
	}

	for _, repo := range repos {
		size := repoSizes[repo]
		for i := 1; i <= size; i++ {
			digest := generateDigest(current)

			fmt.Printf("[%d / %d] Syncing localhost:8888/%s:%s\n",
				current,
				total,
				repo,
				digest,
			)
			current++

			sleepTime := time.Duration(10+rand.Intn(90)) * time.Millisecond
			time.Sleep(sleepTime)
		}
	}
}

func generateDigest(seq int) string {
	return fmt.Sprintf("%064x", seq)
}
