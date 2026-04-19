// Copyright 2025 Flant JSC
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

package labels

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeLabels(t *testing.T) {
	t.Run("no arguments returns empty map", func(t *testing.T) {
		result := MergeLabels()
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("single nil map returns empty map", func(t *testing.T) {
		result := MergeLabels(nil)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("single map copies it", func(t *testing.T) {
		original := map[string]string{"a": "1", "b": "2"}
		result := MergeLabels(original)

		assert.Equal(t, original, result)

		// Verify it is a copy, not the same reference
		result["c"] = "3"
		assert.NotContains(t, original, "c")
	})

	t.Run("two maps merge correctly", func(t *testing.T) {
		m1 := map[string]string{"a": "1", "b": "2"}
		m2 := map[string]string{"c": "3", "d": "4"}

		result := MergeLabels(m1, m2)
		expected := map[string]string{"a": "1", "b": "2", "c": "3", "d": "4"}
		assert.Equal(t, expected, result)
	})

	t.Run("later maps override earlier keys", func(t *testing.T) {
		m1 := map[string]string{"a": "old", "b": "keep"}
		m2 := map[string]string{"a": "new", "c": "added"}

		result := MergeLabels(m1, m2)
		expected := map[string]string{"a": "new", "b": "keep", "c": "added"}
		assert.Equal(t, expected, result)
	})

	t.Run("three maps with cascading overrides", func(t *testing.T) {
		m1 := map[string]string{"a": "first", "b": "first"}
		m2 := map[string]string{"a": "second", "c": "second"}
		m3 := map[string]string{"a": "third", "d": "third"}

		result := MergeLabels(m1, m2, m3)
		expected := map[string]string{"a": "third", "b": "first", "c": "second", "d": "third"}
		assert.Equal(t, expected, result)
	})

	t.Run("empty maps in between", func(t *testing.T) {
		m1 := map[string]string{"a": "1"}
		m2 := map[string]string{}
		m3 := map[string]string{"b": "2"}

		result := MergeLabels(m1, m2, m3)
		expected := map[string]string{"a": "1", "b": "2"}
		assert.Equal(t, expected, result)
	})

	t.Run("nil maps in between", func(t *testing.T) {
		m1 := map[string]string{"a": "1"}
		m3 := map[string]string{"b": "2"}

		result := MergeLabels(m1, nil, m3)
		expected := map[string]string{"a": "1", "b": "2"}
		assert.Equal(t, expected, result)
	})

	t.Run("empty string keys and values", func(t *testing.T) {
		m := map[string]string{"": "empty_key", "empty_val": ""}
		result := MergeLabels(m)
		assert.Equal(t, m, result)
	})
}

func TestLabelNames(t *testing.T) {
	t.Run("nil map returns empty slice", func(t *testing.T) {
		result := LabelNames(nil)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("empty map returns empty slice", func(t *testing.T) {
		result := LabelNames(map[string]string{})
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("single entry", func(t *testing.T) {
		result := LabelNames(map[string]string{"key": "val"})
		assert.Equal(t, []string{"key"}, result)
	})

	t.Run("multiple entries returned sorted", func(t *testing.T) {
		labels := map[string]string{"z": "1", "a": "2", "m": "3", "b": "4"}
		result := LabelNames(labels)
		assert.Equal(t, []string{"a", "b", "m", "z"}, result)
	})

	t.Run("already sorted keys", func(t *testing.T) {
		labels := map[string]string{"a": "1", "b": "2", "c": "3"}
		result := LabelNames(labels)
		assert.Equal(t, []string{"a", "b", "c"}, result)
	})
}

func TestLabelValues(t *testing.T) {
	t.Run("empty inputs", func(t *testing.T) {
		result := LabelValues(nil, nil)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("empty label names returns empty slice", func(t *testing.T) {
		result := LabelValues(map[string]string{"a": "1"}, []string{})
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("values in label names order", func(t *testing.T) {
		labels := map[string]string{"b": "B_val", "a": "A_val", "c": "C_val"}
		names := []string{"a", "b", "c"}
		result := LabelValues(labels, names)
		assert.Equal(t, []string{"A_val", "B_val", "C_val"}, result)
	})

	t.Run("missing label returns empty string", func(t *testing.T) {
		labels := map[string]string{"a": "1"}
		names := []string{"a", "b", "c"}
		result := LabelValues(labels, names)
		assert.Equal(t, []string{"1", "", ""}, result)
	})

	t.Run("nil labels map returns empty strings", func(t *testing.T) {
		result := LabelValues(nil, []string{"a", "b"})
		assert.Equal(t, []string{"", ""}, result)
	})

	t.Run("extra labels in map are ignored", func(t *testing.T) {
		labels := map[string]string{"a": "1", "b": "2", "extra": "ignored"}
		names := []string{"a", "b"}
		result := LabelValues(labels, names)
		assert.Equal(t, []string{"1", "2"}, result)
	})
}

func TestIsSubset(t *testing.T) {
	t.Run("both nil", func(t *testing.T) {
		assert.True(t, IsSubset(nil, nil))
	})

	t.Run("both empty", func(t *testing.T) {
		assert.True(t, IsSubset([]string{}, []string{}))
	})

	t.Run("empty b is subset of any a", func(t *testing.T) {
		assert.True(t, IsSubset([]string{"a", "b"}, []string{}))
	})

	t.Run("empty b is subset of empty a", func(t *testing.T) {
		assert.True(t, IsSubset([]string{}, []string{}))
	})

	t.Run("non-empty b not subset of empty a", func(t *testing.T) {
		assert.False(t, IsSubset([]string{}, []string{"a"}))
	})

	t.Run("exact match is subset", func(t *testing.T) {
		assert.True(t, IsSubset([]string{"a", "b", "c"}, []string{"a", "b", "c"}))
	})

	t.Run("proper subset", func(t *testing.T) {
		assert.True(t, IsSubset([]string{"a", "b", "c"}, []string{"a", "c"}))
	})

	t.Run("single element subset", func(t *testing.T) {
		assert.True(t, IsSubset([]string{"a", "b", "c"}, []string{"b"}))
	})

	t.Run("b has element not in a", func(t *testing.T) {
		assert.False(t, IsSubset([]string{"a", "b"}, []string{"a", "c"}))
	})

	t.Run("b larger than a", func(t *testing.T) {
		assert.False(t, IsSubset([]string{"a"}, []string{"a", "b"}))
	})

	t.Run("completely disjoint sets", func(t *testing.T) {
		assert.False(t, IsSubset([]string{"a", "b"}, []string{"c", "d"}))
	})

	t.Run("duplicates in a", func(t *testing.T) {
		assert.True(t, IsSubset([]string{"a", "a", "b"}, []string{"a", "b"}))
	})

	t.Run("duplicates in b", func(t *testing.T) {
		assert.True(t, IsSubset([]string{"a", "b"}, []string{"a", "a"}))
	})
}

// Benchmarks

func BenchmarkMergeLabels_Single(b *testing.B) {
	m := map[string]string{"a": "1", "b": "2", "c": "3"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MergeLabels(m)
	}
}

func BenchmarkMergeLabels_Two(b *testing.B) {
	m1 := map[string]string{"a": "1", "b": "2", "c": "3"}
	m2 := map[string]string{"d": "4", "e": "5", "a": "override"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MergeLabels(m1, m2)
	}
}

func BenchmarkMergeLabels_Three(b *testing.B) {
	m1 := map[string]string{"a": "1", "b": "2"}
	m2 := map[string]string{"c": "3", "d": "4"}
	m3 := map[string]string{"e": "5", "a": "override"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MergeLabels(m1, m2, m3)
	}
}

func BenchmarkLabelNames_Small(b *testing.B) {
	labels := map[string]string{"z": "1", "a": "2", "m": "3"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LabelNames(labels)
	}
}

func BenchmarkLabelNames_Large(b *testing.B) {
	labels := make(map[string]string, 20)
	for i := 0; i < 20; i++ {
		labels[string(rune('a'+i))] = "val"
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LabelNames(labels)
	}
}

func BenchmarkLabelValues(b *testing.B) {
	labels := map[string]string{"a": "1", "b": "2", "c": "3", "d": "4", "e": "5"}
	names := []string{"a", "b", "c", "d", "e"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LabelValues(labels, names)
	}
}

func BenchmarkIsSubset_Match(b *testing.B) {
	a := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	sub := []string{"a", "c", "e", "g"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsSubset(a, sub)
	}
}

func BenchmarkIsSubset_NoMatch(b *testing.B) {
	a := []string{"a", "b", "c", "d"}
	sub := []string{"x", "y", "z"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsSubset(a, sub)
	}
}
