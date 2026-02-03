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

package version

type UniqueAggregator struct {
	set         map[string]struct{}
	uniqueItems []string
	sortFunc    func([]string)
}

func NewUniqueAggregator(sortFunc func([]string)) *UniqueAggregator {
	return &UniqueAggregator{
		set:         make(map[string]struct{}),
		uniqueItems: make([]string, 0),
		sortFunc:    sortFunc,
	}
}

func (a *UniqueAggregator) Set(item string) {
	if _, exists := a.set[item]; exists {
		return
	}

	a.set[item] = struct{}{}
	a.uniqueItems = append(a.uniqueItems, item)
}

func (a *UniqueAggregator) GetMin() string {
	if len(a.uniqueItems) == 0 {
		return ""
	}
	a.sortFunc(a.uniqueItems)
	return a.uniqueItems[0]
}

func (a *UniqueAggregator) GetMax() string {
	if len(a.uniqueItems) == 0 {
		return ""
	}
	a.sortFunc(a.uniqueItems)
	return a.uniqueItems[len(a.uniqueItems)-1]
}
