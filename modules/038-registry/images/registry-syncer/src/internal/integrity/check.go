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

package integrity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/time/rate"
)

// Outcome is what reading one blob found.
type Outcome int

const (
	// Intact: the content matches the digest.
	Intact Outcome = iota

	// Corrupted: the content does not match the digest.
	Corrupted

	// NotHeld: the store has no content for the digest.
	NotHeld

	// Unsupported: the digest is of an algorithm this does not check. Only sha256 is; nothing in a
	// store of the platform is anything else.
	Unsupported
)

// chunk is how much is read and hashed at a time, and the burst of every limiter here: a limiter
// smaller than a read would refuse it.
const chunk = 1 << 20

// Limiter bounds how fast blobs are read. Nil reads as fast as the disk gives.
func Limiter(bytesPerSecond int64) *rate.Limiter {
	if bytesPerSecond <= 0 {
		return nil
	}
	return rate.NewLimiter(rate.Limit(bytesPerSecond), chunk)
}

// BlobPath is where the registry keeps a blob's content: blobs/<algorithm>/<first two>/<hex>/data.
func BlobPath(root, digest string) (string, bool) {
	algorithm, encoded, found := strings.Cut(digest, ":")
	if !found || algorithm == "" || len(encoded) < 2 || strings.ContainsAny(encoded, `/\.`) {
		return "", false
	}
	return filepath.Join(blobsDir(root), algorithm, encoded[:2], encoded, "data"), true
}

func blobsDir(root string) string {
	return filepath.Join(root, "docker", "registry", "v2", "blobs")
}

// Check reads one blob of the store and compares it with its digest, at most as fast as limiter
// allows. Also returns how many bytes it read.
//
// The pages it read are dropped from the page cache as it goes, where the system allows: the store is
// many times the memory of a master, and reading all of it would push out what etcd and the API server
// keep there.
func Check(ctx context.Context, root, digest string, limiter *rate.Limiter) (Outcome, int64, error) {
	algorithm, expected, _ := strings.Cut(digest, ":")
	if algorithm != "sha256" || len(expected) != sha256.Size*2 {
		return Unsupported, 0, nil
	}
	path, ok := BlobPath(root, digest)
	if !ok {
		return Unsupported, 0, nil
	}

	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return NotHeld, 0, nil
	}
	if err != nil {
		return Intact, 0, fmt.Errorf("opening %s: %w", digest, err)
	}
	defer func() { _ = file.Close() }()

	read, sum, err := hashFile(ctx, file, limiter)
	if err != nil {
		if ctx.Err() != nil {
			return Intact, read, ctx.Err()
		}
		if errors.Is(err, errPaced) {
			// The pace refused to wait past the deadline, before the context reached it: the blob was
			// not read to its end, which says nothing about it.
			return Intact, read, err
		}
		// Opened and not readable to its end — an I/O error of the disk under it. The registry cannot
		// serve it either, so it is as broken as content that does not match, and a Repair writes a
		// new copy beside it.
		return Corrupted, read, nil
	}
	if sum != expected {
		return Corrupted, read, nil
	}
	return Intact, read, nil
}

// errPaced is a read the pace stopped: rate.Limiter refuses to wait past the context's deadline as
// soon as the wait would cross it, while the context itself has not ended yet.
var errPaced = errors.New("the check was stopped by its pace before it read the blob to its end")

// hashFile reads a file to its end, paced, and returns its sha256 in hex.
func hashFile(ctx context.Context, file *os.File, limiter *rate.Limiter) (int64, string, error) {
	hash := sha256.New()
	buffer := make([]byte, chunk)
	var read, dropped int64
	for {
		if err := ctx.Err(); err != nil {
			return read, "", err
		}
		n, err := file.Read(buffer)
		if n > 0 {
			hash.Write(buffer[:n])
			read += int64(n)
			if limiter != nil {
				if waitErr := limiter.WaitN(ctx, n); waitErr != nil {
					return read, "", fmt.Errorf("%w: %w", errPaced, waitErr)
				}
			}
			// In steps rather than once at the end, so that a blob of several gigabytes does not sit in
			// the cache whole before it is let go.
			if read-dropped >= 64*chunk {
				dropCache(file, dropped, read-dropped)
				dropped = read
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return read, "", err
		}
	}
	dropCache(file, dropped, 0)
	return read, hex.EncodeToString(hash.Sum(nil)), nil
}

// Blob is one blob of the store as a walk of it found it.
type Blob struct {
	Digest string
	Size   int64
}

// WalkBlobs lists every blob the store holds content for. A store never written to holds none.
func WalkBlobs(root string) ([]Blob, error) {
	dir := blobsDir(root)
	var blobs []Blob
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if entry.IsDir() || entry.Name() != "data" {
			return nil
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		if len(parts) != 4 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		blobs = append(blobs, Blob{Digest: parts[0] + ":" + parts[2], Size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking the blobs of the store: %w", err)
	}
	return blobs, nil
}
