// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package process

// ByteSequenceMatcher can be used to match byte stream against a string
// byte by byte.
type ByteSequenceMatcher struct {
	// settings
	Pattern        string
	waitNonMatched bool

	patternBytes []byte
	patternLen   int

	// state
	// index of a byte that should be matched
	state        int
	patternFound bool
	matchFound   bool
}

func NewByteSequenceMatcher(pattern string) *ByteSequenceMatcher {
	b := []byte(pattern)
	return &ByteSequenceMatcher{
		Pattern:      pattern,
		patternBytes: b,
		patternLen:   len(b),
		state:        0, // need to check first byte
	}
}

func (m *ByteSequenceMatcher) WaitNonMatched() *ByteSequenceMatcher {
	m.waitNonMatched = true
	return m
}

// Analyze matches Pattern from byte stream and ignores \r and \n after it.
// when match is not found, return n
// when match is found, return 0
// return index (0 or more) of a first byte after pattern and \r, \n
// This behaviour is used to write bytes to Reader only after match is found.
func (m *ByteSequenceMatcher) Analyze(buf []byte) (n int) {
	for i, b := range buf {
		// ignore \r and \n
		if b == '\r' || b == '\n' {
			// reset pattern state
			m.state = 0
			continue
		}

		if m.matchFound {
			return i
		}

		if m.patternFound {
			m.matchFound = true
			return i
		}
		if b == m.patternBytes[m.state] {
			m.state++
		} else {
			m.state = 0
		}
		if m.state == m.patternLen {
			m.patternFound = true
			if !m.waitNonMatched {
				m.matchFound = true
				return i + 1
			}
		}
	}

	return len(buf)
}

func (m *ByteSequenceMatcher) Reset() {
	m.matchFound = false
	m.patternFound = false
	m.state = 0
}

func (m *ByteSequenceMatcher) IsMatched() bool {
	return m.matchFound
}
