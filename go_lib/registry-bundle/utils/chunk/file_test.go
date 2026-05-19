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

package chunk

import (
	"testing"
)

func TestIsFirstChunkFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{
			name:     "first chunk file",
			filename: "file.0000.chunk",
			want:     true,
		},
		{
			name:     "chunk file with multiple dots",
			filename: "backup.tar.0000.chunk",
			want:     true,
		},
		{
			name:     "second chunk file",
			filename: "file.0001.chunk",
			want:     false,
		},
		{
			name:     "chunk file with different number",
			filename: "file.1234.chunk",
			want:     false,
		},
		{
			name:     "chunk file with path",
			filename: "path/to/file.1234.chunk",
			want:     false,
		},
		{
			name:     "not a chunk file - wrong extension",
			filename: "file.0000.txt",
			want:     false,
		},
		{
			name:     "not a chunk file - no extension",
			filename: "file",
			want:     false,
		},
		{
			name:     "not a chunk file - less digits",
			filename: "file.000.chunk",
			want:     false,
		},
		{
			name:     "not a chunk file - more digits",
			filename: "file.00000.chunk",
			want:     false,
		},
		{
			name:     "not a chunk file - letters instead of digits",
			filename: "file.xxxx.chunk",
			want:     false,
		},
		{
			name:     "not a chunk file - wrong pattern",
			filename: "file.txt",
			want:     false,
		},
		{
			name:     "chunk without leading zeros",
			filename: "file.1.chunk",
			want:     false,
		},
		{
			name:     "empty filename",
			filename: "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsFirstChunkFile(tt.filename)
			if got != tt.want {
				t.Errorf("IsFirstChunkFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestIsChunkFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{
			name:     "first chunk file",
			filename: "file.0000.chunk",
			want:     true,
		},
		{
			name:     "second chunk file",
			filename: "file.0001.chunk",
			want:     true,
		},
		{
			name:     "chunk file with different number",
			filename: "file.1234.chunk",
			want:     true,
		},
		{
			name:     "chunk file with multiple dots",
			filename: "backup.tar.0000.chunk",
			want:     true,
		},
		{
			name:     "chunk file with path",
			filename: "path/to/file.1234.chunk",
			want:     true,
		},
		{
			name:     "not a chunk file - wrong extension",
			filename: "file.0000.txt",
			want:     false,
		},
		{
			name:     "not a chunk file - no extension",
			filename: "file",
			want:     false,
		},
		{
			name:     "not a chunk file - less digits",
			filename: "file.000.chunk",
			want:     false,
		},
		{
			name:     "not a chunk file - more digits",
			filename: "file.00000.chunk",
			want:     false,
		},
		{
			name:     "not a chunk file - letters instead of digits",
			filename: "file.xxxx.chunk",
			want:     false,
		},
		{
			name:     "not a chunk file - wrong pattern",
			filename: "file.txt",
			want:     false,
		},
		{
			name:     "chunk without leading zeros",
			filename: "file.1.chunk",
			want:     false,
		},
		{
			name:     "empty filename",
			filename: "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsChunkFile(tt.filename)
			if got != tt.want {
				t.Errorf("IsChunkFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestBaseName(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "first chunk file",
			filename: "file.0000.chunk",
			want:     "file",
		},
		{
			name:     "second chunk file",
			filename: "file.0001.chunk",
			want:     "file",
		},
		{
			name:     "chunk file with different number",
			filename: "file.1234.chunk",
			want:     "file",
		},
		{
			name:     "chunk file with multiple dots",
			filename: "backup.tar.0000.chunk",
			want:     "backup.tar",
		},
		{
			name:     "chunk file with path",
			filename: "path/to/file.1234.chunk",
			want:     "path/to/file",
		},
		{
			name:     "not a chunk file - wrong extension",
			filename: "file.0000.txt",
			want:     "file.0000.txt",
		},
		{
			name:     "not a chunk file - no extension",
			filename: "file",
			want:     "file",
		},
		{
			name:     "not a chunk file - less digits",
			filename: "file.000.chunk",
			want:     "file.000.chunk",
		},
		{
			name:     "not a chunk file - more digits",
			filename: "file.00000.chunk",
			want:     "file.00000.chunk",
		},
		{
			name:     "not a chunk file - letters instead of digits",
			filename: "file.xxxx.chunk",
			want:     "file.xxxx.chunk",
		},
		{
			name:     "not a chunk file - wrong pattern",
			filename: "file.txt",
			want:     "file.txt",
		},
		{
			name:     "chunk without leading zeros",
			filename: "file.1.chunk",
			want:     "file.1.chunk",
		},
		{
			name:     "empty filename",
			filename: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BaseName(tt.filename)
			if got != tt.want {
				t.Errorf("BaseName(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}
