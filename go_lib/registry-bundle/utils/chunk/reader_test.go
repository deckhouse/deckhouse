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
	"bytes"
	"io"
	"testing"
)

// sectionChunk wraps bytes.NewReader in an io.SectionReader so it satisfies
// ReaderAtSeekerCloser without touching the filesystem.
func sectionChunk(data []byte) chunk {
	r := bytes.NewReader(data)
	sr := io.NewSectionReader(r, 0, int64(len(data)))
	return chunk{file: struct {
		*io.SectionReader
		io.Closer
	}{sr, io.NopCloser(nil)}, size: int64(len(data))}
}

// newReader builds a ChunkReader from the given chunks, computing totalSize.
func newReader(chunks ...chunk) *ChunkReader {
	r := &ChunkReader{chunks: chunks}
	for _, ch := range chunks {
		r.totalSize += ch.size
	}
	return r
}

type closerFunc func() error

func (f closerFunc) Close() error { return f() }

// ---------------------------------------------------------------------------
// eofPosition
// ---------------------------------------------------------------------------

func TestEofPosition(t *testing.T) {
	tests := []struct {
		name       string
		reader     *ChunkReader
		wantIndex  int
		wantOffset int64
	}{
		{
			name:      "two chunks",
			reader:    newReader(sectionChunk([]byte("abc")), sectionChunk([]byte("de"))),
			wantIndex: 2,
		},
		{
			name:      "empty reader",
			reader:    &ChunkReader{},
			wantIndex: 0,
		},
		{
			name:      "single chunk",
			reader:    newReader(sectionChunk([]byte("abc"))),
			wantIndex: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := tt.reader.eofPosition()
			if pos.index != tt.wantIndex {
				t.Errorf("eofPosition().index = %d, want %d", pos.index, tt.wantIndex)
			}
			if pos.offset != tt.wantOffset {
				t.Errorf("eofPosition().offset = %d, want %d", pos.offset, tt.wantOffset)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// positionOffset
// ---------------------------------------------------------------------------

func TestPositionOffset(t *testing.T) {
	// chunks: "abc" (3) + "de" (2) => total 5
	r := newReader(sectionChunk([]byte("abc")), sectionChunk([]byte("de")))

	tests := []struct {
		name string
		pos  position
		want int64
	}{
		{
			name: "start of first chunk",
			pos:  position{0, 0},
			want: 0,
		},
		{
			name: "middle of first chunk",
			pos:  position{0, 1},
			want: 1,
		},
		{
			name: "end of first chunk (boundary)",
			pos:  position{0, 3},
			want: 3,
		},
		{
			name: "start of second chunk",
			pos:  position{1, 0},
			want: 3,
		},
		{
			name: "middle of second chunk",
			pos:  position{1, 1},
			want: 4,
		},
		{
			name: "end of second chunk",
			pos:  position{1, 2},
			want: 5,
		},
		{
			name: "eof sentinel (index == len)",
			pos:  position{2, 0},
			want: 5,
		},
		{
			name: "far past end",
			pos:  position{99, 0},
			want: 5,
		},
		{
			name: "in-chunk offset exceeds totalSize — capped",
			pos:  position{0, 100},
			want: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.positionOffset(tt.pos)
			if got != tt.want {
				t.Errorf("positionOffset(%+v) = %d, want %d", tt.pos, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// offsetPosition
// ---------------------------------------------------------------------------

func TestOffsetPosition(t *testing.T) {
	// chunks: "abc" (3) + "de" (2) => total 5
	r := newReader(sectionChunk([]byte("abc")), sectionChunk([]byte("de")))

	tests := []struct {
		name    string
		offset  int64
		wantPos position
		wantErr error
	}{
		{
			name:    "offset 0 — first byte",
			offset:  0,
			wantPos: position{0, 0},
			wantErr: nil,
		},
		{
			name:    "offset 2 — last byte of first chunk",
			offset:  2,
			wantPos: position{0, 2},
			wantErr: nil,
		},
		{
			name:    "offset 3 — first byte of second chunk",
			offset:  3,
			wantPos: position{1, 0},
			wantErr: nil,
		},
		{
			name:    "offset 4 — last byte of second chunk",
			offset:  4,
			wantPos: position{1, 1},
			wantErr: nil,
		},
		{
			name:    "offset 5 — exactly totalSize (EOF)",
			offset:  5,
			wantPos: position{2, 0},
			wantErr: io.EOF,
		},
		{
			name:    "offset far beyond totalSize",
			offset:  100,
			wantPos: position{2, 0},
			wantErr: io.EOF,
		},
		{
			name:    "negative offset",
			offset:  -1,
			wantPos: position{},
			wantErr: errNegativeOffset,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := r.offsetPosition(tt.offset)
			if err != tt.wantErr {
				t.Errorf("offsetPosition(%d) err = %v, want %v", tt.offset, err, tt.wantErr)
			}
			if got != tt.wantPos {
				t.Errorf("offsetPosition(%d) pos = %+v, want %+v", tt.offset, got, tt.wantPos)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Read
// ---------------------------------------------------------------------------

func TestRead(t *testing.T) {
	type readStep struct {
		size    int
		want    string
		wantErr error
	}

	tests := []struct {
		name   string
		chunks [][]byte
		reads  []readStep
	}{
		{
			name:   "sequential reads across two chunks",
			chunks: [][]byte{[]byte("hello"), []byte("world")},
			reads: []readStep{
				{size: 5, want: "hello", wantErr: nil},
				{size: 5, want: "world", wantErr: nil},
				{size: 5, want: "", wantErr: io.EOF},
			},
		},
		{
			name:   "cross chunk boundary",
			chunks: [][]byte{[]byte("ab"), []byte("cd"), []byte("ef"), []byte("gh")},
			reads: []readStep{
				{size: 3, want: "abc", wantErr: nil},
				{size: 4, want: "defg", wantErr: nil},
			},
		},
		{
			name:   "empty buffer",
			chunks: [][]byte{[]byte("data")},
			reads: []readStep{
				{size: 0, want: "", wantErr: nil},
			},
		},
		{
			name:   "buffer larger than data",
			chunks: [][]byte{[]byte("hi")},
			reads: []readStep{
				{size: 100, want: "hi", wantErr: io.EOF},
			},
		},
		{
			name:   "single chunk exact size",
			chunks: [][]byte{[]byte("abcde")},
			reads: []readStep{
				{size: 5, want: "abcde", wantErr: nil},
			},
		},
		{
			name:   "byte by byte",
			chunks: [][]byte{[]byte("abcde"), []byte("world")},
			reads: []readStep{
				{size: 1, want: "a", wantErr: nil},
				{size: 1, want: "b", wantErr: nil},
				{size: 1, want: "c", wantErr: nil},
				{size: 1, want: "d", wantErr: nil},
				{size: 1, want: "e", wantErr: nil},
				{size: 1, want: "w", wantErr: nil},
				{size: 1, want: "o", wantErr: nil},
				{size: 1, want: "r", wantErr: nil},
				{size: 1, want: "l", wantErr: nil},
				{size: 1, want: "d", wantErr: nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := make([]chunk, len(tt.chunks))
			for i, data := range tt.chunks {
				chunks[i] = sectionChunk(data)
			}
			r := newReader(chunks...)

			for i, rd := range tt.reads {
				buf := make([]byte, rd.size)
				n, err := r.Read(buf)
				if err != rd.wantErr {
					t.Errorf("read[%d] err = %v, want %v", i, err, rd.wantErr)
				}
				if got := string(buf[:n]); got != rd.want {
					t.Errorf("read[%d] = %q, want %q", i, got, rd.want)
				}
			}
		})
	}
}

func TestReadAtDoesNotChangePosition(t *testing.T) {
	tests := []struct {
		name         string
		initialRead  int
		readAtOffset int64
		readAtSize   int
		nextRead     int
		wantNext     string
	}{
		{
			name:         "ReadAt does not move sequential position",
			initialRead:  3,
			readAtOffset: 0,
			readAtSize:   5,
			nextRead:     3,
			wantNext:     "low",
		},
		{
			name:         "ReadAt at end does not move position",
			initialRead:  5,
			readAtOffset: 9,
			readAtSize:   1,
			nextRead:     5,
			wantNext:     "world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newReader(sectionChunk([]byte("hello")), sectionChunk([]byte("world")))
			_, _ = r.Read(make([]byte, tt.initialRead))
			_, _ = r.ReadAt(make([]byte, tt.readAtSize), tt.readAtOffset)

			buf := make([]byte, tt.nextRead)
			n, err := r.Read(buf)
			if err != nil {
				t.Errorf("Read after ReadAt err = %v, want nil", err)
			}
			if got := string(buf[:n]); got != tt.wantNext {
				t.Errorf("Read after ReadAt = %q, want %q", got, tt.wantNext)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ReadAt
// ---------------------------------------------------------------------------

func TestReadAt(t *testing.T) {
	r := newReader(sectionChunk([]byte("hello")), sectionChunk([]byte("world")))

	tests := []struct {
		name    string
		offset  int64
		size    int
		want    string
		wantErr error
	}{
		{
			name:    "start of file",
			offset:  0,
			size:    5,
			want:    "hello",
			wantErr: nil,
		},
		{
			name:    "second chunk",
			offset:  5,
			size:    5,
			want:    "world",
			wantErr: nil,
		},
		{
			name:    "cross boundary",
			offset:  3,
			size:    4,
			want:    "lowo",
			wantErr: nil,
		},
		{
			name:    "last byte",
			offset:  9,
			size:    1,
			want:    "d",
			wantErr: nil,
		},
		{
			name:    "empty buffer",
			offset:  0,
			size:    0,
			want:    "",
			wantErr: nil,
		},
		{
			name:    "at EOF",
			offset:  10,
			size:    1,
			want:    "",
			wantErr: io.EOF,
		},
		{
			name:    "beyond EOF",
			offset:  100,
			size:    1,
			want:    "",
			wantErr: io.EOF,
		},
		{
			name:    "negative offset",
			offset:  -1,
			size:    1,
			want:    "",
			wantErr: errNegativeOffset,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, tt.size)
			n, err := r.ReadAt(buf, tt.offset)
			if err != tt.wantErr {
				t.Errorf("ReadAt(%d, size=%d) err = %v, want %v", tt.offset, tt.size, err, tt.wantErr)
			}
			if got := string(buf[:n]); got != tt.want {
				t.Errorf("ReadAt(%d, size=%d) = %q, want %q", tt.offset, tt.size, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Seek
// ---------------------------------------------------------------------------

func TestSeek(t *testing.T) {
	tests := []struct {
		name     string
		setup    int64 // SeekStart offset applied before the test seek (to set initial position)
		offset   int64
		whence   int
		wantOff  int64
		wantErr  error
		readByte byte // 0 means skip read check
	}{
		// SeekStart
		{
			name:     "SeekStart to 0",
			offset:   0,
			whence:   io.SeekStart,
			wantOff:  0,
			readByte: 'h',
		},
		{
			name:     "SeekStart to 5",
			offset:   5,
			whence:   io.SeekStart,
			wantOff:  5,
			readByte: 'w',
		},
		{
			name:     "SeekStart to last byte",
			offset:   9,
			whence:   io.SeekStart,
			wantOff:  9,
			readByte: 'd',
		},
		{
			name:    "SeekStart to EOF",
			offset:  10,
			whence:  io.SeekStart,
			wantOff: 10,
		},
		{
			name:    "SeekStart negative",
			offset:  -1,
			whence:  io.SeekStart,
			wantErr: errNegativeOffset,
		},
		// SeekEnd
		{
			name:    "SeekEnd 0 from end",
			offset:  0,
			whence:  io.SeekEnd,
			wantOff: 10,
		},
		{
			name:     "SeekEnd -5 from end",
			offset:   -5,
			whence:   io.SeekEnd,
			wantOff:  5,
			readByte: 'w',
		},
		{
			name:     "SeekEnd -10 from end (start)",
			offset:   -10,
			whence:   io.SeekEnd,
			wantOff:  0,
			readByte: 'h',
		},
		{
			name:    "SeekEnd past start",
			offset:  -11,
			whence:  io.SeekEnd,
			wantErr: errNegativeOffset,
		},
		// SeekCurrent
		{
			name:     "SeekCurrent +2 from 3",
			setup:    3,
			offset:   2,
			whence:   io.SeekCurrent,
			wantOff:  5,
			readByte: 'w',
		},
		{
			name:     "SeekCurrent -3 from 5",
			setup:    5,
			offset:   -3,
			whence:   io.SeekCurrent,
			wantOff:  2,
			readByte: 'l',
		},
		{
			name:    "SeekCurrent past start",
			offset:  -1,
			whence:  io.SeekCurrent,
			wantErr: errNegativeOffset,
		},
		// invalid whence
		{
			name:    "invalid whence",
			offset:  0,
			whence:  99,
			wantErr: errInvalidWhence,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newReader(sectionChunk([]byte("hello")), sectionChunk([]byte("world")))
			if tt.setup != 0 {
				_, _ = r.Seek(tt.setup, io.SeekStart)
			}

			off, err := r.Seek(tt.offset, tt.whence)
			if err != tt.wantErr {
				t.Errorf("Seek err = %v, want %v", err, tt.wantErr)
				return
			}
			if err == nil && off != tt.wantOff {
				t.Errorf("Seek offset = %d, want %d", off, tt.wantOff)
			}
			if tt.readByte != 0 {
				buf := make([]byte, 1)
				n, _ := r.Read(buf)
				if n != 1 || buf[0] != tt.readByte {
					t.Errorf("Read after Seek = %q, want %q", buf[:n], string([]byte{tt.readByte}))
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

func TestClose(t *testing.T) {
	tests := []struct {
		name    string
		reader  func() (*ChunkReader, []bool)
		wantErr bool
	}{
		{
			name: "nil receiver",
			reader: func() (*ChunkReader, []bool) {
				return nil, nil
			},
		},
		{
			name: "empty reader",
			reader: func() (*ChunkReader, []bool) {
				return &ChunkReader{}, nil
			},
		},
		{
			name: "closes all chunks",
			reader: func() (*ChunkReader, []bool) {
				closed := make([]bool, 2)
				makeChunk := func(data []byte, i int) chunk {
					r := bytes.NewReader(data)
					sr := io.NewSectionReader(r, 0, int64(len(data)))
					return chunk{file: struct {
						*io.SectionReader
						io.Closer
					}{sr, closerFunc(func() error { closed[i] = true; return nil })}, size: int64(len(data))}
				}
				r := &ChunkReader{
					chunks:    []chunk{makeChunk([]byte("ab"), 0), makeChunk([]byte("cd"), 1)},
					totalSize: 4,
				}
				return r, closed
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, closed := tt.reader()
			err := r.Close()
			if (err != nil) != tt.wantErr {
				t.Errorf("Close() err = %v, wantErr = %v", err, tt.wantErr)
			}
			for i, c := range closed {
				if !c {
					t.Errorf("chunk %d was not closed", i)
				}
			}
		})
	}
}
