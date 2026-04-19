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

// scheduledPing holds the next scheduled send time, the target host, and remaining count.
package fastping

import "time"

type scheduledPing struct {
	host   string
	sendAt time.Time
	count  int
}

// pingHeap is a priority queue (min-heap) of scheduled pings, ordered by sendAt time.
type pingHeap []*scheduledPing

func (h pingHeap) Len() int           { return len(h) }
func (h pingHeap) Less(i, j int) bool { return h[i].sendAt.Before(h[j].sendAt) }
func (h pingHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *pingHeap) Push(x any) {
	*h = append(*h, x.(*scheduledPing))
}

func (h *pingHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}
