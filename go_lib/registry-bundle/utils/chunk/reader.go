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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

var (
	errInvalidWhence  = errors.New("invalid whence")
	errNegativeOffset = errors.New("negative offset")
	errClosed         = errors.New("closed")
)

// ReaderAtSeekerCloser is the interface for reading chunked files (sequential and random access).
type ReaderAtSeekerCloser interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Closer
}

// Ensure implementations satisfy the interface
var (
	_ ReaderAtSeekerCloser = (*ChunkReader)(nil)
)

// chunk represents a single chunk file.
type chunk struct {
	file ReaderAtSeekerCloser
	size int64
}

// position holds the chunk index and offset within that chunk.
type position struct {
	index  int   // index of the chunk
	offset int64 // offset within the chunk
}

// ChunkReader implements ReaderAtSeekerCloser over multiple chunk files.
type ChunkReader struct {
	chunks    []chunk
	totalSize int64

	pos   position   // current position for sequential Read; also used by Seek
	posMu sync.Mutex // protects pos and concurrent access in Read/Seek calls

	closed  atomic.Bool  // set to true after Close is called; checked without holding closeMu
	closeMu sync.RWMutex // guards Close against concurrent Read/ReadAt/Seek calls
}

// Open opens the chunked file in baseDir with base name baseFileName
// (e.g. "data" finds data.0000.chunk, data.0001.chunk, ...).
func Open(baseDir, baseFileName string) (ReaderAtSeekerCloser, error) {
	reader := &ChunkReader{
		chunks: make([]chunk, 0),
	}

	withClose := func(err error) error {
		closeErr := reader.Close()
		if closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return err
	}

	for chunkIndex := 0; ; chunkIndex++ {
		file, size, err := openNextChunk(baseDir, baseFileName, chunkIndex)

		if errors.Is(err, os.ErrNotExist) {
			err = fmt.Errorf("no chunks found for %s: %w", baseFileName, err)
			return nil, withClose(err)
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, withClose(err)
		}

		reader.chunks = append(reader.chunks, chunk{file: file, size: size})
		reader.totalSize += size
	}

	if len(reader.chunks) == 0 {
		err := fmt.Errorf("no chunks found for %s", baseFileName)
		return nil, withClose(err)
	}

	if len(reader.chunks) == 1 {
		return reader.chunks[0].file, nil
	}
	return reader, nil
}

// openNextChunk opens the chunk file for the given index. It returns the file
// and its size, or an error (os.ErrNotExist when index is 0 and no chunk
// exists; io.EOF when index > 0 and no more chunks exist).
func openNextChunk(baseDir, baseFileName string, chunkIndex int) (ReaderAtSeekerCloser, int64, error) {
	chunkName := fmt.Sprintf("%s.%04d.chunk", baseFileName, chunkIndex)
	chunkPath := filepath.Join(baseDir, chunkName)
	file, err := os.Open(chunkPath)

	if err != nil {
		if errors.Is(err, os.ErrNotExist) && chunkIndex == 0 {
			return nil, 0, os.ErrNotExist
		}

		if errors.Is(err, os.ErrNotExist) && chunkIndex > 0 {
			return nil, 0, io.EOF
		}

		return nil, 0, fmt.Errorf("chunk open %s: %w", chunkName, err)
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, 0, fmt.Errorf("chunk stat %s: %w", chunkName, err)
	}

	return file, info.Size(), nil
}

// Close closes all chunk files.
// Safe to call concurrently or multiple times; subsequent calls are no-ops.
func (r *ChunkReader) Close() error {
	if r == nil {
		return nil
	}

	// Close fast check
	if r.closed.Load() {
		return nil
	}

	// Close second check
	r.closeMu.Lock()
	defer r.closeMu.Unlock()
	if r.closed.Load() {
		return nil
	}

	// Close
	defer r.closed.Store(true)

	var err error
	for _, ch := range r.chunks {
		closeErr := ch.file.Close()
		if closeErr != nil {
			err = errors.Join(err, fmt.Errorf("closing chunk: %w", closeErr))
		}
	}
	return err
}

// Read reads from the current position and advances it.
func (r *ChunkReader) Read(b []byte) (int, error) {
	// Close fast check
	if r.closed.Load() {
		return 0, errClosed
	}

	// Close second check
	r.closeMu.RLock()
	defer r.closeMu.RUnlock()
	if r.closed.Load() {
		return 0, errClosed
	}

	if len(b) == 0 {
		return 0, nil
	}

	// Serialise position update with concurrent reads.
	r.posMu.Lock()
	defer r.posMu.Unlock()

	n, newPos, err := r.readFromPosition(b, r.pos)
	r.pos = newPos
	return n, err
}

// ReadAt reads at the given offset without changing the current position.
func (r *ChunkReader) ReadAt(b []byte, offset int64) (int, error) {
	// Close fast check
	if r.closed.Load() {
		return 0, errClosed
	}

	// Close second check
	r.closeMu.RLock()
	defer r.closeMu.RUnlock()
	if r.closed.Load() {
		return 0, errClosed
	}

	if len(b) == 0 {
		return 0, nil
	}

	pos, err := r.offsetPosition(offset)
	if err != nil {
		return 0, err
	}

	n, _, err := r.readFromPosition(b, pos)
	return n, err
}

// Seek sets the position for the next Read.
func (r *ChunkReader) Seek(offset int64, whence int) (int64, error) {
	// Close fast check
	if r.closed.Load() {
		return 0, errClosed
	}

	// Close second check
	r.closeMu.RLock()
	defer r.closeMu.RUnlock()
	if r.closed.Load() {
		return 0, errClosed
	}

	// Serialise position update with concurrent reads.
	r.posMu.Lock()
	defer r.posMu.Unlock()

	switch whence {
	default:
		return 0, errInvalidWhence

	case io.SeekStart:
		// no changes

	case io.SeekCurrent:
		offset += r.positionOffset(r.pos)

	case io.SeekEnd:
		offset += r.totalSize
	}

	if offset < 0 {
		return 0, errNegativeOffset
	}

	newPos, err := r.offsetPosition(offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, err
	}

	r.pos = newPos
	return offset, nil
}

// readFromPosition reads into b starting at the given chunk position.
// It reads continuously across chunk boundaries until either:
//   - the buffer b is full
//   - no more chunks are available
//   - an error occurs
//
// Parameters:
//   - b: buffer to read data into
//   - pos: starting position (chunk index and offset within that chunk)
//
// Returns:
//   - number of bytes read
//   - new position after reading
//   - error (nil if successful, io.EOF if all chunks are exhausted, or other read errors)
func (r *ChunkReader) readFromPosition(b []byte, pos position) (int, position, error) {
	// Nothing to read, return early
	if len(b) == 0 {
		return 0, pos, nil
	}

	totalRead := 0

	// Continue reading while we have buffer space and chunks available
	for len(b) > 0 && len(r.chunks) > pos.index {
		chunk := r.chunks[pos.index]

		// Determine how many bytes to read in this iteration
		toRead := len(b)

		// Read from the current chunk at the current offset
		bytesRead, err := chunk.file.ReadAt(b[:toRead], pos.offset)

		// Update counters and position
		totalRead += bytesRead
		b = b[bytesRead:]              // Advance the buffer slice
		pos.offset += int64(bytesRead) // Advance offset within chunk

		// Handle EOF: move to next chunk and continue reading
		if errors.Is(err, io.EOF) {
			pos.index++
			pos.offset = 0
			continue
		}

		// Handle other errors (disk failure, permissions, etc.)
		if err != nil {
			return totalRead, pos, err
		}
	}

	// Return EOF if we've exhausted all chunks
	if pos.index >= len(r.chunks) {
		return totalRead, r.eofPosition(), io.EOF
	}

	return totalRead, pos, nil
}

// positionOffset converts a chunk position (chunk index and in-chunk offset)
// back to a global byte offset. If the position index is out of range (>= len(r.chunks)),
// it returns totalSize (end-of-file). The result is capped at totalSize.
func (r *ChunkReader) positionOffset(pos position) int64 {
	// If the chunk index is beyond the last valid chunk, return the total size (EOF position)
	if pos.index >= len(r.chunks) {
		return r.totalSize
	}

	// Sum the sizes of all chunks before the target chunk
	var offset int64
	for i := 0; i < pos.index; i++ {
		offset += r.chunks[i].size
	}

	// Add the offset within the target chunk
	offset += pos.offset

	// Cap at total size to prevent returning values beyond EOF
	if offset > r.totalSize {
		offset = r.totalSize
	}
	return offset
}

// offsetPosition converts a global byte offset into a chunk position (chunk index and in-chunk offset).
// It returns the position along with a nil error if the offset is within the valid range.
// If the offset is out of bounds (negative or beyond totalSize), it returns an appropriate error
// (errNegativeOffset or io.EOF) along with the corresponding EOF sentinel position.
func (r *ChunkReader) offsetPosition(offset int64) (position, error) {
	// Negative offsets are invalid
	if offset < 0 {
		return position{}, errNegativeOffset
	}

	// Offsets at or beyond total size are considered end-of-file
	if offset >= r.totalSize {
		return r.eofPosition(), io.EOF
	}

	// Traverse chunks sequentially, subtracting each chunk's size until
	// the remaining offset fits within the current chunk.
	remaining := offset
	for i, ch := range r.chunks {
		if remaining < ch.size {
			// Found the chunk containing this offset
			return position{
				index:  i,
				offset: remaining,
			}, nil
		}
		remaining -= ch.size
	}

	// Should never reach here if totalSize is consistent with chunk sizes,
	// but handle defensively by returning EOF.
	return r.eofPosition(), io.EOF
}

// eofPosition returns a sentinel position representing the end-of-file.
// The index is set to len(r.chunks) (one past the last valid chunk index),
// and offset is 0. This position is used to indicate EOF conditions.
func (r *ChunkReader) eofPosition() position {
	return position{
		index:  len(r.chunks), // out-of-range sentinel value
		offset: 0,
	}
}
