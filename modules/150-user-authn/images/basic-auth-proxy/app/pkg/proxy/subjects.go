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

package proxy

// subjectIndex finds the cache keys of the successful credential checks of a user by the subject of
// the ID token Dex issued for them. It is not safe for concurrent use: the Handler guards it together
// with the cache.
type subjectIndex struct {
	byKey     map[string]indexedEntry
	bySubject map[string]map[string]struct{}
	lastSeq   uint64
}

// indexedEntry is the cache entry a key holds. seq tells it from a later entry under the same key.
type indexedEntry struct {
	subject    string
	generation uint64
	seq        uint64
}

func newSubjectIndex() *subjectIndex {
	return &subjectIndex{
		byKey:     make(map[string]indexedEntry),
		bySubject: make(map[string]map[string]struct{}),
	}
}

// add indexes the entry key now holds and returns its sequence number.
func (x *subjectIndex) add(key, subject string, generation uint64) uint64 {
	x.remove(key)

	x.lastSeq++
	x.byKey[key] = indexedEntry{subject: subject, generation: generation, seq: x.lastSeq}

	keys, ok := x.bySubject[subject]
	if !ok {
		keys = make(map[string]struct{})
		x.bySubject[subject] = keys
	}
	keys[key] = struct{}{}

	return x.lastSeq
}

// remove drops key from the index.
func (x *subjectIndex) remove(key string) {
	entry, ok := x.byKey[key]
	if !ok {
		return
	}
	delete(x.byKey, key)

	keys := x.bySubject[entry.subject]
	delete(keys, key)
	if len(keys) == 0 {
		delete(x.bySubject, entry.subject)
	}
}

// forget drops key once its entry seq expires. The cache reports an expiry asynchronously, after a new
// entry may have taken the key; that one stays indexed.
func (x *subjectIndex) forget(key string, seq uint64) {
	if entry, ok := x.byKey[key]; ok && entry.seq == seq {
		x.remove(key)
	}
}

// revoked drops and returns the keys of subject whose tokens have a generation below generation.
func (x *subjectIndex) revoked(subject string, generation uint64) []string {
	var keys []string
	for key := range x.bySubject[subject] {
		if x.byKey[key].generation < generation {
			keys = append(keys, key)
		}
	}
	for _, key := range keys {
		x.remove(key)
	}
	return keys
}

func (x *subjectIndex) len() int {
	return len(x.byKey)
}
