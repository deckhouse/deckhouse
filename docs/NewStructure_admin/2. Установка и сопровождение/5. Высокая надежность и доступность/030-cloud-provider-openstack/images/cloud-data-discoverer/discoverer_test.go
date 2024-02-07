/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"reflect"
	"testing"
)

func TestRemoveDuplicates(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{
			args: []string{"1", ""},
			want: []string{"1"},
		},
		{
			args: []string{"1", "1"},
			want: []string{"1"},
		},
		{
			args: []string{"1", "2", "3", "1"},
			want: []string{"1", "2", "3"},
		},
		{
			args: []string{"one", "two", "three", "four", "two", "four", "five"},
			want: []string{"one", "two", "three", "four", "five"},
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			if got := removeDuplicates(test.args); !reflect.DeepEqual(got, test.want) {
				t.Errorf("removeDuplicates() = %v, want %v", got, test.want)
			}
		})
	}
}
