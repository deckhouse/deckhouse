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

import (
	"testing"
)

func Test_Match_Several_buffers(t *testing.T) {
	m := NewByteSequenceMatcher("SUCCESS").WaitNonMatched()

	r := m.Analyze([]byte("SUCC"))

	if r != 4 {
		t.Errorf("Should return len buf when match is not triggered: expect 4, got %d", r)
	}

	r = m.Analyze([]byte("ESS"))

	if r != 3 {
		t.Errorf("Should return len buf when match is not triggered: expect 3, got %d", r)
	}

	r = m.Analyze([]byte("\r\n\r\n\n\nOutput"))
	if r != 6 {
		t.Errorf("Should return first non \r \n byte after match is triggered: expect 6, got %d", r)
	}
}

func Test_Match_Several_buffers_no_r_n(t *testing.T) {
	m := NewByteSequenceMatcher("SUCCESS").WaitNonMatched()

	r := m.Analyze([]byte("SUCC"))

	if r != 4 {
		t.Errorf("Should return len buf when match is not triggered: expect 4, got %d", r)
	}

	r = m.Analyze([]byte("ESS"))

	if r != 3 {
		t.Errorf("Should return len buf when match is not triggered: expect 3, got %d", r)
	}

	r = m.Analyze([]byte("Output"))
	if r != 0 {
		t.Errorf("Should return first non \r \n byte after match is triggered: expect 6, got %d", r)
	}
}

func Test_Match_Several_buffers_almost_match(t *testing.T) {
	m := NewByteSequenceMatcher("SUCCESS").WaitNonMatched()

	r := m.Analyze([]byte("SUCC"))

	if r != 4 {
		t.Errorf("Should return len buf when match is not triggered: expect 4, got %d", r)
	}

	r = m.Analyze([]byte("ES"))

	if r != 2 {
		t.Errorf("Should return len buf when match is not triggered: expect 2, got %d", r)
	}

	r = m.Analyze([]byte("-SUCC"))

	if r != 5 {
		t.Errorf("Should return len buf when match is not triggered: expect 5, got %d", r)
	}

	r = m.Analyze([]byte("ESS"))

	if r != 3 {
		t.Errorf("Should return len buf when match is not triggered: expect 3, got %d", r)
	}

	r = m.Analyze([]byte("Output"))
	if r != 0 {
		t.Errorf("Should return first non \r \n byte after match is triggered: expect 6, got %d", r)
	}
}

func Test_Match_One_buffer(t *testing.T) {
	m := NewByteSequenceMatcher("SUCCESS").WaitNonMatched()

	before := []byte("Sometext\r\n\r\r\n")
	pattern := []byte("SUCCESS\r")
	after := []byte("More text")

	var buf []byte

	buf = append(buf, before...)
	buf = append(buf, pattern...)
	buf = append(buf, after...)

	r := m.Analyze(buf)

	expect := len(before) + len(pattern)
	if r != expect {
		t.Errorf("Should return first non \r \n byte after match is triggered: expect %d, got %d", expect, r)
	}
}

func Test_Match_NoWait(t *testing.T) {
	m := NewByteSequenceMatcher("SUCCESS")

	r := m.Analyze([]byte("SUCC"))

	if r != 4 {
		t.Errorf("Should return len buf when match is not triggered: expect 4, got %d", r)
	}

	r = m.Analyze([]byte("ESS"))
	if r != 3 {
		t.Errorf("Should return len buf when match is not triggered: expect 3, got %d", r)
	}

	r = m.Analyze([]byte("\r\n\r\n\n\nOutput"))
	if r != 6 {
		t.Errorf("Should return first non \r \n byte after match is triggered: expect 6, got %d", r)
	}
}
