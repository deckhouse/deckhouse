// Copyright 2023 Flant JSC
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

package mirror

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

type chunkedFileWriter struct {
	chunkSize  int64
	chunkIndex int

	workingDir   string
	baseFileName string
	activeChunk  *os.File
}

func newChunkWriter(chunkSize int64, dirPath, baseFileName string) *chunkedFileWriter {
	return &chunkedFileWriter{
		chunkSize:    chunkSize,
		workingDir:   filepath.Clean(dirPath),
		baseFileName: baseFileName,
	}
}

func (c *chunkedFileWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	if c.activeChunk == nil {
		if err := c.swapActiveChunk(); err != nil {
			return 0, fmt.Errorf("Create first chunk: %w", err)
		}
	}

	chunkStat, err := c.activeChunk.Stat()
	if err != nil {
		return 0, fmt.Errorf("Read chunk size: %w", err)
	}

	buf := bytes.NewBuffer(p)
	bytesWritten := 0
	for {
		for c.chunkSize-chunkStat.Size() > 0 {
			s := buf.Next(512 * 1024)
			if len(s) == 0 {
				return bytesWritten, nil
			}

			written, err := c.activeChunk.Write(s)
			bytesWritten += written
			if err != nil {
				return 0, fmt.Errorf("Write to chunk: %w", err)
			}

			chunkStat, err = c.activeChunk.Stat()
			if err != nil {
				return 0, fmt.Errorf("Read chunk size: %w", err)
			}
		}

		if err = c.swapActiveChunk(); err != nil {
			return 0, fmt.Errorf("Swap active chunk: %w", err)
		}
		chunkStat, err = c.activeChunk.Stat()
		if err != nil {
			return 0, fmt.Errorf("Read chunk size: %w", err)
		}
	}
}

func (c *chunkedFileWriter) Close() error {
	if c.activeChunk != nil {
		if err := c.activeChunk.Sync(); err != nil {
			return fmt.Errorf("Flush chunk: %w", err)
		}
		if err := c.activeChunk.Close(); err != nil {
			return fmt.Errorf("Close chunk: %w", err)
		}
	}
	return nil
}

func (c *chunkedFileWriter) swapActiveChunk() error {
	if c.activeChunk != nil {
		if err := c.activeChunk.Sync(); err != nil {
			return fmt.Errorf("Flush chunk: %w", err)
		}
		if err := c.activeChunk.Close(); err != nil {
			return fmt.Errorf("Close previous chunk: %w", err)
		}
		c.chunkIndex += 1
	}

	newChunk, err := os.Create(filepath.Join(c.workingDir, fmt.Sprintf("%s.%04d.chunk", c.baseFileName, c.chunkIndex)))
	if err != nil {
		return fmt.Errorf("Create new chunk file: %w", err)
	}

	c.activeChunk = newChunk
	return nil
}
