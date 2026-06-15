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

package fswrap

import (
	"io/fs"
	"testing"
	"testing/fstest"
)

// makeFS builds an in-memory fstest.MapFS for tests.
func makeFS() fstest.MapFS {
	return fstest.MapFS{
		"file.txt":            {Data: []byte("root file")},
		"dir/child.txt":       {Data: []byte("child file")},
		"dir/nested/deep.txt": {Data: []byte("deep file")},
		"other/file.txt":      {Data: []byte("other file")},
	}
}

// ---------------------------------------------------------------------------
// Open
// ---------------------------------------------------------------------------

func TestOpen(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "root dot",
			path:    ".",
			wantErr: false,
		},
		{
			name:    "existing file",
			path:    "file.txt",
			wantErr: false,
		},
		{
			name:    "existing directory",
			path:    "dir",
			wantErr: false,
		},
		{
			name:    "nested file",
			path:    "dir/child.txt",
			wantErr: false,
		},
		{
			name:    "non-existing path",
			path:    "no_such_file.txt",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSubFS(makeFS())
			f, err := s.Open(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Open(%q) err = %v, wantErr = %v", tt.path, err, tt.wantErr)
			}
			if err == nil {
				_ = f.Close()
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ReadDir
// ---------------------------------------------------------------------------

func TestReadDir(t *testing.T) {
	tests := []struct {
		name      string
		dir       string
		wantNames []string
		wantErr   bool
	}{
		{
			name:      "root directory",
			dir:       ".",
			wantNames: []string{"dir", "file.txt", "other"},
			wantErr:   false,
		},
		{
			name:      "sub directory",
			dir:       "dir",
			wantNames: []string{"child.txt", "nested"},
			wantErr:   false,
		},
		{
			name:    "non-existing directory",
			dir:     "no_such_dir",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSubFS(makeFS())
			entries, err := s.ReadDir(tt.dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadDir(%q) err = %v, wantErr = %v", tt.dir, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if len(entries) != len(tt.wantNames) {
				t.Fatalf("ReadDir(%q) returned %d entries, want %d", tt.dir, len(entries), len(tt.wantNames))
			}
			for i, e := range entries {
				if e.Name() != tt.wantNames[i] {
					t.Errorf("ReadDir(%q)[%d] = %q, want %q", tt.dir, i, e.Name(), tt.wantNames[i])
				}
			}
		})
	}
}

// TestReadDirFallback verifies that ReadDir returns fs.ErrInvalid when the
// underlying fs.FS does not implement fs.ReadDirFS.
func TestReadDirFallback(t *testing.T) {
	// minFS implements only fs.FS (no ReadDir).
	type minFS struct{ fs.FS }
	s := &SubFS{base: minFS{makeFS()}, relRoot: "."}

	_, err := s.ReadDir(".")
	if err != fs.ErrInvalid {
		t.Errorf("ReadDir on non-ReadDirFS = %v, want fs.ErrInvalid", err)
	}
}

// ---------------------------------------------------------------------------
// Sub
// ---------------------------------------------------------------------------

func TestSub(t *testing.T) {
	tests := []struct {
		name        string
		dir         string
		openAfter   string
		wantOpenErr bool
		wantSubErr  bool
	}{
		{
			name:        "dot returns same SubFS",
			dir:         ".",
			openAfter:   "file.txt",
			wantOpenErr: false,
			wantSubErr:  false,
		},
		{
			name:        "sub into existing dir",
			dir:         "dir",
			openAfter:   "child.txt",
			wantOpenErr: false,
			wantSubErr:  false,
		},
		{
			name:       "sub into non-existing dir",
			dir:        "no_such_dir",
			wantSubErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSubFS(makeFS())
			sub, err := s.Sub(tt.dir)
			if (err != nil) != tt.wantSubErr {
				t.Errorf("Sub(%q) err = %v, wantErr = %v", tt.dir, err, tt.wantSubErr)
				return
			}
			if err != nil {
				return
			}
			if tt.openAfter == "" {
				return
			}
			f, err := sub.Open(tt.openAfter)
			if (err != nil) != tt.wantOpenErr {
				t.Errorf("Open(%q) after Sub(%q) err = %v, wantErr = %v", tt.openAfter, tt.dir, err, tt.wantOpenErr)
				return
			}
			if err == nil {
				_ = f.Close()
			}
		})
	}
}

func TestSubDotReturnsSameInstance(t *testing.T) {
	s := NewSubFS(makeFS())
	sub, err := s.Sub(".")
	if err != nil {
		t.Fatalf("Sub(.) err = %v", err)
	}
	if sub != s {
		t.Error("Sub(.) did not return the same SubFS instance")
	}
}

func TestSubRestrictsAccess(t *testing.T) {
	// After Sub("dir"), files outside "dir" should not be accessible.
	s := NewSubFS(makeFS())
	sub, err := s.Sub("dir")
	if err != nil {
		t.Fatalf("Sub(dir) err = %v", err)
	}

	// file.txt is in root, not in dir — must not be reachable.
	_, err = sub.Open("file.txt")
	if err == nil {
		t.Error("Open(file.txt) after Sub(dir) succeeded, want error")
	}

	// child.txt is inside dir — must be reachable.
	f, err := sub.Open("child.txt")
	if err != nil {
		t.Errorf("Open(child.txt) after Sub(dir) err = %v, want nil", err)
	} else {
		_ = f.Close()
	}
}

func TestSubNested(t *testing.T) {
	s := NewSubFS(makeFS())

	sub, err := s.Sub("dir")
	if err != nil {
		t.Fatalf("Sub(dir) err = %v", err)
	}

	nested, err := sub.(fs.SubFS).Sub("nested")
	if err != nil {
		t.Fatalf("Sub(nested) err = %v", err)
	}

	f, err := nested.Open("deep.txt")
	if err != nil {
		t.Errorf("Open(deep.txt) in nested sub err = %v, want nil", err)
	} else {
		_ = f.Close()
	}
}

// ---------------------------------------------------------------------------
// join (indirect via Open/ReadDir behavior)
// ---------------------------------------------------------------------------

func TestJoinNormalizesPath(t *testing.T) {
	// Paths with redundant components should still resolve correctly.
	s := NewSubFS(makeFS())

	// "dir/../file.txt" should clean to "file.txt"
	f, err := s.Open("dir/../file.txt")
	if err != nil {
		t.Errorf("Open(dir/../file.txt) err = %v, want nil", err)
	} else {
		_ = f.Close()
	}
}
